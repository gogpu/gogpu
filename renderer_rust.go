//go:build rust && windows

package gogpu

import (
	// Register rust backend when built with -tags rust on Windows
	_ "github.com/gogpu/gogpu/gpu/backend/rust"
)
