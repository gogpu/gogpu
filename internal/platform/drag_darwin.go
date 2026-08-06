//go:build darwin

package platform

// drag_darwin.go — Outgoing drag-and-drop (drag source) via macOS NSDragging.
//
// Uses NSView.beginDraggingSessionWithItems:event:source: to initiate a drag.
// Each file path is wrapped as an NSURL → NSDraggingItem. Our GoGPUView is
// registered as the NSDraggingSource by implementing
// draggingSession:sourceOperationMaskForDraggingContext: via class_addMethod
// during view creation.
//
// The drag is asynchronous on macOS — the event loop continues to run. The
// done callback fires when draggingSession:endedAtPoint:operation: is invoked
// by AppKit.

import (
	"github.com/gogpu/gogpu/internal/platform/darwin"
)

// startDragDarwin initiates a macOS drag session with file paths.
// The done callback is called asynchronously when the drag ends.
func startDragDarwin(window *darwin.Window, paths []string, done func(DragResult)) {
	if window == nil {
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	result := window.StartDrag(paths)

	// On macOS, the drag operation runs inside the AppKit event loop.
	// darwin.Window.StartDrag is synchronous — it blocks until the drag
	// session completes (AppKit runs a modal tracking loop internally).
	var dragResult DragResult
	switch result {
	case darwin.DragOperationCopy:
		dragResult = DragCopied
	case darwin.DragOperationMove:
		dragResult = DragMoved
	default:
		dragResult = DragCancelled
	}

	if done != nil {
		done(dragResult)
	}
}
