package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/gogpu/internal/platform"
)

// TestApp_FocusLossResetsInputState pins issue #571: the release of a key or
// button held while the focus goes away is delivered to the new focus owner,
// so the state must be cleared on the focus-loss event, not left stuck.
func TestApp_FocusLossResetsInputState(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
		inputState:    input.New(),
	}

	platformID := platform.NewWindowID()
	internalID := app.windowManager.allocate()
	app.windowManager.add(&Window{id: internalID, platformID: platformID, visible: true})

	app.inputState.Keyboard().SetKey(input.KeyAltLeft, true)
	app.inputState.Keyboard().SetKey(input.KeyA, true)
	app.inputState.Mouse().SetButton(input.MouseButtonLeft, true)

	app.classifyEvent(
		&platform.Event{
			Type:     platform.EventFocus,
			WindowID: platformID,
			Focused:  false,
		}, nil, nil,
	)

	if app.inputState.Keyboard().Pressed(input.KeyAltLeft) {
		t.Error("focus loss left Alt pressed")
	}
	if app.inputState.Keyboard().AnyPressed() {
		t.Error("focus loss left some key pressed")
	}
	if app.inputState.Keyboard().Modifier(input.ModAlt) {
		t.Error("focus loss left the Alt modifier stuck")
	}
	if app.inputState.Mouse().Pressed(input.MouseButtonLeft) {
		t.Error("focus loss left the left mouse button pressed")
	}
}

// TestApp_FocusGainKeepsInputState: gaining the focus must not clear what the
// app already knows; only the loss resets.
func TestApp_FocusGainKeepsInputState(t *testing.T) {
	app := &App{
		windowManager: newWindowManager(),
		renderLoop:    &mockRenderLoop{},
		inputState:    input.New(),
	}

	platformID := platform.NewWindowID()
	internalID := app.windowManager.allocate()
	app.windowManager.add(&Window{id: internalID, platformID: platformID, visible: true})

	app.inputState.Keyboard().SetKey(input.KeyA, true)

	app.classifyEvent(
		&platform.Event{
			Type:     platform.EventFocus,
			WindowID: platformID,
			Focused:  true,
		}, nil, nil,
	)

	if !app.inputState.Keyboard().Pressed(input.KeyA) {
		t.Error("focus gain cleared a pressed key")
	}
}
