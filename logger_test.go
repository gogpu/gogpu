package gogpu

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestNopHandler_Enabled(t *testing.T) {
	h := nopHandler{}
	for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		if h.Enabled(context.Background(), level) {
			t.Errorf("nopHandler.Enabled(%v) = true, want false", level)
		}
	}
}

func TestNopHandler_Handle(t *testing.T) {
	h := nopHandler{}
	if err := h.Handle(context.Background(), slog.Record{}); err != nil {
		t.Errorf("nopHandler.Handle() = %v, want nil", err)
	}
}

func TestLoggerDefaultSilent(t *testing.T) {
	l := Logger()
	if l == nil {
		t.Fatal("Logger() returned nil")
	}
	if l.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("default logger should not be enabled for Debug")
	}
}

func TestSetLogger(t *testing.T) {
	orig := Logger()
	t.Cleanup(func() { SetLogger(orig) })

	var buf bytes.Buffer
	custom := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	SetLogger(custom)

	got := Logger()
	if got != custom {
		t.Error("Logger() did not return the custom logger set via SetLogger")
	}

	got.Info("test message", "key", "value")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log output to contain 'test message', got: %s", buf.String())
	}
}

func TestSetLoggerNil(t *testing.T) {
	orig := Logger()
	t.Cleanup(func() { SetLogger(orig) })

	SetLogger(nil)

	l := Logger()
	if l == nil {
		t.Fatal("SetLogger(nil) should set nop logger, not nil")
	}
	if l.Enabled(context.Background(), slog.LevelError) {
		t.Error("SetLogger(nil) should produce a disabled logger")
	}
}

func TestLoggerConcurrentAccess(t *testing.T) {
	orig := Logger()
	t.Cleanup(func() { SetLogger(orig) })

	var wg sync.WaitGroup
	const goroutines = 100

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := Logger()
			if l == nil {
				t.Error("Logger() returned nil during concurrent access")
			}
		}()
	}

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetLogger(slog.Default())
			SetLogger(nil)
		}()
	}

	wg.Wait()
}

func TestSloggerReturnsCurrentLogger(t *testing.T) {
	orig := Logger()
	t.Cleanup(func() { SetLogger(orig) })

	custom := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	SetLogger(custom)

	got := slogger()
	if got != custom {
		t.Error("slogger() did not return the logger set via SetLogger")
	}
}
