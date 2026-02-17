package input

import (
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// Keyboard benchmarks
// ---------------------------------------------------------------------------

// BenchmarkKeyboardSetKey measures the cost of recording a key press,
// called by the platform layer for every key event.
func BenchmarkKeyboardSetKey(b *testing.B) {
	b.ReportAllocs()
	s := New()
	for b.Loop() {
		s.Keyboard().SetKey(KeyA, true)
	}
}

// BenchmarkKeyboardPressed measures key-pressed polling cost.
// Games poll multiple keys per frame.
func BenchmarkKeyboardPressed(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Keyboard().SetKey(KeyA, true)
	var result bool
	for b.Loop() {
		result = s.Keyboard().Pressed(KeyA)
	}
	runtime.KeepAlive(result)
}

// BenchmarkKeyboardJustPressed measures just-pressed detection,
// used for single-action triggers (jump, shoot).
func BenchmarkKeyboardJustPressed(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Keyboard().SetKey(KeySpace, true)
	var result bool
	for b.Loop() {
		result = s.Keyboard().JustPressed(KeySpace)
	}
	runtime.KeepAlive(result)
}

// BenchmarkKeyboardJustReleased measures just-released detection.
func BenchmarkKeyboardJustReleased(b *testing.B) {
	b.ReportAllocs()
	s := New()
	// Press then release to trigger just-released
	s.Keyboard().SetKey(KeyA, true)
	s.Keyboard().UpdateFrame()
	s.Keyboard().SetKey(KeyA, false)
	var result bool
	for b.Loop() {
		result = s.Keyboard().JustReleased(KeyA)
	}
	runtime.KeepAlive(result)
}

// BenchmarkKeyboardModifier measures modifier key checking,
// used for keyboard shortcuts (Ctrl+C, Shift+Click).
func BenchmarkKeyboardModifier(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Keyboard().SetKey(KeyControlLeft, true)

	modifiers := []Modifier{ModShift, ModControl, ModAlt, ModSuper}
	var result bool
	for b.Loop() {
		for _, mod := range modifiers {
			result = s.Keyboard().Modifier(mod)
		}
	}
	runtime.KeepAlive(result)
}

// BenchmarkKeyboardAnyPressed measures the cost of scanning all keys.
func BenchmarkKeyboardAnyPressed(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Keyboard().SetKey(KeyW, true) // WASD movement
	var result bool
	for b.Loop() {
		result = s.Keyboard().AnyPressed()
	}
	runtime.KeepAlive(result)
}

// BenchmarkKeyboardUpdateFrame measures frame advance cost.
// Called once per frame to rotate current → previous state.
func BenchmarkKeyboardUpdateFrame(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Keyboard().SetKey(KeyW, true)
	s.Keyboard().SetKey(KeyA, true)
	s.Keyboard().SetKey(KeyS, true)
	s.Keyboard().SetKey(KeyD, true)
	for b.Loop() {
		s.Keyboard().UpdateFrame()
	}
}

// BenchmarkKeyboardMultiKeyPoll simulates polling WASD + modifiers
// as a game would do each frame.
func BenchmarkKeyboardMultiKeyPoll(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Keyboard().SetKey(KeyW, true)
	s.Keyboard().SetKey(KeyShiftLeft, true)

	var result bool
	for b.Loop() {
		kb := s.Keyboard()
		result = kb.Pressed(KeyW) || kb.Pressed(KeyA) ||
			kb.Pressed(KeyS) || kb.Pressed(KeyD) ||
			kb.Modifier(ModShift) || kb.Modifier(ModControl)
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// Mouse benchmarks
// ---------------------------------------------------------------------------

// BenchmarkMouseSetPosition measures the cost of updating mouse position,
// called by the platform layer on every mouse move event.
func BenchmarkMouseSetPosition(b *testing.B) {
	b.ReportAllocs()
	s := New()
	for b.Loop() {
		s.Mouse().SetPosition(400, 300)
	}
}

// BenchmarkMousePosition measures the cost of reading mouse position.
func BenchmarkMousePosition(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Mouse().SetPosition(400, 300)
	var x, y float32
	for b.Loop() {
		x, y = s.Mouse().Position()
	}
	runtime.KeepAlive(x)
	runtime.KeepAlive(y)
}

// BenchmarkMouseSetButton measures the cost of recording a button press.
func BenchmarkMouseSetButton(b *testing.B) {
	b.ReportAllocs()
	s := New()
	for b.Loop() {
		s.Mouse().SetButton(MouseButtonLeft, true)
	}
}

// BenchmarkMousePressed measures button-pressed polling cost.
func BenchmarkMousePressed(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Mouse().SetButton(MouseButtonLeft, true)
	var result bool
	for b.Loop() {
		result = s.Mouse().Pressed(MouseButtonLeft)
	}
	runtime.KeepAlive(result)
}

// BenchmarkMouseJustPressed measures just-pressed detection for clicks.
func BenchmarkMouseJustPressed(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Mouse().SetButton(MouseButtonLeft, true)
	var result bool
	for b.Loop() {
		result = s.Mouse().JustPressed(MouseButtonLeft)
	}
	runtime.KeepAlive(result)
}

// BenchmarkMouseDelta measures mouse movement delta calculation.
func BenchmarkMouseDelta(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Mouse().SetPosition(100, 200)
	s.Mouse().UpdateFrame()
	s.Mouse().SetPosition(110, 205)
	var dx, dy float32
	for b.Loop() {
		dx, dy = s.Mouse().Delta()
	}
	runtime.KeepAlive(dx)
	runtime.KeepAlive(dy)
}

// BenchmarkMouseScroll measures scroll wheel reading.
func BenchmarkMouseScroll(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Mouse().SetScroll(0, 3)
	var x, y float32
	for b.Loop() {
		x, y = s.Mouse().Scroll()
	}
	runtime.KeepAlive(x)
	runtime.KeepAlive(y)
}

// BenchmarkMouseUpdateFrame measures frame advance cost for mouse state.
func BenchmarkMouseUpdateFrame(b *testing.B) {
	b.ReportAllocs()
	s := New()
	s.Mouse().SetPosition(400, 300)
	s.Mouse().SetButton(MouseButtonLeft, true)
	s.Mouse().SetScroll(0, 1)
	for b.Loop() {
		s.Mouse().UpdateFrame()
	}
}

// BenchmarkInputStateUpdate measures the full per-frame input update,
// which advances both keyboard and mouse state.
func BenchmarkInputStateUpdate(b *testing.B) {
	b.ReportAllocs()
	s := New()
	// Simulate active input
	s.Keyboard().SetKey(KeyW, true)
	s.Keyboard().SetKey(KeyShiftLeft, true)
	s.Mouse().SetPosition(400, 300)
	s.Mouse().SetButton(MouseButtonLeft, true)
	for b.Loop() {
		s.Update()
	}
}
