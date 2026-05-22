package gogpu

import "testing"

func TestAppLifecycle_IsActive(t *testing.T) {
	tests := []struct {
		state  AppLifecycle
		active bool
	}{
		{AppIdle, false},
		{AppRunning, true},
		{AppSuspending, true},
		{AppSuspended, false},
		{AppResuming, true},
	}
	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.IsActive(); got != tt.active {
				t.Errorf("AppLifecycle(%d).IsActive() = %v, want %v", tt.state, got, tt.active)
			}
		})
	}
}

func TestAppLifecycle_String(t *testing.T) {
	tests := []struct {
		state AppLifecycle
		want  string
	}{
		{AppIdle, "Idle"},
		{AppRunning, "Running"},
		{AppSuspending, "Suspending"},
		{AppSuspended, "Suspended"},
		{AppResuming, "Resuming"},
		{AppLifecycle(99), "Unknown lifecycle"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApp_Lifecycle_DefaultIdle(t *testing.T) {
	app := NewApp(DefaultConfig())
	if app.Lifecycle() != AppIdle {
		t.Errorf("new App lifecycle = %v, want AppIdle", app.Lifecycle())
	}
}

func TestApp_LifecycleCallbacks_Chainable(t *testing.T) {
	app := NewApp(DefaultConfig())

	result := app.
		OnSurfaceAvailable(func() {}).
		OnSurfaceDestroyed(func() {}).
		OnResumed(func() {}).
		OnSuspended(func() {}).
		OnMemoryWarning(func() {})

	if result != app {
		t.Error("lifecycle callbacks should return *App for chaining")
	}
}

func TestApp_LifecycleCallbacks_NilSafe(t *testing.T) {
	app := NewApp(DefaultConfig())

	// All callbacks nil by default — no panic when not set
	if app.onSurfaceAvailable != nil {
		t.Error("onSurfaceAvailable should be nil by default")
	}
	if app.onSurfaceDestroyed != nil {
		t.Error("onSurfaceDestroyed should be nil by default")
	}
	if app.onResumed != nil {
		t.Error("onResumed should be nil by default")
	}
	if app.onSuspended != nil {
		t.Error("onSuspended should be nil by default")
	}
	if app.onMemoryWarning != nil {
		t.Error("onMemoryWarning should be nil by default")
	}
}
