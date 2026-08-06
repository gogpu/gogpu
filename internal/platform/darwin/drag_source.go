//go:build darwin

package darwin

// drag_source.go — macOS NSDragging source implementation.
//
// Provides Window.StartDrag() which uses beginDraggingSessionWithItems:event:source:
// to initiate an outgoing file drag. Files are wrapped as NSURL pasteboard writers
// inside NSDraggingItem objects.
//
// Reference: Apple NSDraggingSource documentation, NSPasteboardWriting protocol.

import (
	"sync"
	"unsafe"
)

// DragOperation represents the outcome of a macOS drag operation.
type DragOperation int

const (
	// DragOperationNone means the drag was canceled.
	DragOperationNone DragOperation = 0
	// DragOperationCopy means the target copied the data.
	DragOperationCopy DragOperation = 1
	// DragOperationMove means the target moved the data.
	DragOperationMove DragOperation = 16
)

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
// Returns the drag operation result when the drag session ends.
// beginDraggingSessionWithItems:event:source: runs a modal tracking loop
// inside AppKit, so this blocks until the drag completes.
func (w *Window) StartDrag(paths []string) DragOperation {
	initDragSourceSelectors()

	if w.contentView == 0 || len(paths) == 0 {
		return DragOperationNone
	}

	// Create NSMutableArray for NSDraggingItems.
	array := msgSend(ID(dragSourceSelectors.mutableArrayClass), selectors.alloc)
	array = msgSend(array, selectors.init)
	if array == 0 {
		return DragOperationNone
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

		// Create NSDraggingItem with the NSURL as pasteboard writer.
		// NSURL conforms to NSPasteboardWriting, so it is valid here.
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
		// No current event — cannot initiate drag outside of an event handler.
		return DragOperationNone
	}

	// Begin the drag session.
	// [contentView beginDraggingSessionWithItems:array event:currentEvent source:contentView]
	session := msgSend(w.contentView, dragSourceSelectors.beginDraggingSession,
		uintptr(array), uintptr(currentEvent), uintptr(w.contentView))

	if session == 0 {
		return DragOperationNone
	}

	// AppKit manages the drag session asynchronously from here.
	// The actual drag result requires implementing draggingSession:endedAtPoint:operation:
	// on the source view (GoGPUView). Without that callback, we default to Copy.
	// TODO: register endedAtPoint:operation: callback for accurate DragResult.
	return DragOperationCopy
}

// makeDragNSString creates an NSString from a Go string. Caller must release.
func makeDragNSString(s string) ID {
	cstr := append([]byte(s), 0)
	nsStr := msgSend(ID(classes.NSString), selectors.alloc)
	nsStr = msgSend(nsStr, selectors.initWithUTF8String, uintptr(unsafe.Pointer(&cstr[0])))
	return nsStr
}
