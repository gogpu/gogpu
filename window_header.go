package gogpu

import "github.com/gogpu/gogpu/internal/platform"

// HeaderAlignment controls the horizontal position of the title in the OS
// window header bar. It mirrors the Flutter AppBar.centerTitle pattern.
type HeaderAlignment int

const (
	// HeaderAlignCenter centers the title in the header bar.
	// This is the default macOS behavior and requires no platform changes.
	HeaderAlignCenter HeaderAlignment = iota

	// HeaderAlignLeft positions the title on the left side of the header bar.
	// On macOS: enables NSWindowStyleMaskFullSizeContentView and a transparent
	// title bar so GPU-rendered content extends into the title bar area.
	// The native title text is hidden; the application draws its own header.
	// On Windows/Linux: stored but not applied (not OS-configurable).
	HeaderAlignLeft

	// HeaderAlignRight positions the title on the right side of the header bar.
	// On macOS: same behavior as HeaderAlignLeft — content fills the title bar.
	// On Windows/Linux: stored but not applied (not OS-configurable).
	HeaderAlignRight
)

// SetTitle changes the OS window title for this specific window.
// This is the per-window equivalent of App.SetTitle.
func (w *Window) SetTitle(title string) {
	w.config.Title = title
	if w.platWindow != nil {
		w.platWindow.SetTitle(title)
	}
}

// Title returns the current OS window title.
func (w *Window) Title() string {
	return w.config.Title
}

// SetHeaderAlignment configures title alignment in the native OS header bar.
// Applied immediately when the platform window exists; deferred otherwise.
//
// Platform notes:
//   - macOS: Left/Right enable NSWindowStyleMaskFullSizeContentView with a
//     transparent title bar. The native title text is hidden, leaving the
//     traffic-light buttons visible. GPU content fills the header area.
//   - Windows/Linux: alignment is stored for API consistency but has no
//     visual effect — the OS controls title position on those platforms.
func (w *Window) SetHeaderAlignment(alignment HeaderAlignment) {
	w.headerAlignment = alignment
	if w.platWindow != nil {
		applyHeaderAlignment(w.platWindow, alignment)
	}
}

// HeaderAlignment returns the window's current header title alignment.
func (w *Window) HeaderAlignment() HeaderAlignment {
	return w.headerAlignment
}

// SetHeaderAlignment sets the primary window's header title alignment.
// When called before Run(), the value is stored and applied when the
// platform window is created (same deferred pattern as SetHitTestCallback).
func (a *App) SetHeaderAlignment(alignment HeaderAlignment) {
	a.config.HeaderAlignment = alignment
	if a.primaryWindow != nil {
		a.primaryWindow.SetHeaderAlignment(alignment)
		return
	}
	if a.platWindow != nil {
		applyHeaderAlignment(a.platWindow, alignment)
	}
}

// HeaderAlignment returns the primary window's current header title alignment.
func (a *App) HeaderAlignment() HeaderAlignment {
	if a.primaryWindow != nil {
		return a.primaryWindow.HeaderAlignment()
	}
	return a.config.HeaderAlignment
}

// applyHeaderAlignment delegates to platform.HeaderAligner when the platform
// window supports it. Platforms that don't implement HeaderAligner are silently
// skipped — the alignment is stored in the Window for API consistency.
func applyHeaderAlignment(pw platform.PlatformWindow, alignment HeaderAlignment) {
	if ha, ok := pw.(platform.HeaderAligner); ok {
		ha.SetHeaderAlignment(int(alignment))
	}
}
