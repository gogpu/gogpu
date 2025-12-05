//go:build darwin

package platform

import "errors"

// darwinPlatform is a stub for macOS.
// TODO: Implement using Cocoa/AppKit
type darwinPlatform struct{}

func newPlatform() Platform {
	return &darwinPlatform{}
}

func (p *darwinPlatform) Init(config Config) error {
	return errors.New("macOS platform not yet implemented")
}

func (p *darwinPlatform) PollEvents() Event {
	return Event{Type: EventNone}
}

func (p *darwinPlatform) ShouldClose() bool {
	return true
}

func (p *darwinPlatform) GetSize() (width, height int) {
	return 0, 0
}

func (p *darwinPlatform) GetHandle() (instance, window uintptr) {
	return 0, 0
}

func (p *darwinPlatform) Destroy() {}
