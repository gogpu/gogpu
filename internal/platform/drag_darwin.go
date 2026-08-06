//go:build darwin

package platform

// drag_darwin.go — Outgoing drag-and-drop (drag source) via macOS NSDragging.
//
// Uses NSView.beginDraggingSessionWithItems:event:source: to initiate a drag.
// Each file path is wrapped as an NSURL → NSDraggingItem. Our GoGPUView is
// registered as the NSDraggingSource by implementing both
// draggingSession:sourceOperationMaskForDraggingContext: and
// draggingSession:endedAtPoint:operation: via class_addMethod during view creation.
//
// The drag is asynchronous on macOS — beginDraggingSessionWithItems returns
// immediately and the event loop continues. The done callback fires when
// AppKit invokes draggingSession:endedAtPoint:operation: on GoGPUView.

import (
	"github.com/gogpu/gogpu/internal/platform/darwin"
)

// startDragDarwin initiates a macOS drag session with file paths.
// The done callback fires asynchronously when the drag ends.
func startDragDarwin(window *darwin.Window, paths []string, done func(DragResult)) {
	if window == nil {
		if done != nil {
			done(DragCancelled)
		}
		return
	}

	window.StartDrag(paths, func(op darwin.DragOperation) {
		if done == nil {
			return
		}
		switch op {
		case darwin.DragOperationCopy:
			done(DragCopied)
		case darwin.DragOperationMove:
			done(DragMoved)
		default:
			done(DragCancelled)
		}
	})
}
