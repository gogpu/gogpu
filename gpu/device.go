package gpu

// DeviceDescriptor describes how to create a device.
type DeviceDescriptor struct {
	Label           string
	RequiredFeatures []string
	RequiredLimits   *Limits
}

// Device represents a GPU device.
// It is the main entry point for creating GPU resources.
type Device struct {
	label    string
	features Features
	limits   Limits
	// Internal handle will be added when integrating with webgpu
}

// Label returns the device label.
func (d *Device) Label() string {
	return d.label
}

// Features returns the device features.
func (d *Device) Features() Features {
	return d.features
}

// Limits returns the device limits.
func (d *Device) Limits() Limits {
	return d.limits
}

// Adapter represents a GPU adapter (physical device).
type Adapter struct {
	name     string
	vendor   string
	backend  Backend
	features Features
	limits   Limits
}

// Name returns the adapter name.
func (a *Adapter) Name() string {
	return a.name
}

// Vendor returns the adapter vendor.
func (a *Adapter) Vendor() string {
	return a.vendor
}

// Backend returns the graphics backend.
func (a *Adapter) Backend() Backend {
	return a.backend
}

// Features returns supported features.
func (a *Adapter) Features() Features {
	return a.features
}

// Limits returns adapter limits.
func (a *Adapter) Limits() Limits {
	return a.limits
}

// RequestDevice creates a new device from the adapter.
func (a *Adapter) RequestDevice(desc *DeviceDescriptor) (*Device, error) {
	// TODO: Implement with webgpu bindings
	return nil, ErrDeviceCreation
}
