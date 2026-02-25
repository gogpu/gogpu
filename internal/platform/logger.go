package platform

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// nopHandler silently discards all log records.
type nopHandler struct{}

func (nopHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (nopHandler) Handle(context.Context, slog.Record) error { return nil }
func (nopHandler) WithAttrs([]slog.Attr) slog.Handler        { return nopHandler{} }
func (nopHandler) WithGroup(string) slog.Handler             { return nopHandler{} }

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	l := slog.New(nopHandler{})
	loggerPtr.Store(l)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger sets the logger for the platform package.
// Pass nil to restore default silent behavior.
func SetLogger(l *slog.Logger) {
	if l == nil {
		l = slog.New(nopHandler{})
	}
	loggerPtr.Store(l)
}
