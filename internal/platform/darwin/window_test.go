//go:build darwin

package darwin_test

import (
	"testing"
	"time"

	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
)

// TestZoomTogglesTheZoomStateAndReturns zooms a shown window, whose delegate
// AppKit calls inside zoom:, and zooms it back.
func TestZoomTogglesTheZoomStateAndReturns(t *testing.T) {
	window := openShownWindow(t)
	for _, want := range []bool{true, false} {
		var zoomed bool
		runOnMainThreadWithin(t, "Zoom", func() {
			window.Zoom()
			zoomed = window.IsZoomed()
		})
		if zoomed != want {
			t.Errorf("IsZoomed after Zoom = %v, want %v", zoomed, want)
		}
	}
	runOnMainThread(t, window.Destroy)
}

// TestRequestSizeRestoresAZoomedWindowAndReturns sizes a zoomed window, which
// RequestSize restores through zoom: first.
func TestRequestSizeRestoresAZoomedWindowAndReturns(t *testing.T) {
	window := openShownWindow(t)
	var zoomed bool
	runOnMainThreadWithin(t, "Zoom", func() {
		window.Zoom()
		zoomed = window.IsZoomed()
	})
	if !zoomed {
		t.Fatal("IsZoomed after Zoom = false, want true")
	}

	var content platformdarwin.NSRect
	runOnMainThreadWithin(t, "RequestSize", func() {
		window.RequestSize(500, 350)
		zoomed = window.IsZoomed()
		content = window.ContentRect()
	})
	if zoomed {
		t.Error("IsZoomed after RequestSize = true, want false")
	}
	if content.Size.Width != 500 || content.Size.Height != 350 {
		t.Errorf("content size after RequestSize = %vx%v, want 500x350", content.Size.Width, content.Size.Height)
	}
	runOnMainThread(t, window.Destroy)
}

// openShownWindow opens a window on the main thread and shows it.
func openShownWindow(t *testing.T) *platformdarwin.Window {
	t.Helper()
	var window *platformdarwin.Window
	var err error
	runOnMainThread(t, func() {
		if err = platformdarwin.GetApplication().Init(); err != nil {
			return
		}
		window, err = platformdarwin.NewWindow(platformdarwin.WindowConfig{
			Title:     "zoom",
			Width:     400,
			Height:    300,
			Resizable: true,
		})
		if err == nil {
			window.Show()
		}
	})
	if err != nil {
		t.Fatalf("opening a window: %v", err)
	}
	return window
}

// runOnMainThreadWithin runs fn on the main thread, and ends the process if
// fn does not return within 5s: t.Fatal would leave it hung on the stuck main
// thread.
func runOnMainThreadWithin(t *testing.T, call string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	mainThreadTasks <- func() {
		fn()
		close(done)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		panic(call + " did not return within 5s")
	}
}
