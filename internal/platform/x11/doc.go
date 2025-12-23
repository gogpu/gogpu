//go:build linux

// Package x11 implements a pure Go X11 client for window creation.
//
// This package provides X11 windowing support by implementing the X11
// wire protocol directly in Go, communicating with the X server over
// Unix domain sockets. No CGO or external libraries are required.
//
// # Wire Protocol
//
// X11 uses a binary wire protocol over Unix sockets (local) or TCP (remote).
// The client can choose byte order ('B' for big-endian, 'l' for little-endian).
// This implementation uses little-endian (LSBFirst) as default.
//
// The wire format is:
//
//	Request:  [opcode:1][data:1][length:2] + args
//	Reply:    [1:1][data:1][seq:2][length:4][data:24] = 32 bytes base
//	Event:    [code:1][data:31] = 32 bytes fixed
//	Error:    [0:1][code:1][seq:2][data:28] = 32 bytes fixed
//
// # Authentication
//
// X11 uses MIT-MAGIC-COOKIE-1 authentication. The cookie is read from
// the .Xauthority file (at $XAUTHORITY or $HOME/.Xauthority).
//
// # Core Operations
//
// This package implements a minimal subset of X11 for windowing:
//   - Connection setup with authentication
//   - Window creation, mapping, and destruction
//   - Atom interning for properties (WM_DELETE_WINDOW, etc.)
//   - Basic event handling (Expose, ConfigureNotify, KeyPress, etc.)
//
// # Vulkan Surface
//
// For VK_KHR_xlib_surface, you need:
//   - Display: The connection file descriptor (returned by GetHandle)
//   - Window: The X11 window ID (uint32)
//
// Note: This pure Go implementation returns the socket FD as the "display"
// handle. This works with some Vulkan implementations that accept raw FDs.
//
// # Thread Safety
//
// The Connection type is safe for concurrent request sending (protected by
// mutex). Event handling should be done from a single goroutine for
// deterministic ordering.
//
// # Build Tags
//
// This package only builds on linux:
//
//	//go:build linux
//
// # Example
//
//	conn, err := x11.Connect("")
//	if err != nil {
//	    return err
//	}
//	defer conn.Close()
//
//	window, err := conn.CreateWindow(800, 600, "My App")
//	if err != nil {
//	    return err
//	}
//	conn.MapWindow(window)
//
//	for {
//	    event, err := conn.WaitForEvent()
//	    if err != nil {
//	        break
//	    }
//	    switch e := event.(type) {
//	    case *x11.ClientMessageEvent:
//	        if e.IsDeleteWindow() {
//	            return
//	        }
//	    }
//	}
//
// # References
//
//   - X11 Protocol: https://www.x.org/releases/X11R7.7/doc/xproto/x11protocol.html
//   - jezek/xgb (reference): https://github.com/jezek/xgb
//   - ICCCM: https://tronche.com/gui/x/icccm/
//   - EWMH: https://specifications.freedesktop.org/wm-spec/wm-spec-latest.html
package x11
