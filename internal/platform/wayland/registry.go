//go:build linux

package wayland

import (
	"fmt"
	"sync"
)

// wl_registry opcodes (requests)
const (
	registryBind Opcode = 0 // bind(name: uint, id: new_id)
)

// wl_registry event opcodes
const (
	registryEventGlobal       Opcode = 0 // global(name: uint, interface: string, version: uint)
	registryEventGlobalRemove Opcode = 1 // global_remove(name: uint)
)

// Well-known Wayland interface names.
const (
	InterfaceWlCompositor   = "wl_compositor"
	InterfaceWlShm          = "wl_shm"
	InterfaceWlSeat         = "wl_seat"
	InterfaceWlOutput       = "wl_output"
	InterfaceXdgWmBase      = "xdg_wm_base"
	InterfaceWlSubcompositor = "wl_subcompositor"
	InterfaceWlDataDeviceManager = "wl_data_device_manager"
	InterfaceZwpLinuxDmabuf = "zwp_linux_dmabuf_v1"
)

// Global represents a Wayland global interface advertised by the compositor.
type Global struct {
	// Name is the unique identifier for this global (used for binding).
	Name uint32

	// Interface is the interface name (e.g., "wl_compositor").
	Interface string

	// Version is the interface version supported by the compositor.
	Version uint32
}

// Registry represents the wl_registry interface.
// It receives advertisements of global interfaces from the compositor
// and allows clients to bind to them.
type Registry struct {
	display *Display
	id      ObjectID

	mu      sync.RWMutex
	globals map[uint32]*Global // name -> Global

	// Event handlers
	onGlobal       func(global *Global)
	onGlobalRemove func(name uint32)
}

// newRegistry creates a new Registry instance.
func newRegistry(display *Display, id ObjectID) *Registry {
	return &Registry{
		display: display,
		id:      id,
		globals: make(map[uint32]*Global),
	}
}

// ID returns the object ID of the registry.
func (r *Registry) ID() ObjectID {
	return r.id
}

// Bind binds to a global interface, creating a new object.
// Returns the object ID of the newly created object.
//
// Parameters:
//   - name: The name of the global (from the global event)
//   - iface: The interface name (must match the global's interface)
//   - version: The version to bind (must be <= global's version)
func (r *Registry) Bind(name uint32, iface string, version uint32) (ObjectID, error) {
	r.mu.RLock()
	global, ok := r.globals[name]
	r.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("wayland: global %d not found", name)
	}

	if global.Interface != iface {
		return 0, fmt.Errorf("wayland: interface mismatch: expected %s, got %s",
			global.Interface, iface)
	}

	if version > global.Version {
		return 0, fmt.Errorf("wayland: version %d exceeds available %d for %s",
			version, global.Version, iface)
	}

	objectID := r.display.AllocID()

	// Build bind request: bind(name: uint, id: new_id)
	// For new_id without type, we send: interface (string), version (uint), id (uint)
	builder := NewMessageBuilder()
	builder.PutUint32(name)
	builder.PutNewIDFull(iface, version, objectID)

	msg := builder.BuildMessage(r.id, registryBind)

	if err := r.display.SendMessage(msg); err != nil {
		return 0, err
	}

	return objectID, nil
}

// BindCompositor binds to the wl_compositor global.
func (r *Registry) BindCompositor(version uint32) (ObjectID, error) {
	name, err := r.FindGlobal(InterfaceWlCompositor)
	if err != nil {
		return 0, err
	}
	return r.Bind(name, InterfaceWlCompositor, version)
}

// BindShm binds to the wl_shm global.
func (r *Registry) BindShm(version uint32) (ObjectID, error) {
	name, err := r.FindGlobal(InterfaceWlShm)
	if err != nil {
		return 0, err
	}
	return r.Bind(name, InterfaceWlShm, version)
}

// BindSeat binds to the wl_seat global.
func (r *Registry) BindSeat(version uint32) (ObjectID, error) {
	name, err := r.FindGlobal(InterfaceWlSeat)
	if err != nil {
		return 0, err
	}
	return r.Bind(name, InterfaceWlSeat, version)
}

// BindXdgWmBase binds to the xdg_wm_base global.
func (r *Registry) BindXdgWmBase(version uint32) (ObjectID, error) {
	name, err := r.FindGlobal(InterfaceXdgWmBase)
	if err != nil {
		return 0, err
	}
	return r.Bind(name, InterfaceXdgWmBase, version)
}

// FindGlobal finds a global by interface name and returns its name.
// Returns an error if the global is not found.
func (r *Registry) FindGlobal(iface string) (uint32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, g := range r.globals {
		if g.Interface == iface {
			return g.Name, nil
		}
	}

	return 0, fmt.Errorf("wayland: global %s not found", iface)
}

