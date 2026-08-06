//go:build darwin

package darwin

// drag_source.go — macOS NSDragging source implementation.
//
// Provides Window.StartDrag() which uses beginDraggingSessionWithItems:event:source:
// to initiate an outgoing file drag. Files are wrapped as NSURL pasteboard writers
// inside NSDraggingItem objects.
//
// The drag session is ASYNCHRONOUS — beginDraggingSessionWithItems returns immediately.
// The result is delivered via draggingSession:endedAtPoint:operation: callback on the
// source view (GoGPUView), registered in registerGoGPUViewClass().
//
// Reference: Apple NSDraggingSource protocol, NSPasteboardWriting protocol.

import (
	"sync"
	"unsafe"
)

// DragOperation represents the outcome of a macOS drag operation.
type DragOperation int

const (
	DragOperationNone DragOperation = 0
	DragOperationCopy DragOperation = 1
	DragOperationMove DragOperation = 16
)

// dragSourceCallbacks maps view pointers to their drag completion callbacks.
// Same pattern as viewDragHandlers for incoming drag.
var (
	dragSourceCallbacks   = make(map[uintptr]func(DragOperation))
	dragSourceCallbacksMu sync.RWMutex
)

// SetDragSourceCallback registers a one-shot callback for the drag result.
func SetDragSourceCallback(view ID, cb func(DragOperation)) {
	dragSourceCallbacksMu.Lock()
	defer dragSourceCallbacksMu.Unlock()
	if cb == nil {
		delete(dragSourceCallbacks, view.Ptr())
	} else {
		dragSourceCallbacks[view.Ptr()] = cb
	}
}

// getDragSourceCallback retrieves the drag completion callback for a view.
func getDragSourceCallback(viewPtr uintptr) func(DragOperation) {
	dragSourceCallbacksMu.RLock()
	defer dragSourceCallbacksMu.RUnlock()
	return dragSourceCallbacks[viewPtr]
}

// dragSourceSelectors holds selectors for drag source operations.
var dragSourceSelectors struct {
	once sync.Once

	fileURLWithPath          SEL
	urlClass                 Class
	draggingItemClass        Class
	initWithPasteboardWriter SEL
	beginDraggingSession     SEL
	mutableArrayClass        Class
	addObject                SEL
	currentEvent             SEL
}

func initDragSourceSelectors() {
	dragSourceSelectors.once.Do(func() {
		initSelectors()

		dragSourceSelectors.fileURLWithPath = RegisterSelector("fileURLWithPath:")
		dragSourceSelectors.urlClass = GetClass("NSURL")
		dragSourceSelectors.draggingItemClass = GetClass("NSDraggingItem")
		dragSourceSelectors.initWithPasteboardWriter = RegisterSelector("initWithPasteboardWriter:")
		dragSourceSelectors.beginDraggingSession = RegisterSelector("beginDraggingSessionWithItems:event:source:")
		dragSourceSelectors.mutableArrayClass = GetClass("NSMutableArray")
		dragSourceSelectors.addObject = RegisterSelector("addObject:")
		dragSourceSelectors.currentEvent = RegisterSelector("currentEvent")
	})
}

// StartDrag initiates an outgoing file drag from the window's content view.
// The done callback fires asynchronously when the drag session ends via
// draggingSession:endedAtPoint:operation: on GoGPUView.
func (w *Window) StartDrag(paths []string, done func(DragOperation)) {
	initDragSourceSelectors()

	if w.contentView == 0 || len(paths) == 0 {
		if done != nil {
			done(DragOperationNone)
		}
		return
	}

	// Register callback BEFORE starting drag (async — result arrives later).
	if done != nil {
		SetDragSourceCallback(w.contentView, func(op DragOperation) {
			SetDragSourceCallback(w.contentView, nil) // one-shot
			done(op)
		})
	}

	// Create NSMutableArray for NSDraggingItems.
	array := msgSend(ID(dragSourceSelectors.mutableArrayClass), selectors.alloc)
	array = msgSend(array, selectors.init)
	if array == 0 {
		SetDragSourceCallback(w.contentView, nil)
		if done != nil {
			done(DragOperationNone)
		}
		return
	}
	defer msgSend(array, selectors.release)

	for _, path := range paths {
		nsPath := makeDragNSString(path)
		if nsPath == 0 {
			continue
		}
		nsURL := msgSend(ID(dragSourceSelectors.urlClass), dragSourceSelectors.fileURLWithPath, uintptr(nsPath))
		msgSend(nsPath, selectors.release)
		if nsURL == 0 {
			continue
		}

		item := msgSend(ID(dragSourceSelectors.draggingItemClass), selectors.alloc)
		item = msgSend(item, dragSourceSelectors.initWithPasteboardWriter, uintptr(nsURL))
		if item == 0 {
			continue
		}

		msgSend(array, dragSourceSelectors.addObject, uintptr(item))
		msgSend(item, selectors.release)
	}

	// Get the current event (needed by beginDraggingSession).
	nsApp := msgSend(ID(classes.NSApplication), selectors.sharedApplication)
	currentEvent := msgSend(nsApp, dragSourceSelectors.currentEvent)

	if currentEvent == 0 {
		SetDragSourceCallback(w.contentView, nil)
		if done != nil {
			done(DragOperationNone)
		}
		return
	}

	// Begin the async drag session.
	session := msgSend(w.contentView, dragSourceSelectors.beginDraggingSession,
		uintptr(array), uintptr(currentEvent), uintptr(w.contentView))

	if session == 0 {
		SetDragSourceCallback(w.contentView, nil)
		if done != nil {
			done(DragOperationNone)
		}
		return
	}
	// Session is async — result arrives via endedAtPoint:operation: callback.
}

// makeDragNSString creates an NSString from a Go string. Caller must release.
func makeDragNSString(s string) ID {
	cstr := append([]byte(s), 0)
	nsStr := msgSend(ID(classes.NSString), selectors.alloc)
	nsStr = msgSend(nsStr, selectors.initWithUTF8String, uintptr(unsafe.Pointer(&cstr[0])))
	return nsStr
}
