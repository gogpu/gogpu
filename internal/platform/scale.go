//go:build !darwin && !windows

package platform

// SystemScaleFactor returns the primary display DPI scale factor.
// Returns 1.0 on platforms where the scale cannot be determined before
// platform manager initialization.
func SystemScaleFactor() float64 { return 1.0 }
