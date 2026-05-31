//go:build linux

package platform

// Linux Phase 1 menu stub.
//
// Neither waylandPlatform nor x11Platform implements PlatMenuManager.
// App checks for PlatMenuManager via type assertion before calling menu
// methods, so all menu operations silently no-op on Linux.
//
