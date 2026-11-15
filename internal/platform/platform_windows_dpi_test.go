//go:build windows

package platform

import "testing"

// TestDPIForWindowFallback covers issue #472: on Windows older than 10 1607
// user32 exports no GetDpiForWindow, and the lazy call panicked at the first
// window. The guarded helper must answer with the screen DPI instead of zero.
func TestDPIForWindowFallback(t *testing.T) {
	if got := dpiForWindowWith(false, 0); got == 0 {
		t.Fatal("missing export returned 0 DPI")
	}
	if got := dpiForWindowWith(true, 0); got == 0 {
		t.Fatal("missing HWND returned 0 DPI")
	}
	if got := dpiForWindow(0); got == 0 {
		t.Fatal("dpiForWindow(0) returned 0 DPI")
	}
}
