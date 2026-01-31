// Package input provides keyboard, mouse, and gamepad input handling.
// All methods are thread-safe for use with Ebiten-style polling from
// multiple goroutines.
package input

// State holds the current input state.
// All methods are thread-safe.
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
// This advances the "just pressed/released" tracking to the next frame.
// Thread-safe.
func (s *State) Update() {
	// Use the thread-safe UpdateFrame methods
	s.keyboard.UpdateFrame()
	s.mouse.UpdateFrame()
}

// Keyboard returns the keyboard state.
func (s *State) Keyboard() *KeyboardState {
	return &s.keyboard
}

// Mouse returns the mouse state.
func (s *State) Mouse() *MouseState {
	return &s.mouse
}
