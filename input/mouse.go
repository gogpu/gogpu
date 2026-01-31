package input

import (
	"sync"
)

// MouseButton represents a mouse button.
type MouseButton uint8

const (
	MouseButtonLeft MouseButton = iota
	MouseButtonRight
	MouseButtonMiddle
	MouseButton4
	MouseButton5
	MouseButtonCount
)

// MouseState holds mouse input state.
// All methods are thread-safe.
type MouseState struct {
	mu               sync.RWMutex
	x, y             float32
	prevX, prevY     float32
	scrollX, scrollY float32
	current          [MouseButtonCount]bool
	previous         [MouseButtonCount]bool
}

func newMouseState() MouseState {
	return MouseState{}
}

// SetPosition sets mouse position (called by platform layer).
// Thread-safe.
func (m *MouseState) SetPosition(x, y float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.x = x
	m.y = y
}

// SetButton sets button state (called by platform layer).
// Thread-safe.
func (m *MouseState) SetButton(button MouseButton, pressed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if button < MouseButtonCount {
		m.current[button] = pressed
	}
}

// SetScroll sets scroll delta (called by platform layer).
// Thread-safe.
func (m *MouseState) SetScroll(x, y float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scrollX = x
	m.scrollY = y
}

// Position returns current mouse position.
// Thread-safe.
func (m *MouseState) Position() (x, y float32) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.x, m.y
}

// X returns current mouse X position.
// Thread-safe.
func (m *MouseState) X() float32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.x
}

// Y returns current mouse Y position.
// Thread-safe.
func (m *MouseState) Y() float32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.y
}

// Delta returns mouse movement since last frame.
// Thread-safe.
func (m *MouseState) Delta() (dx, dy float32) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.x - m.prevX, m.y - m.prevY
}

// Scroll returns scroll wheel delta.
// Thread-safe.
func (m *MouseState) Scroll() (x, y float32) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scrollX, m.scrollY
}

// Pressed returns true if button is currently pressed.
// Thread-safe.
func (m *MouseState) Pressed(button MouseButton) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if button >= MouseButtonCount {
		return false
	}
	return m.current[button]
}

// JustPressed returns true if button was just pressed this frame.
// Thread-safe.
func (m *MouseState) JustPressed(button MouseButton) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if button >= MouseButtonCount {
		return false
	}
	return m.current[button] && !m.previous[button]
}

// JustReleased returns true if button was just released this frame.
// Thread-safe.
func (m *MouseState) JustReleased(button MouseButton) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if button >= MouseButtonCount {
		return false
	}
	return !m.current[button] && m.previous[button]
}

// UpdateFrame advances the mouse state to the next frame.
// Call this once per frame before processing new events.
// Thread-safe.
func (m *MouseState) UpdateFrame() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.previous = m.current
	m.prevX = m.x
	m.prevY = m.y
	m.scrollX = 0
	m.scrollY = 0
}
