//go:build !(js && wasm)

package gogpu

import (
	"errors"

	"github.com/gogpu/wgpu/hal"
)

// isZeroArea reports whether a surface configure failed on a zero extent,
// as a minimized window's surface does before the platform reports the
// minimize.
func isZeroArea(err error) bool { return errors.Is(err, hal.ErrZeroArea) }
