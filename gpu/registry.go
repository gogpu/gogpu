package gpu

import (
	"errors"
	"sync"
)

// ErrNoBackendRegistered is returned when no backend is available.
var ErrNoBackendRegistered = errors.New("gpu: no backend registered")

// BackendFactory creates a new backend instance.
type BackendFactory func() Backend

// registry holds registered backends.
var (
	registryMu sync.RWMutex
	backends   = make(map[string]BackendFactory)
	// Priority order for backend selection (first available wins)
	backendPriority = []string{"rust", "native"}
)

// RegisterBackend registers a backend factory with the given name.
// This is typically called from init() functions in backend packages.
// If a backend with the same name is already registered, it will be replaced.
func RegisterBackend(name string, factory BackendFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	backends[name] = factory
}

// UnregisterBackend removes a backend from the registry.
// Useful for testing.
func UnregisterBackend(name string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(backends, name)
}

// AvailableBackends returns a list of registered backend names.
func AvailableBackends() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	return names
}

// IsBackendRegistered checks if a backend with the given name is registered.
func IsBackendRegistered(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := backends[name]
	return ok
}

// CreateBackend creates a backend instance by name.
// Returns nil if the backend is not registered.
func CreateBackend(name string) Backend {
	registryMu.RLock()
	defer registryMu.RUnlock()

	factory, ok := backends[name]
	if !ok {
		return nil
	}
	return factory()
}

// SelectBestBackend returns the best available backend based on priority.
// Priority order: rust > native
// Returns nil if no backends are registered.
func SelectBestBackend() Backend {
	registryMu.RLock()
	defer registryMu.RUnlock()

	for _, name := range backendPriority {
		if factory, ok := backends[name]; ok {
			b := factory()
			if b != nil {
				return b
			}
		}
	}

	// Fallback: return first available
	for _, factory := range backends {
		if b := factory(); b != nil {
			return b
		}
	}

	return nil
}

// MustSelectBackend returns the best available backend or panics.
func MustSelectBackend() Backend {
	b := SelectBestBackend()
	if b == nil {
		panic("gpu: no backend available")
	}
	return b
}

// InitDefaultBackend initializes the default backend based on availability.
// This is called automatically when using gogpu.NewApp() without explicit backend selection.
func InitDefaultBackend() error {
	b := SelectBestBackend()
	if b == nil {
		return ErrNoBackendRegistered
	}

	if err := b.Init(); err != nil {
		return err
	}

	SetBackend(b)
	return nil
}
