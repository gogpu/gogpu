package gogpu

import (
	"errors"
	"testing"
)

type mockLayerShellWindow struct {
	mockWindow
	keyboardMode uint32
	interactive  bool
	failKeyboard bool
	failInput    bool
}

func (m *mockLayerShellWindow) SetLayerKeyboardInteractivity(mode uint32) error {
	if m.failKeyboard {
		return errors.New("keyboard error")
	}
	m.keyboardMode = mode
	return nil
}

func (m *mockLayerShellWindow) SetInputRegion(interactive bool) error {
	if m.failInput {
		return errors.New("input region error")
	}
	m.interactive = interactive
	return nil
}

func TestAppSetLayerKeyboardInteractivity(t *testing.T) {
	// 1. platWindow == nil
	app := &App{}
	if err := app.SetLayerKeyboardInteractivity(LayerShellKeyboardOnDemand); err == nil {
		t.Error("expected error when platWindow is nil")
	}

	// 2. Window does not support layer shell
	app.platWindow = &mockWindow{}
	if err := app.SetLayerKeyboardInteractivity(LayerShellKeyboardOnDemand); err == nil {
		t.Error("expected error when platWindow does not support layer shell")
	}

	// 3. Supported window
	mockLS := &mockLayerShellWindow{}
	app.platWindow = mockLS
	if err := app.SetLayerKeyboardInteractivity(LayerShellKeyboardOnDemand); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockLS.keyboardMode != uint32(LayerShellKeyboardOnDemand) {
		t.Errorf("keyboardMode = %d, want %d", mockLS.keyboardMode, LayerShellKeyboardOnDemand)
	}

	// 4. Propagates underlying error
	mockLS.failKeyboard = true
	if err := app.SetLayerKeyboardInteractivity(LayerShellKeyboardExclusive); err == nil {
		t.Error("expected error from underlying window")
	}
}

func TestAppSetInputRegion(t *testing.T) {
	// 1. platWindow == nil
	app := &App{}
	if err := app.SetInputRegion(false); err == nil {
		t.Error("expected error when platWindow is nil")
	}

	// 2. Window does not support layer shell input regions
	app.platWindow = &mockWindow{}
	if err := app.SetInputRegion(false); err == nil {
		t.Error("expected error when platWindow does not support layer shell")
	}

	// 3. Supported window
	mockLS := &mockLayerShellWindow{}
	app.platWindow = mockLS
	if err := app.SetInputRegion(false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockLS.interactive {
		t.Error("expected interactive = false")
	}
	if err := app.SetInputRegion(true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mockLS.interactive {
		t.Error("expected interactive = true")
	}

	// 4. Propagates underlying error
	mockLS.failInput = true
	if err := app.SetInputRegion(true); err == nil {
		t.Error("expected error from underlying window")
	}
}
