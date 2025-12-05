// Package input provides keyboard, mouse, and gamepad input handling.
package input

// State holds the current input state.
type State struct {
	keyboard KeyboardState
	mouse    MouseState
	// Gamepads will be added later
}

// New creates a new input state.
func New() *State {
	return &State{
		keyboard: newKeyboardState(),
		mouse:    newMouseState(),
	}
}

// Update should be called each frame to update input state.
func (s *State) Update() {
	s.keyboard.update()
	s.mouse.update()
}

// Keyboard returns the keyboard state.
func (s *State) Keyboard() *KeyboardState {
	return &s.keyboard
}

// Mouse returns the mouse state.
func (s *State) Mouse() *MouseState {
	return &s.mouse
}
