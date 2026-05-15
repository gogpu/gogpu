//go:build js && wasm

package gogpu

import "fmt"

// rustHalAvailable returns false on browser — Rust FFI is not available.
func rustHalAvailable() bool {
	return false
}

// initRust returns an error because Rust FFI is not available in the browser.
func (r *Renderer) initRust() error {
	return fmt.Errorf("gogpu: rust backend not available in browser")
}
