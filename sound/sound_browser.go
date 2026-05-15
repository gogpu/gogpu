//go:build js && wasm

package sound

// platformPlay is a no-op on browser.
// Browser audio requires user interaction to start (autoplay policy)
// and would need the Web Audio API — out of scope for initial WASM support.
func platformPlay(_ SystemSound) {}

// platformPlayFile is a no-op on browser.
func platformPlayFile(_ string) error { return nil }
