//go:build linux

package platform

import "log/slog"

func logger() *slog.Logger { return loggerPtr.Load() }
