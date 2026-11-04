//go:build js && wasm

package gogpu

// isZeroArea reports whether a surface configure failed on a zero extent.
// The browser canvas never reports one.
func isZeroArea(error) bool { return false }
