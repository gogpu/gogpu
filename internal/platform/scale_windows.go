//go:build windows

package platform

// SystemScaleFactor returns the primary display DPI scale factor.
// On Windows this queries GetDpiForMonitor on the primary monitor via shcore.dll
// and is safe to call before platform manager initialization.
// Returns 1.0 on Windows 7/8 where shcore.dll is unavailable.
func SystemScaleFactor() float64 {
	return float64(monitorDpiFromPoint(0, 0, true)) / 96.0
}
