package gogpu

import (
	"fmt"

	"github.com/gogpu/gogpu/internal/platform"
)

// DragData describes the data to drag from the application window.
// Currently supports file paths; future versions may add text, images, etc.
type DragData struct {
	// FilePaths is the list of local file paths to drag.
	// Each path must be absolute. Relative paths are rejected.
	FilePaths []string
}

// DragResult describes how a drag-and-drop operation ended.
type DragResult int

const (
	// DragCancelled means the user canceled the drag (released outside a
	// valid drop target, or pressed Escape).
	DragCancelled DragResult = iota

	// DragCopied means the target copied the dragged data.
	DragCopied

	// DragMoved means the target moved the dragged data (the source should
	// delete its copy).
	DragMoved
)

// platformToDragResult converts a platform.DragResult to a public DragResult.
func platformToDragResult(r platform.DragResult) DragResult {
	switch r {
	case platform.DragCopied:
		return DragCopied
	case platform.DragMoved:
		return DragMoved
	default:
		return DragCancelled
	}
}

// StartDrag initiates an outgoing drag-and-drop operation from this window.
// The user must already be pressing a mouse button (drag typically starts from
// a pointer-down event handler). The onComplete callback is invoked when the
// drag finishes, reporting whether the target copied, moved, or canceled.
//
// On platforms where the drag session blocks the caller (Windows, X11), the
// callback fires before StartDrag returns. On async platforms (Wayland, macOS),
// the callback fires later.
//
// Returns an error if no file paths are provided or the platform window is not
// available.
func (w *Window) StartDrag(data DragData, onComplete func(DragResult)) error {
	if len(data.FilePaths) == 0 {
		return fmt.Errorf("gogpu: StartDrag requires at least one file path")
	}
	if w.platWindow == nil {
		return fmt.Errorf("gogpu: StartDrag called on a window without a platform window")
	}
	w.platWindow.StartDrag(data.FilePaths, func(r platform.DragResult) {
		if onComplete != nil {
			onComplete(platformToDragResult(r))
		}
	})
	return nil
}

// StartDrag initiates an outgoing drag-and-drop operation from the primary window.
// Convenience wrapper around PrimaryWindow().StartDrag().
func (a *App) StartDrag(data DragData, onComplete func(DragResult)) error {
	if a.primaryWindow == nil {
		return fmt.Errorf("gogpu: StartDrag called before Run()")
	}
	return a.primaryWindow.StartDrag(data, onComplete)
}
