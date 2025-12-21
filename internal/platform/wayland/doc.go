//go:build linux

// Package wayland implements a pure Go Wayland client.
//
// This package provides a Wayland client implementation that communicates
// directly with the Wayland compositor over Unix sockets, without using
// libwayland-client.so. This enables zero-CGO builds on Linux.
//
// # Wire Protocol
//
// Wayland uses a binary wire protocol over Unix domain sockets. Messages
// consist of a header (object ID + size/opcode) followed by arguments.
// All values are encoded as 32-bit little-endian words.
//
// The wire format is:
//
//	+--------+--------+--------+--------+
//	| Object ID (4 bytes)               |
//	+--------+--------+--------+--------+
//	| Size (16 bits) | Opcode (16 bits) |
//	+--------+--------+--------+--------+
//	| Arguments...                      |
//	+--------+--------+--------+--------+
//
// # Argument Types
//
// The protocol supports several argument types:
//   - int: Signed 32-bit integer
//   - uint: Unsigned 32-bit integer
//   - fixed: Signed 24.8 fixed-point number
//   - string: Length-prefixed UTF-8 string (padded to 4 bytes)
//   - object: Object ID (uint32)
//   - new_id: New object ID (uint32), sometimes with interface+version
//   - array: Length-prefixed byte array (padded to 4 bytes)
//   - fd: File descriptor (passed via SCM_RIGHTS)
//
// # Core Interfaces
//
// This package implements the core Wayland interfaces:
//   - wl_display: The connection to the compositor (object ID 1)
//   - wl_registry: Global registry for binding to interfaces
//
// Additional interfaces (wl_compositor, wl_surface, xdg_wm_base, etc.)
// are implemented in separate files.
//
// # Usage
//
// Connect to the compositor and bind to required interfaces:
//
//	display, err := wayland.Connect()
//	if err != nil {
//	    return err
//	}
//	defer display.Close()
//
//	registry, err := display.GetRegistry()
//	if err != nil {
//	    return err
//	}
//
//	// Wait for globals to be advertised
//	display.Roundtrip()
//
// # File Descriptors
//
// Wayland uses SCM_RIGHTS to pass file descriptors for shared memory
// buffers and DMA-BUF handles. This requires special socket handling
// via the golang.org/x/sys/unix package.
//
// # Thread Safety
//
// The Display type is safe for concurrent use from multiple goroutines.
// Individual objects should be accessed from a single goroutine or
// protected by external synchronization.
package wayland
