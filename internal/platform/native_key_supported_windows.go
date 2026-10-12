//go:build windows

package platform

import "golang.org/x/sys/windows"

// Older systems cannot inspect a dead key without mutating its native state.
var NativeKeysSupported = windows.RtlGetVersion().BuildNumber >= 14393
