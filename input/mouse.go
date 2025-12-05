package input

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
type MouseState struct {
	x, y             float32
	prevX, prevY     float32
	scrollX, scrollY float32
	current          [MouseButtonCount]bool
	previous         [MouseButtonCount]bool
}

func newMouseState() MouseState {
	return MouseState{}
}

func (m *MouseState) update() {
	m.previous = m.current
	m.prevX = m.x
	m.prevY = m.y
	m.scrollX = 0
	m.scrollY = 0
}

// SetPosition sets mouse position (called by platform layer).
func (m *MouseState) SetPosition(x, y float32) {
	m.x = x
	m.y = y
}

// SetButton sets button state (called by platform layer).
func (m *MouseState) SetButton(button MouseButton, pressed bool) {
	if button < MouseButtonCount {
		m.current[button] = pressed
	}
}

// SetScroll sets scroll delta (called by platform layer).
func (m *MouseState) SetScroll(x, y float32) {
	m.scrollX = x
	m.scrollY = y
}

// Position returns current mouse position.
func (m *MouseState) Position() (x, y float32) {
	return m.x, m.y
}

// X returns current mouse X position.
func (m *MouseState) X() float32 {
	return m.x
}

// Y returns current mouse Y position.
func (m *MouseState) Y() float32 {
	return m.y
}

// Delta returns mouse movement since last frame.
func (m *MouseState) Delta() (dx, dy float32) {
	return m.x - m.prevX, m.y - m.prevY
}

// Scroll returns scroll wheel delta.
func (m *MouseState) Scroll() (x, y float32) {
	return m.scrollX, m.scrollY
}

// Pressed returns true if button is currently pressed.
func (m *MouseState) Pressed(button MouseButton) bool {
	if button >= MouseButtonCount {
		return false
	}
	return m.current[button]
}

// JustPressed returns true if button was just pressed this frame.
func (m *MouseState) JustPressed(button MouseButton) bool {
	if button >= MouseButtonCount {
		return false
	}
	return m.current[button] && !m.previous[button]
}

// JustReleased returns true if button was just released this frame.
func (m *MouseState) JustReleased(button MouseButton) bool {
	if button >= MouseButtonCount {
		return false
	}
	return !m.current[button] && m.previous[button]
}
