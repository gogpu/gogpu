//go:build linux

package platform

import "errors"

// linuxPlatform is a stub for Linux.
// TODO: Implement using X11 or Wayland
type linuxPlatform struct{}

func newPlatform() Platform {
	return &linuxPlatform{}
}

func (p *linuxPlatform) Init(config Config) error {
	return errors.New("linux platform not yet implemented")
}

func (p *linuxPlatform) PollEvents() Event {
	return Event{Type: EventNone}
}

func (p *linuxPlatform) ShouldClose() bool {
	return true
}

func (p *linuxPlatform) GetSize() (width, height int) {
	return 0, 0
}

func (p *linuxPlatform) GetHandle() (instance, window uintptr) {
	return 0, 0
}

func (p *linuxPlatform) Destroy() {}
