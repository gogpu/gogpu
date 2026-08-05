package input

import (
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Keyboard: basic edge detection
// ---------------------------------------------------------------------------

func TestKeyboard_JustPressed_SingleFrame(t *testing.T) {
	s := New()

	s.Keyboard().SetKey(KeySpace, true)

	if !s.Keyboard().JustPressed(KeySpace) {
		t.Fatal("JustPressed must be true after press, before UpdateFrame")
	}
	if !s.Keyboard().Pressed(KeySpace) {
		t.Fatal("Pressed must be true after press")
	}

	s.Update()

	if s.Keyboard().JustPressed(KeySpace) {
		t.Fatal("JustPressed must be false after UpdateFrame (previous now matches current)")
	}
	if !s.Keyboard().Pressed(KeySpace) {
		t.Fatal("Pressed must remain true while key is held")
	}
}

func TestKeyboard_JustReleased_SingleFrame(t *testing.T) {
	s := New()

	// Press and advance so previous=true, current=true.
	s.Keyboard().SetKey(KeyA, true)
	s.Update()

	// Release.
	s.Keyboard().SetKey(KeyA, false)

	if !s.Keyboard().JustReleased(KeyA) {
		t.Fatal("JustReleased must be true after release, before UpdateFrame")
	}
	if s.Keyboard().Pressed(KeyA) {
		t.Fatal("Pressed must be false after release")
	}

	s.Update()

	if s.Keyboard().JustReleased(KeyA) {
		t.Fatal("JustReleased must be false after UpdateFrame")
	}
}

// ---------------------------------------------------------------------------
// Keyboard: multi-frame lifecycle
// ---------------------------------------------------------------------------

func TestKeyboard_MultiFrameLifecycle(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	// Frame 1: press key.
	kb.SetKey(KeyW, true)

	if !kb.JustPressed(KeyW) {
		t.Fatal("frame 1: JustPressed must be true")
	}
	if !kb.Pressed(KeyW) {
		t.Fatal("frame 1: Pressed must be true")
	}
	if kb.JustReleased(KeyW) {
		t.Fatal("frame 1: JustReleased must be false")
	}

	s.Update()

	// Frame 2: hold key (no new events).
	if kb.JustPressed(KeyW) {
		t.Fatal("frame 2: JustPressed must be false (held, not new press)")
	}
	if !kb.Pressed(KeyW) {
		t.Fatal("frame 2: Pressed must be true (still held)")
	}
	if kb.JustReleased(KeyW) {
		t.Fatal("frame 2: JustReleased must be false")
	}

	s.Update()

	// Frame 3: release key.
	kb.SetKey(KeyW, false)

	if kb.JustPressed(KeyW) {
		t.Fatal("frame 3: JustPressed must be false")
	}
	if kb.Pressed(KeyW) {
		t.Fatal("frame 3: Pressed must be false after release")
	}
	if !kb.JustReleased(KeyW) {
		t.Fatal("frame 3: JustReleased must be true")
	}

	s.Update()

	// Frame 4: idle (no events).
	if kb.JustPressed(KeyW) {
		t.Fatal("frame 4: JustPressed must be false")
	}
	if kb.Pressed(KeyW) {
		t.Fatal("frame 4: Pressed must be false")
	}
	if kb.JustReleased(KeyW) {
		t.Fatal("frame 4: JustReleased must be false (stale)")
	}
}

// ---------------------------------------------------------------------------
// Keyboard: press and release within same frame
// ---------------------------------------------------------------------------

func TestKeyboard_PressAndReleaseSameFrame(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	// Both press and release arrive before UpdateFrame.
	// The final state is current=false. With previous=false (initial),
	// both JustPressed and JustReleased are false. The transient press
	// is lost. This is the expected behavior of a last-write-wins model.
	kb.SetKey(KeyD, true)
	kb.SetKey(KeyD, false)

	if kb.JustPressed(KeyD) {
		t.Fatal("JustPressed must be false: key ended up released, previous was also released")
	}
	if kb.JustReleased(KeyD) {
		t.Fatal("JustReleased must be false: previous was false too")
	}
	if kb.Pressed(KeyD) {
		t.Fatal("Pressed must be false: final state is released")
	}
}

func TestKeyboard_ReleaseThenPressSameFrame(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	// First hold the key so previous=true after update.
	kb.SetKey(KeyE, true)
	s.Update()

	// Release then re-press within one frame. Final state = pressed.
	// previous=true, current=true -> JustPressed=false (was already down).
	kb.SetKey(KeyE, false)
	kb.SetKey(KeyE, true)

	if kb.JustPressed(KeyE) {
		t.Fatal("JustPressed must be false: previous was true, current is true (re-press lost)")
	}
	if kb.JustReleased(KeyE) {
		t.Fatal("JustReleased must be false: key is currently pressed")
	}
	if !kb.Pressed(KeyE) {
		t.Fatal("Pressed must be true: final state is pressed")
	}
}

// ---------------------------------------------------------------------------
// Keyboard: multiple keys simultaneously
// ---------------------------------------------------------------------------

func TestKeyboard_MultipleKeysSimultaneous(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	kb.SetKey(KeyA, true)
	kb.SetKey(KeyB, true)
	kb.SetKey(KeyC, true)

	for _, key := range []Key{KeyA, KeyB, KeyC} {
		if !kb.JustPressed(key) {
			t.Fatalf("JustPressed(%d) must be true when multiple keys pressed in same frame", key)
		}
	}

	// Unrelated key must remain unaffected.
	if kb.JustPressed(KeyD) {
		t.Fatal("JustPressed(KeyD) must be false: never pressed")
	}

	s.Update()

	for _, key := range []Key{KeyA, KeyB, KeyC} {
		if kb.JustPressed(key) {
			t.Fatalf("JustPressed(%d) must be false after UpdateFrame", key)
		}
		if !kb.Pressed(key) {
			t.Fatalf("Pressed(%d) must remain true (keys held)", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Keyboard: out-of-range keys (boundary safety)
// ---------------------------------------------------------------------------

func TestKeyboard_OutOfRangeKey(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	bigKey := KeyCount + 10

	// SetKey must not panic.
	kb.SetKey(bigKey, true)

	if kb.Pressed(bigKey) {
		t.Fatal("Pressed must return false for out-of-range key")
	}
	if kb.JustPressed(bigKey) {
		t.Fatal("JustPressed must return false for out-of-range key")
	}
	if kb.JustReleased(bigKey) {
		t.Fatal("JustReleased must return false for out-of-range key")
	}
}

// ---------------------------------------------------------------------------
// Keyboard: modifier keys
// ---------------------------------------------------------------------------

func TestKeyboard_Modifiers(t *testing.T) {
	tests := []struct {
		name string
		mod  Modifier
		left Key
		right Key
	}{
		{"Shift", ModShift, KeyShiftLeft, KeyShiftRight},
		{"Control", ModControl, KeyControlLeft, KeyControlRight},
		{"Alt", ModAlt, KeyAltLeft, KeyAltRight},
		{"Super", ModSuper, KeySuperLeft, KeySuperRight},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Left", func(t *testing.T) {
			s := New()
			s.Keyboard().SetKey(tt.left, true)
			if !s.Keyboard().Modifier(tt.mod) {
				t.Fatalf("Modifier(%v) must be true when left key pressed", tt.mod)
			}
		})
		t.Run(tt.name+"_Right", func(t *testing.T) {
			s := New()
			s.Keyboard().SetKey(tt.right, true)
			if !s.Keyboard().Modifier(tt.mod) {
				t.Fatalf("Modifier(%v) must be true when right key pressed", tt.mod)
			}
		})
		t.Run(tt.name+"_None", func(t *testing.T) {
			s := New()
			if s.Keyboard().Modifier(tt.mod) {
				t.Fatalf("Modifier(%v) must be false when neither key pressed", tt.mod)
			}
		})
	}
}

func TestKeyboard_ModifierUnknown(t *testing.T) {
	s := New()
	// Modifier value that does not match any case in the switch.
	if s.Keyboard().Modifier(Modifier(0)) {
		t.Fatal("Modifier(0) must return false for unknown modifier")
	}
}

// ---------------------------------------------------------------------------
// Keyboard: AnyPressed
// ---------------------------------------------------------------------------

func TestKeyboard_AnyPressed(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	if kb.AnyPressed() {
		t.Fatal("AnyPressed must be false initially")
	}

	kb.SetKey(KeyZ, true)
	if !kb.AnyPressed() {
		t.Fatal("AnyPressed must be true after pressing a key")
	}

	kb.SetKey(KeyZ, false)
	if kb.AnyPressed() {
		t.Fatal("AnyPressed must be false after releasing the only pressed key")
	}
}

// ---------------------------------------------------------------------------
// Mouse: button edge detection
// ---------------------------------------------------------------------------

func TestMouse_JustPressed_SingleFrame(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetButton(MouseButtonLeft, true)

	if !m.JustPressed(MouseButtonLeft) {
		t.Fatal("Mouse JustPressed must be true after button press, before UpdateFrame")
	}
	if !m.Pressed(MouseButtonLeft) {
		t.Fatal("Mouse Pressed must be true after button press")
	}

	s.Update()

	if m.JustPressed(MouseButtonLeft) {
		t.Fatal("Mouse JustPressed must be false after UpdateFrame")
	}
	if !m.Pressed(MouseButtonLeft) {
		t.Fatal("Mouse Pressed must remain true while held")
	}
}

func TestMouse_JustReleased_SingleFrame(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetButton(MouseButtonRight, true)
	s.Update()

	m.SetButton(MouseButtonRight, false)

	if !m.JustReleased(MouseButtonRight) {
		t.Fatal("Mouse JustReleased must be true after release, before UpdateFrame")
	}

	s.Update()

	if m.JustReleased(MouseButtonRight) {
		t.Fatal("Mouse JustReleased must be false after UpdateFrame")
	}
}

func TestMouse_MultiFrameLifecycle(t *testing.T) {
	s := New()
	m := s.Mouse()

	// Frame 1: press.
	m.SetButton(MouseButtonMiddle, true)
	if !m.JustPressed(MouseButtonMiddle) {
		t.Fatal("frame 1: JustPressed must be true")
	}

	s.Update()

	// Frame 2: hold.
	if m.JustPressed(MouseButtonMiddle) {
		t.Fatal("frame 2: JustPressed must be false (held)")
	}
	if !m.Pressed(MouseButtonMiddle) {
		t.Fatal("frame 2: Pressed must be true")
	}

	s.Update()

	// Frame 3: release.
	m.SetButton(MouseButtonMiddle, false)
	if !m.JustReleased(MouseButtonMiddle) {
		t.Fatal("frame 3: JustReleased must be true")
	}

	s.Update()

	// Frame 4: idle.
	if m.JustReleased(MouseButtonMiddle) {
		t.Fatal("frame 4: JustReleased must be false")
	}
	if m.Pressed(MouseButtonMiddle) {
		t.Fatal("frame 4: Pressed must be false")
	}
}

func TestMouse_PressAndReleaseSameFrame(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetButton(MouseButtonLeft, true)
	m.SetButton(MouseButtonLeft, false)

	// Last-write-wins: current=false, previous=false -> both edges false.
	if m.JustPressed(MouseButtonLeft) {
		t.Fatal("JustPressed must be false: transient press lost in same frame")
	}
	if m.JustReleased(MouseButtonLeft) {
		t.Fatal("JustReleased must be false: previous was also false")
	}
}

func TestMouse_OutOfRangeButton(t *testing.T) {
	s := New()
	m := s.Mouse()

	bigButton := MouseButtonCount + 5

	// Must not panic.
	m.SetButton(bigButton, true)

	if m.Pressed(bigButton) {
		t.Fatal("Pressed must return false for out-of-range button")
	}
	if m.JustPressed(bigButton) {
		t.Fatal("JustPressed must return false for out-of-range button")
	}
	if m.JustReleased(bigButton) {
		t.Fatal("JustReleased must return false for out-of-range button")
	}
}

func TestMouse_AllButtons(t *testing.T) {
	buttons := []struct {
		name   string
		button MouseButton
	}{
		{"Left", MouseButtonLeft},
		{"Right", MouseButtonRight},
		{"Middle", MouseButtonMiddle},
		{"Button4", MouseButton4},
		{"Button5", MouseButton5},
	}

	for _, tt := range buttons {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			m := s.Mouse()

			m.SetButton(tt.button, true)
			if !m.JustPressed(tt.button) {
				t.Fatalf("JustPressed(%s) must be true", tt.name)
			}
			if !m.Pressed(tt.button) {
				t.Fatalf("Pressed(%s) must be true", tt.name)
			}

			s.Update()

			m.SetButton(tt.button, false)
			if !m.JustReleased(tt.button) {
				t.Fatalf("JustReleased(%s) must be true", tt.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Mouse: position and delta
// ---------------------------------------------------------------------------

func TestMouse_Delta_BasicMovement(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetPosition(100, 200)
	s.Update()

	m.SetPosition(110, 205)

	dx, dy := m.Delta()
	if dx != 10 || dy != 5 {
		t.Fatalf("Delta = (%v, %v), want (10, 5)", dx, dy)
	}

	s.Update()

	dx, dy = m.Delta()
	if dx != 0 || dy != 0 {
		t.Fatalf("Delta after UpdateFrame with no movement = (%v, %v), want (0, 0)", dx, dy)
	}
}

func TestMouse_Delta_MultipleMovesAccumulateToFinalPosition(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetPosition(0, 0)
	s.Update()

	// Multiple SetPosition calls within one frame. SetPosition overwrites
	// (not accumulates), so delta is measured from prevX/Y to the LAST
	// position set.
	m.SetPosition(10, 20)
	m.SetPosition(30, 50)
	m.SetPosition(25, 40)

	dx, dy := m.Delta()
	if dx != 25 || dy != 40 {
		t.Fatalf("Delta = (%v, %v), want (25, 40) — last SetPosition minus prevX/Y", dx, dy)
	}
}

func TestMouse_Delta_NegativeMovement(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetPosition(100, 100)
	s.Update()

	m.SetPosition(90, 80)

	dx, dy := m.Delta()
	if dx != -10 || dy != -20 {
		t.Fatalf("Delta = (%v, %v), want (-10, -20)", dx, dy)
	}
}

func TestMouse_Delta_ZeroOnFirstFrameBeforeMovement(t *testing.T) {
	s := New()
	m := s.Mouse()

	// Before any SetPosition, both x and prevX are 0.
	dx, dy := m.Delta()
	if dx != 0 || dy != 0 {
		t.Fatalf("Delta on fresh state = (%v, %v), want (0, 0)", dx, dy)
	}
}

func TestMouse_Position_IndividualAccessors(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetPosition(42.5, 99.1)

	x, y := m.Position()
	if x != 42.5 || y != 99.1 {
		t.Fatalf("Position() = (%v, %v), want (42.5, 99.1)", x, y)
	}
	if m.X() != 42.5 {
		t.Fatalf("X() = %v, want 42.5", m.X())
	}
	if m.Y() != 99.1 {
		t.Fatalf("Y() = %v, want 99.1", m.Y())
	}
}

// ---------------------------------------------------------------------------
// Mouse: scroll
// ---------------------------------------------------------------------------

func TestMouse_Scroll_Accumulation(t *testing.T) {
	s := New()
	m := s.Mouse()

	// Multiple scroll events within one frame accumulate.
	m.SetScroll(0, 3)
	m.SetScroll(0, 2)
	m.SetScroll(1, 0)

	// Scroll() returns frameScrollX/Y, which is set by UpdateFrame.
	// Before the first UpdateFrame, frameScroll is 0 — accumulated
	// values are in scrollX/Y, not yet visible via Scroll().
	sx, sy := m.Scroll()
	if sx != 0 || sy != 0 {
		t.Fatalf("Scroll() before UpdateFrame = (%v, %v), want (0, 0) — not yet snapshotted", sx, sy)
	}

	s.Update()

	// After UpdateFrame: frameScroll = accumulated scroll, scroll reset to 0.
	sx, sy = m.Scroll()
	if sx != 1 || sy != 5 {
		t.Fatalf("Scroll() after UpdateFrame = (%v, %v), want (1, 5)", sx, sy)
	}

	s.Update()

	// No new scroll events -> frameScroll = 0.
	sx, sy = m.Scroll()
	if sx != 0 || sy != 0 {
		t.Fatalf("Scroll() with no new events = (%v, %v), want (0, 0)", sx, sy)
	}
}

func TestMouse_Scroll_NegativeValues(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetScroll(-1, -3)
	s.Update()

	sx, sy := m.Scroll()
	if sx != -1 || sy != -3 {
		t.Fatalf("Scroll() = (%v, %v), want (-1, -3)", sx, sy)
	}
}

func TestMouse_Scroll_MixedDirections(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetScroll(2, -1)
	m.SetScroll(-3, 4)
	s.Update()

	// Net: x = 2 + (-3) = -1, y = -1 + 4 = 3.
	sx, sy := m.Scroll()
	if sx != -1 || sy != 3 {
		t.Fatalf("Scroll() = (%v, %v), want (-1, 3)", sx, sy)
	}
}

// ---------------------------------------------------------------------------
// Ordering simulation: correct vs broken app loop
// ---------------------------------------------------------------------------

func TestAppLoop_CorrectOrder_EdgesVisible(t *testing.T) {
	// Correct order: platform events -> user reads edges -> Update.
	// This is the pattern the fix enforces.
	s := New()

	// --- Frame 1: platform delivers key press ---
	s.Keyboard().SetKey(KeySpace, true)
	s.Mouse().SetButton(MouseButtonLeft, true)
	s.Mouse().SetPosition(50, 60)

	// User callback reads edges BEFORE Update.
	if !s.Keyboard().JustPressed(KeySpace) {
		t.Fatal("correct order: keyboard JustPressed must be visible before Update")
	}
	if !s.Mouse().JustPressed(MouseButtonLeft) {
		t.Fatal("correct order: mouse JustPressed must be visible before Update")
	}

	// Frame advance AFTER user reads.
	s.Update()

	// --- Frame 2: platform delivers key release + mouse move ---
	s.Keyboard().SetKey(KeySpace, false)
	s.Mouse().SetPosition(70, 80)

	// User callback reads edges.
	if !s.Keyboard().JustReleased(KeySpace) {
		t.Fatal("correct order: keyboard JustReleased must be visible before Update")
	}

	dx, dy := s.Mouse().Delta()
	if dx != 20 || dy != 20 {
		t.Fatalf("correct order: mouse Delta = (%v, %v), want (20, 20)", dx, dy)
	}

	s.Update()
}

func TestAppLoop_BrokenOrder_EdgesLost(t *testing.T) {
	// Broken order: platform events -> Update -> user reads.
	// This was the original bug. Edges are always false because
	// Update copies current into previous before user reads.
	s := New()

	// Platform delivers press.
	s.Keyboard().SetKey(KeySpace, true)
	s.Mouse().SetButton(MouseButtonLeft, true)

	// BUG: Update called BEFORE user reads edges.
	s.Update()

	// User reads: previous == current now, so JustPressed = false.
	if s.Keyboard().JustPressed(KeySpace) {
		t.Fatal("broken order: JustPressed should be false (this is the bug scenario)")
	}
	if s.Mouse().JustPressed(MouseButtonLeft) {
		t.Fatal("broken order: mouse JustPressed should be false (this is the bug scenario)")
	}

	// Pressed is still true (the key IS held), but the EDGE was lost.
	if !s.Keyboard().Pressed(KeySpace) {
		t.Fatal("broken order: Pressed must still be true even though edge was lost")
	}
}

// ---------------------------------------------------------------------------
// State: independent keyboard and mouse
// ---------------------------------------------------------------------------

func TestState_KeyboardAndMouseIndependent(t *testing.T) {
	s := New()

	s.Keyboard().SetKey(KeyA, true)
	s.Mouse().SetButton(MouseButtonRight, true)

	if !s.Keyboard().JustPressed(KeyA) {
		t.Fatal("keyboard edge must be independent of mouse state")
	}
	if !s.Mouse().JustPressed(MouseButtonRight) {
		t.Fatal("mouse edge must be independent of keyboard state")
	}

	// Releasing keyboard must not affect mouse.
	s.Update()
	s.Keyboard().SetKey(KeyA, false)

	if !s.Keyboard().JustReleased(KeyA) {
		t.Fatal("keyboard JustReleased must work independently")
	}
	if s.Mouse().JustReleased(MouseButtonRight) {
		t.Fatal("mouse JustReleased must be false: mouse button still held")
	}
	if !s.Mouse().Pressed(MouseButtonRight) {
		t.Fatal("mouse Pressed must remain true")
	}
}

// ---------------------------------------------------------------------------
// State: zero-value initialization
// ---------------------------------------------------------------------------

func TestState_ZeroValueUsable(t *testing.T) {
	// State zero-value (without New()) should not panic. The struct fields
	// are value types (arrays, sync.RWMutex) so zero-init is valid.
	var s State

	// All queries return false/zero.
	if s.Keyboard().Pressed(KeyA) {
		t.Fatal("zero-value Pressed must be false")
	}
	if s.Keyboard().JustPressed(KeyA) {
		t.Fatal("zero-value JustPressed must be false")
	}
	if s.Keyboard().JustReleased(KeyA) {
		t.Fatal("zero-value JustReleased must be false")
	}
	if s.Keyboard().AnyPressed() {
		t.Fatal("zero-value AnyPressed must be false")
	}
	if s.Mouse().Pressed(MouseButtonLeft) {
		t.Fatal("zero-value mouse Pressed must be false")
	}

	dx, dy := s.Mouse().Delta()
	if dx != 0 || dy != 0 {
		t.Fatalf("zero-value mouse Delta = (%v, %v), want (0, 0)", dx, dy)
	}

	sx, sy := s.Mouse().Scroll()
	if sx != 0 || sy != 0 {
		t.Fatalf("zero-value mouse Scroll = (%v, %v), want (0, 0)", sx, sy)
	}

	// Update must not panic on zero-value.
	s.Update()
}

// ---------------------------------------------------------------------------
// Keyboard: rapid press-release-press across frames
// ---------------------------------------------------------------------------

func TestKeyboard_RapidToggle(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	// Frame 1: press.
	kb.SetKey(KeyF, true)
	if !kb.JustPressed(KeyF) {
		t.Fatal("frame 1: JustPressed must be true")
	}
	s.Update()

	// Frame 2: release.
	kb.SetKey(KeyF, false)
	if !kb.JustReleased(KeyF) {
		t.Fatal("frame 2: JustReleased must be true")
	}
	s.Update()

	// Frame 3: press again. This must register as a new JustPressed.
	kb.SetKey(KeyF, true)
	if !kb.JustPressed(KeyF) {
		t.Fatal("frame 3: JustPressed must be true for re-press")
	}
	if kb.JustReleased(KeyF) {
		t.Fatal("frame 3: JustReleased must be false")
	}
	s.Update()

	// Frame 4: release again.
	kb.SetKey(KeyF, false)
	if !kb.JustReleased(KeyF) {
		t.Fatal("frame 4: JustReleased must be true")
	}
	s.Update()

	// Frame 5: idle.
	if kb.JustPressed(KeyF) || kb.JustReleased(KeyF) || kb.Pressed(KeyF) {
		t.Fatal("frame 5: all must be false after idle")
	}
}

// ---------------------------------------------------------------------------
// Keyboard: table-driven edge detection
// ---------------------------------------------------------------------------

func TestKeyboard_EdgeDetection_Table(t *testing.T) {
	type frame struct {
		action       string // "press", "release", "none"
		wantJP       bool   // JustPressed
		wantJR       bool   // JustReleased
		wantPressed  bool   // Pressed
	}

	tests := []struct {
		name   string
		key    Key
		frames []frame
	}{
		{
			name: "press_hold_release_idle",
			key:  KeyG,
			frames: []frame{
				{"press", true, false, true},
				{"none", false, false, true},
				{"none", false, false, true},
				{"release", false, true, false},
				{"none", false, false, false},
			},
		},
		{
			name: "double_tap",
			key:  KeyH,
			frames: []frame{
				{"press", true, false, true},
				{"release", false, true, false},
				{"press", true, false, true},
				{"release", false, true, false},
				{"none", false, false, false},
			},
		},
		{
			name: "never_pressed",
			key:  KeyI,
			frames: []frame{
				{"none", false, false, false},
				{"none", false, false, false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			kb := s.Keyboard()

			for i, f := range tt.frames {
				switch f.action {
				case "press":
					kb.SetKey(tt.key, true)
				case "release":
					kb.SetKey(tt.key, false)
				case "none":
					// No input event this frame.
				}

				if got := kb.JustPressed(tt.key); got != f.wantJP {
					t.Fatalf("frame %d: JustPressed = %v, want %v", i+1, got, f.wantJP)
				}
				if got := kb.JustReleased(tt.key); got != f.wantJR {
					t.Fatalf("frame %d: JustReleased = %v, want %v", i+1, got, f.wantJR)
				}
				if got := kb.Pressed(tt.key); got != f.wantPressed {
					t.Fatalf("frame %d: Pressed = %v, want %v", i+1, got, f.wantPressed)
				}

				s.Update()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Mouse: table-driven button edge detection
// ---------------------------------------------------------------------------

func TestMouse_EdgeDetection_Table(t *testing.T) {
	type frame struct {
		action      string
		wantJP      bool
		wantJR      bool
		wantPressed bool
	}

	tests := []struct {
		name   string
		button MouseButton
		frames []frame
	}{
		{
			name:   "click_and_release",
			button: MouseButtonLeft,
			frames: []frame{
				{"press", true, false, true},
				{"none", false, false, true},
				{"release", false, true, false},
				{"none", false, false, false},
			},
		},
		{
			name:   "double_click",
			button: MouseButtonLeft,
			frames: []frame{
				{"press", true, false, true},
				{"release", false, true, false},
				{"press", true, false, true},
				{"release", false, true, false},
				{"none", false, false, false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			m := s.Mouse()

			for i, f := range tt.frames {
				switch f.action {
				case "press":
					m.SetButton(tt.button, true)
				case "release":
					m.SetButton(tt.button, false)
				case "none":
				}

				if got := m.JustPressed(tt.button); got != f.wantJP {
					t.Fatalf("frame %d: JustPressed = %v, want %v", i+1, got, f.wantJP)
				}
				if got := m.JustReleased(tt.button); got != f.wantJR {
					t.Fatalf("frame %d: JustReleased = %v, want %v", i+1, got, f.wantJR)
				}
				if got := m.Pressed(tt.button); got != f.wantPressed {
					t.Fatalf("frame %d: Pressed = %v, want %v", i+1, got, f.wantPressed)
				}

				s.Update()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Mouse: delta multi-frame table
// ---------------------------------------------------------------------------

func TestMouse_Delta_MultiFrame_Table(t *testing.T) {
	type frame struct {
		setX, setY float32
		move       bool // whether to call SetPosition this frame
		wantDX     float32
		wantDY     float32
	}

	frames := []frame{
		// Frame 1: initial position.
		{100, 200, true, 100, 200},
		// Frame 2: move right and down.
		{120, 210, true, 20, 10},
		// Frame 3: no movement.
		{0, 0, false, 0, 0},
		// Frame 4: large jump.
		{500, 100, true, 380, -110},
		// Frame 5: no movement.
		{0, 0, false, 0, 0},
	}

	s := New()
	m := s.Mouse()

	for i, f := range frames {
		if f.move {
			m.SetPosition(f.setX, f.setY)
		}

		dx, dy := m.Delta()
		if dx != f.wantDX || dy != f.wantDY {
			t.Fatalf("frame %d: Delta = (%v, %v), want (%v, %v)", i+1, dx, dy, f.wantDX, f.wantDY)
		}

		s.Update()
	}
}

// ---------------------------------------------------------------------------
// Mouse: scroll multi-frame
// ---------------------------------------------------------------------------

func TestMouse_Scroll_MultiFrame(t *testing.T) {
	s := New()
	m := s.Mouse()

	// Frame 1: accumulate scroll.
	m.SetScroll(0, 3)
	m.SetScroll(0, 2)
	s.Update()

	sx, sy := m.Scroll()
	if sx != 0 || sy != 5 {
		t.Fatalf("frame 1 Scroll = (%v, %v), want (0, 5)", sx, sy)
	}

	// Frame 2: new scroll events.
	m.SetScroll(1, -1)
	s.Update()

	sx, sy = m.Scroll()
	if sx != 1 || sy != -1 {
		t.Fatalf("frame 2 Scroll = (%v, %v), want (1, -1)", sx, sy)
	}

	// Frame 3: no scroll.
	s.Update()

	sx, sy = m.Scroll()
	if sx != 0 || sy != 0 {
		t.Fatalf("frame 3 Scroll = (%v, %v), want (0, 0)", sx, sy)
	}
}

// ---------------------------------------------------------------------------
// Thread safety: concurrent reads and writes
// ---------------------------------------------------------------------------

func TestKeyboard_ConcurrentAccess(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	const goroutines = 8
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers: simulate platform layer pushing key events.
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			key := KeyA + Key(id%26)
			for i := 0; i < iterations; i++ {
				kb.SetKey(key, i%2 == 0)
			}
		}(g)
	}

	// Readers: simulate user code polling edges.
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			key := KeyA + Key(id%26)
			for i := 0; i < iterations; i++ {
				_ = kb.JustPressed(key)
				_ = kb.JustReleased(key)
				_ = kb.Pressed(key)
				_ = kb.AnyPressed()
				_ = kb.Modifier(ModShift)
			}
		}(g)
	}

	wg.Wait()
}

func TestMouse_ConcurrentAccess(t *testing.T) {
	s := New()
	m := s.Mouse()

	const goroutines = 8
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers.
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				m.SetPosition(float32(i), float32(i*2))
				m.SetButton(MouseButtonLeft, i%2 == 0)
				m.SetScroll(0, float32(i%3))
			}
		}()
	}

	// Readers.
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_, _ = m.Position()
				_ = m.X()
				_ = m.Y()
				_, _ = m.Delta()
				_, _ = m.Scroll()
				_ = m.Pressed(MouseButtonLeft)
				_ = m.JustPressed(MouseButtonLeft)
				_ = m.JustReleased(MouseButtonLeft)
			}
		}()
	}

	wg.Wait()
}

func TestState_ConcurrentUpdateAndRead(t *testing.T) {
	s := New()

	const goroutines = 4
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(goroutines + 1)

	// One goroutine calls Update (simulates game loop).
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			s.Keyboard().SetKey(KeyW, true)
			s.Mouse().SetPosition(float32(i), float32(i))
			s.Update()
		}
	}()

	// Multiple reader goroutines.
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = s.Keyboard().JustPressed(KeyW)
				_ = s.Keyboard().Pressed(KeyW)
				_, _ = s.Mouse().Delta()
				_, _ = s.Mouse().Position()
			}
		}()
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Edge detection: edges survive exactly one read before Update
// ---------------------------------------------------------------------------

func TestKeyboard_EdgeSurvivesMultipleReads(t *testing.T) {
	s := New()
	kb := s.Keyboard()

	kb.SetKey(KeyR, true)

	// Reading JustPressed multiple times within the same frame must
	// return true every time -- it is not consumed by reading.
	for i := 0; i < 5; i++ {
		if !kb.JustPressed(KeyR) {
			t.Fatalf("read %d: JustPressed must remain true until UpdateFrame", i+1)
		}
	}

	s.Update()

	if kb.JustPressed(KeyR) {
		t.Fatal("JustPressed must be false after UpdateFrame")
	}
}

func TestMouse_EdgeSurvivesMultipleReads(t *testing.T) {
	s := New()
	m := s.Mouse()

	m.SetButton(MouseButtonLeft, true)

	for i := 0; i < 5; i++ {
		if !m.JustPressed(MouseButtonLeft) {
			t.Fatalf("read %d: JustPressed must remain true until UpdateFrame", i+1)
		}
	}

	s.Update()

	if m.JustPressed(MouseButtonLeft) {
		t.Fatal("JustPressed must be false after UpdateFrame")
	}
}

// ---------------------------------------------------------------------------
// Multiple Updates without events (idempotent)
// ---------------------------------------------------------------------------

func TestState_MultipleUpdatesNoEvents(t *testing.T) {
	s := New()

	s.Keyboard().SetKey(KeyA, true)
	s.Update()

	// Multiple Updates with no events should not alter state.
	s.Update()
	s.Update()
	s.Update()

	if !s.Keyboard().Pressed(KeyA) {
		t.Fatal("Pressed must remain true across multiple Updates with no events")
	}
	if s.Keyboard().JustPressed(KeyA) {
		t.Fatal("JustPressed must remain false: no new press event")
	}
}

// ---------------------------------------------------------------------------
// New() produces fresh state
// ---------------------------------------------------------------------------

func TestNew_FreshState(t *testing.T) {
	s := New()

	for key := Key(0); key < KeyCount; key++ {
		if s.Keyboard().Pressed(key) {
			t.Fatalf("New() state has key %d pressed", key)
		}
	}

	for btn := MouseButton(0); btn < MouseButtonCount; btn++ {
		if s.Mouse().Pressed(btn) {
			t.Fatalf("New() state has button %d pressed", btn)
		}
	}

	x, y := s.Mouse().Position()
	if x != 0 || y != 0 {
		t.Fatalf("New() mouse position = (%v, %v), want (0, 0)", x, y)
	}

	dx, dy := s.Mouse().Delta()
	if dx != 0 || dy != 0 {
		t.Fatalf("New() mouse delta = (%v, %v), want (0, 0)", dx, dy)
	}
}
