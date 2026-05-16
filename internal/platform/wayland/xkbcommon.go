//go:build linux

package wayland

import (
	"github.com/gogpu/gogpu/internal/platform/xkb"
)

// XKBHandle is the shared xkbcommon wrapper for keyboard layout handling.
// Exposed as a type alias so that Wayland platform code and tests can use
// the type directly without importing the internal xkb package.
type XKBHandle = xkb.Handle

// LoadXKBCommon loads libxkbcommon.so.0 and creates an xkb_context.
// Returns (nil, error) if the library is not available -- caller should fall back gracefully.
func LoadXKBCommon() (*XKBHandle, error) {
	return xkb.New()
}
