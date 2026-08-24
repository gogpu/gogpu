//go:build !js || !wasm

package gogpu

// runMainLoop runs the blocking native platform loop. Browser/WASM replaces
// this with a requestAnimationFrame-driven implementation.
func (a *App) runMainLoop() {
	for a.running.Load() && (a.platWindow == nil || !a.platWindow.ShouldClose()) {
		a.runFrame()
	}
}