// GetGlobal returns a global by name.
// Returns nil if the global is not found.
func (r *Registry) GetGlobal(name uint32) *Global {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.globals[name]
}

// GetGlobalByInterface returns a global by interface name.
// Returns nil if the global is not found.
func (r *Registry) GetGlobalByInterface(iface string) *Global {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, g := range r.globals {
		if g.Interface == iface {
			return g
		}
	}
	return nil
}

// ListGlobals returns a copy of all known globals.
func (r *Registry) ListGlobals() []*Global {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Global, 0, len(r.globals))
	for _, g := range r.globals {
		// Return a copy to prevent mutation
		globalCopy := *g
		result = append(result, &globalCopy)
	}
	return result
}

// HasGlobal returns true if a global with the given interface exists.
func (r *Registry) HasGlobal(iface string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, g := range r.globals {
		if g.Interface == iface {
			return true
		}
	}
	return false
}

// GlobalVersion returns the version of a global interface.
// Returns 0 if the global is not found.
func (r *Registry) GlobalVersion(iface string) uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, g := range r.globals {
		if g.Interface == iface {
			return g.Version
		}
	}
	return 0
}

// SetGlobalHandler sets a callback for the global event.
// The handler is called when the compositor advertises a new global.
func (r *Registry) SetGlobalHandler(handler func(global *Global)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onGlobal = handler
}

// SetGlobalRemoveHandler sets a callback for the global_remove event.
// The handler is called when a global is no longer available.
func (r *Registry) SetGlobalRemoveHandler(handler func(name uint32)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onGlobalRemove = handler
}

// dispatch handles registry events.
func (r *Registry) dispatch(msg *Message) error {
	switch msg.Opcode {
	case registryEventGlobal:
		return r.handleGlobal(msg)

	case registryEventGlobalRemove:
		return r.handleGlobalRemove(msg)

	default:
		// Unknown event - ignore
		return nil
	}
}

// handleGlobal handles the wl_registry.global event.
func (r *Registry) handleGlobal(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	name, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: registry.global: failed to decode name: %w", err)
	}

	iface, err := decoder.String()
	if err != nil {
		return fmt.Errorf("wayland: registry.global: failed to decode interface: %w", err)
	}

	version, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: registry.global: failed to decode version: %w", err)
	}

	global := &Global{
		Name:      name,
		Interface: iface,
		Version:   version,
	}

	r.mu.Lock()
	r.globals[name] = global
	handler := r.onGlobal
	r.mu.Unlock()

	if handler != nil {
		handler(global)
	}

	return nil
}

// handleGlobalRemove handles the wl_registry.global_remove event.
func (r *Registry) handleGlobalRemove(msg *Message) error {
	decoder := NewDecoder(msg.Args)

	name, err := decoder.Uint32()
	if err != nil {
		return fmt.Errorf("wayland: registry.global_remove: failed to decode name: %w", err)
	}

	r.mu.Lock()
	delete(r.globals, name)
	handler := r.onGlobalRemove
	r.mu.Unlock()

	if handler != nil {
		handler(name)
	}

	return nil
}

// RequiredGlobals returns a list of interface names required for a typical application.
func RequiredGlobals() []string {
	return []string{
		InterfaceWlCompositor,
		InterfaceWlShm,
		InterfaceXdgWmBase,
	}
}

// CheckRequiredGlobals checks if all required globals are available.
// Returns a list of missing interface names, or nil if all are present.
func (r *Registry) CheckRequiredGlobals() []string {
	required := RequiredGlobals()
	var missing []string

	for _, iface := range required {
		if !r.HasGlobal(iface) {
			missing = append(missing, iface)
		}
	}

	return missing
}

// WaitForGlobals waits for all required globals to be advertised.
// It performs roundtrips until all required globals are available
// or the maximum number of attempts is reached.
func (r *Registry) WaitForGlobals(required []string, maxAttempts int) error {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Perform a roundtrip to receive pending events
		if err := r.display.Roundtrip(); err != nil {
			return fmt.Errorf("wayland: roundtrip failed: %w", err)
		}

		// Check if all required globals are present
		allPresent := true
		for _, iface := range required {
			if !r.HasGlobal(iface) {
				allPresent = false
				break
			}
		}

		if allPresent {
			return nil
		}
	}

	// Return error with missing globals
	var missing []string
	for _, iface := range required {
		if !r.HasGlobal(iface) {
			missing = append(missing, iface)
		}
	}

	return fmt.Errorf("wayland: missing required globals after %d attempts: %v",
		maxAttempts, missing)
}
