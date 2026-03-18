package gogpu

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/wgpu"
)

// nopHandler silently discards all log records.
type nopHandler struct{}

func (nopHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (nopHandler) Handle(context.Context, slog.Record) error { return nil }
func (nopHandler) WithAttrs([]slog.Attr) slog.Handler        { return nopHandler{} }
func (nopHandler) WithGroup(string) slog.Handler             { return nopHandler{} }

// loggerPtr stores the active logger. Accessed atomically for thread safety.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	l := slog.New(nopHandler{})
	loggerPtr.Store(l)
}

// slogger returns the current package logger.
func slogger() *slog.Logger { return loggerPtr.Load() }

// SetLogger configures the logger for gogpu.
// By default, gogpu produces no log output. Call SetLogger to enable logging.
//
// SetLogger is safe for concurrent use.
// Pass nil to disable logging (restore default silent behavior).
//
// Log levels used by gogpu:
//   - [slog.LevelDebug]: internal diagnostics (texture creation, pipeline state)
//   - [slog.LevelInfo]: important lifecycle events (backend selected, adapter info)
//   - [slog.LevelWarn]: non-fatal issues (resource cleanup errors)
//
// Example:
//
//	// Enable info-level logging to stderr:
//	gogpu.SetLogger(slog.Default())
//
//	// Enable debug-level logging for full diagnostics:
//	gogpu.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
//	    Level: slog.LevelDebug,
//	})))
func SetLogger(l *slog.Logger) {
	if l == nil {
		l = slog.New(nopHandler{})
	}
	loggerPtr.Store(l)

	// Propagate to all subsystems so a single SetLogger() call
	// enables logging across the entire stack.
	platform.SetLogger(l)
	wgpu.SetLogger(l)
}

// Logger returns the current logger used by gogpu.
// Logger is safe for concurrent use.
func Logger() *slog.Logger {
	return loggerPtr.Load()
}
