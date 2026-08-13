//go:build darwin

package darwin

// drag_source.go — macOS NSDragging source implementation.
//
// Provides Window.StartDrag() which uses beginDraggingSessionWithItems:event:source:
// to initiate an outgoing file drag. Files are wrapped as NSURL pasteboard writers
// inside NSDraggingItem objects.
//
// AppKit requires every NSDraggingItem to have a non-zero draggingFrame before
// beginDraggingSessionWithItems: — a zero frame raises NSRangeException and
// terminates the process. We set frame + file icon via setDraggingFrame:contents:
// (Finder / Qt / WebKit pattern).
//
// Hardening (Qt qcocoadrag.mm / JUCE Windowing_mac.mm / Flax parity):
//   - local NSAutoreleasePool around session setup
//   - skip relative paths (NSPasteboardWriting rejects them)
//   - re-entrancy guard — one active outgoing drag per view
//
// The drag session is ASYNCHRONOUS — beginDraggingSessionWithItems returns immediately.
// The result is delivered via draggingSession:endedAtPoint:operation: callback on the
// source view (GoGPUView), registered in registerGoGPUViewClass().
//
// Reference: Apple NSDraggingSource protocol, NSPasteboardWriting protocol.

import (
	"path/filepath"
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

// defaultDragIconPoints is the drag preview size in points (logical pixels).
// Matches Finder / NSWorkspace iconForFile default thumbnail used by Cocoa apps.
const defaultDragIconPoints CGFloat = 32

// dragSourceCallbacks maps view pointers to their drag completion callbacks.
// dragSourceActive prevents overlapping StartDrag sessions on the same view
// (Flax MacDragSession / Qt internal drag-loop guard).
var (
	dragSourceCallbacks   = make(map[uintptr]func(DragOperation))
	dragSourceActive      = make(map[uintptr]struct{})
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

// tryAcquireDragSource marks view as having an active outgoing drag.
// Returns false if a session is already in progress (re-entrancy guard).
func tryAcquireDragSource(view ID) bool {
	if view == 0 {
		return false
	}
	dragSourceCallbacksMu.Lock()
	defer dragSourceCallbacksMu.Unlock()
	ptr := view.Ptr()
	if _, ok := dragSourceActive[ptr]; ok {
		return false
	}
	dragSourceActive[ptr] = struct{}{}
	return true
}

// releaseDragSource clears the active flag and completion callback for view.
func releaseDragSource(view ID) {
	if view == 0 {
		return
	}
	dragSourceCallbacksMu.Lock()
	defer dragSourceCallbacksMu.Unlock()
	ptr := view.Ptr()
	delete(dragSourceActive, ptr)
	delete(dragSourceCallbacks, ptr)
}

// isDragSourceActive reports whether view has an in-flight outgoing drag.
func isDragSourceActive(view ID) bool {
	if view == 0 {
		return false
	}
	dragSourceCallbacksMu.RLock()
	defer dragSourceCallbacksMu.RUnlock()
	_, ok := dragSourceActive[view.Ptr()]
	return ok
}

// absoluteDragPaths returns only absolute paths.
// Relative URLs are rejected by NSPasteboardWriting (Qt qcocoadrag.mm).
func absoluteDragPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" && filepath.IsAbs(p) {
			out = append(out, p)
		}
	}
	return out
}

// dragSourceSelectors holds selectors for drag source operations.
var dragSourceSelectors struct {
	once sync.Once

	fileURLWithPath          SEL
	urlClass                 Class
	draggingItemClass        Class
	initWithPasteboardWriter SEL
	setDraggingFrameContents SEL
	beginDraggingSession     SEL
	mutableArrayClass        Class
	addObject                SEL
	currentEvent             SEL
	iconForFile              SEL
	setSize                  SEL
	copy                     SEL
}

func initDragSourceSelectors() {
	dragSourceSelectors.once.Do(func() {
		initSelectors()

		dragSourceSelectors.fileURLWithPath = RegisterSelector("fileURLWithPath:")
		dragSourceSelectors.urlClass = GetClass("NSURL")
		dragSourceSelectors.draggingItemClass = GetClass("NSDraggingItem")
		dragSourceSelectors.initWithPasteboardWriter = RegisterSelector("initWithPasteboardWriter:")
		dragSourceSelectors.setDraggingFrameContents = RegisterSelector("setDraggingFrame:contents:")
		dragSourceSelectors.beginDraggingSession = RegisterSelector("beginDraggingSessionWithItems:event:source:")
		dragSourceSelectors.mutableArrayClass = GetClass("NSMutableArray")
		dragSourceSelectors.addObject = RegisterSelector("addObject:")
		dragSourceSelectors.currentEvent = RegisterSelector("currentEvent")
		dragSourceSelectors.iconForFile = RegisterSelector("iconForFile:")
		dragSourceSelectors.setSize = RegisterSelector("setSize:")
		dragSourceSelectors.copy = RegisterSelector("copy")
	})
}

