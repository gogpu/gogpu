package input

import "testing"

// TestStateResetClearsKeyboardAndMouse pins the focus-loss contract: after
// Reset no key, modifier or button is pressed, and the frame history is empty,
// so a held key's lost release does not surface as JustReleased on the next
// frame.
func TestStateResetClearsKeyboardAndMouse(t *testing.T) {
	s := New()
	s.Keyboard().SetKey(KeyA, true)
	s.Keyboard().SetKey(KeyControlLeft, true)
	s.Mouse().SetButton(MouseButtonLeft, true)
	s.Update() // the presses become the previous frame

	if !s.Keyboard().Pressed(KeyA) || !s.Keyboard().Modifier(ModControl) {
		t.Fatal("precondition: key and modifier should be pressed")
	}
	if !s.Mouse().Pressed(MouseButtonLeft) {
		t.Fatal("precondition: mouse button should be pressed")
	}

	s.Reset()

	if s.Keyboard().Pressed(KeyA) {
		t.Error("Reset left KeyA pressed")
	}
	if s.Keyboard().AnyPressed() {
		t.Error("Reset left some key pressed")
	}
	if s.Keyboard().Modifier(ModControl) {
		t.Error("Reset left the Control modifier pressed")
	}
	if s.Mouse().Pressed(MouseButtonLeft) {
		t.Error("Reset left the left mouse button pressed")
	}

	s.Update()
	if s.Keyboard().JustPressed(KeyA) || s.Keyboard().JustReleased(KeyA) {
		t.Error("Reset left keyboard frame history: a press or release would surface on the next frame")
	}
	if s.Mouse().JustPressed(MouseButtonLeft) || s.Mouse().JustReleased(MouseButtonLeft) {
		t.Error("Reset left mouse frame history: a press or release would surface on the next frame")
	}
}

// TestStateResetKeepsPointerPosition: the pointer is still where it is; only
// the button releases may have been lost.
func TestStateResetKeepsPointerPosition(t *testing.T) {
	s := New()
	s.Mouse().SetPosition(120, 80)
	s.Mouse().SetButton(MouseButtonRight, true)

	s.Reset()

	if x, y := s.Mouse().Position(); x != 120 || y != 80 {
		t.Errorf("Position after Reset = (%v, %v), want (120, 80)", x, y)
	}
	s.Update()
	if dx, dy := s.Mouse().Delta(); dx != 0 || dy != 0 {
		t.Errorf("Delta after Reset = (%v, %v), want (0, 0): the reset must not move the pointer", dx, dy)
	}
}
