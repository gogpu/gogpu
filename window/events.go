package window

// EventType represents different window event types.
type EventType uint8

const (
	EventTypeNone EventType = iota
	EventTypeClose
	EventTypeResize
	EventTypeFocus
	EventTypeBlur
	EventTypeMove
	EventTypeMinimize
	EventTypeMaximize
	EventTypeRestore
	EventTypeDropFile
)

// Event represents a window event.
type Event struct {
	Type EventType
	// Data depends on EventType
	Width  int      // for Resize
	Height int      // for Resize
	X      int      // for Move
	Y      int      // for Move
	Files  []string // for DropFile
}

// EventHandler is a function that handles window events.
type EventHandler func(Event)

// OnClose registers a handler for close events.
func (w *Window) OnClose(handler func()) {
	// TODO: Register platform callback
}

// OnResize registers a handler for resize events.
func (w *Window) OnResize(handler func(width, height int)) {
	// TODO: Register platform callback
}

// OnFocus registers a handler for focus events.
func (w *Window) OnFocus(handler func(focused bool)) {
	// TODO: Register platform callback
}

// OnDropFile registers a handler for file drop events.
func (w *Window) OnDropFile(handler func(files []string)) {
	// TODO: Register platform callback
}