// draggingFrameAt returns a non-zero NSRect centered on loc for drag item index.
// index offsets stacked multi-file previews (AppKit NSDraggingFormationLoose style).
// size < 1 falls back to defaultDragIconPoints — AppKit aborts on zero-size frames.
func draggingFrameAt(loc NSPoint, size CGFloat, index int) NSRect {
	if size < 1 {
		size = defaultDragIconPoints
	}
	half := size / 2
	offset := CGFloat(index) * 4
	return MakeRect(loc.X-half+offset, loc.Y-half-offset, size, size)
}

// fileDragIcon returns an NSImage for path sized to points, or 0 on failure.
// The returned image is owned by the caller (+1) when copy succeeds — release
// after setDraggingFrame:contents: retains it. Falls back to the autoreleased
// NSWorkspace icon when copy fails.
func fileDragIcon(path string, points CGFloat) (icon ID, owned bool) {
	if path == "" || classes.NSWorkspace == 0 {
		return 0, false
	}
	if points < 1 {
		points = defaultDragIconPoints
	}

	workspace := msgSend(ID(classes.NSWorkspace), selectors.sharedWorkspace)
	if workspace == 0 {
		return 0, false
	}

	nsPath := makeDragNSString(path)
	if nsPath == 0 {
		return 0, false
	}
	defer msgSend(nsPath, selectors.release)

	icon = msgSend(workspace, dragSourceSelectors.iconForFile, uintptr(nsPath))
	if icon == 0 {
		return 0, false
	}
	// Defensive copy — setSize: must not mutate NSWorkspace's shared cache
	// (Qt/JUCE pattern: [[icon copy] setSize:]).
	if copied := msgSend(icon, dragSourceSelectors.copy); copied != 0 {
		icon = copied
		owned = true
	}
	icon.SendSize(dragSourceSelectors.setSize, MakeSize(points, points))
	return icon, owned
}

// StartDrag initiates an outgoing file drag from the window's content view.
// The done callback fires asynchronously when the drag session ends via
// draggingSession:endedAtPoint:operation: on GoGPUView.
func (w *Window) StartDrag(paths []string, done func(DragOperation)) {
	initDragSourceSelectors()
	initClasses()

	completeNone := func() {
		if done != nil {
			done(DragOperationNone)
		}
	}

	if w.contentView == 0 || len(paths) == 0 {
		completeNone()
		return
	}

	paths = absoluteDragPaths(paths)
	if len(paths) == 0 {
		completeNone()
		return
	}

	// Flax / Qt: reject overlapping outgoing drag on the same view.
	if !tryAcquireDragSource(w.contentView) {
		completeNone()
		return
	}

	// Always register a release wrapper so the active flag clears on session end,
	// even when the caller passes a nil done callback.
	view := w.contentView
	SetDragSourceCallback(view, func(op DragOperation) {
		releaseDragSource(view)
		if done != nil {
			done(op)
		}
	})

	fail := func() {
		releaseDragSource(view)
		completeNone()
	}

	// Local pool — Qt QMacAutoReleasePool / JUCE_AUTORELEASEPOOL.
	// Transient NSURL/NSImage/NSString from iconForFile: and fileURLWithPath: must drain here.
	RunInFramePool(func() {
		array := msgSend(ID(dragSourceSelectors.mutableArrayClass), selectors.alloc)
		array = msgSend(array, selectors.init)
		if array == 0 {
			fail()
			return
		}
		defer msgSend(array, selectors.release)

		nsApp := msgSend(ID(classes.NSApplication), selectors.sharedApplication)
		currentEvent := msgSend(nsApp, dragSourceSelectors.currentEvent)
		if currentEvent == 0 {
			fail()
			return
		}

		// draggingFrame must be in the initiating view's coordinate system.
		winLoc := currentEvent.GetPoint(selectors.locationInWindow)
		viewLoc := view.ConvertPointFromView(winLoc, 0)

		itemIndex := 0
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

			// REQUIRED: non-zero draggingFrame. Zero size → NSRangeException + process abort.
			frame := draggingFrameAt(viewLoc, defaultDragIconPoints, itemIndex)
			icon, owned := fileDragIcon(path, defaultDragIconPoints)
			item.SendRectPtr(dragSourceSelectors.setDraggingFrameContents, frame, uintptr(icon))
			if owned {
				msgSend(icon, selectors.release)
			}

			msgSend(array, dragSourceSelectors.addObject, uintptr(item))
			msgSend(item, selectors.release)
			itemIndex++
		}

		if itemIndex == 0 {
			fail()
			return
		}

		session := msgSend(view, dragSourceSelectors.beginDraggingSession,
			uintptr(array), uintptr(currentEvent), uintptr(view))
		if session == 0 {
			fail()
			return
		}
		// Session is async — result arrives via endedAtPoint:operation: callback.
	})
}

// makeDragNSString creates an NSString from a Go string. Caller must release.
func makeDragNSString(s string) ID {
	cstr := append([]byte(s), 0)
	nsStr := msgSend(ID(classes.NSString), selectors.alloc)
	nsStr = msgSend(nsStr, selectors.initWithUTF8String, uintptr(unsafe.Pointer(&cstr[0])))
	return nsStr
}
