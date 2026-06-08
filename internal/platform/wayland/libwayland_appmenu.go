//go:build linux

package wayland

// libwayland_appmenu.go — org_kde_kwin_appmenu protocol.
//
// KDE Plasma Wayland requires clients to call set_address on the
// org_kde_kwin_appmenu object (bound from org_kde_kwin_appmenu_manager) so
// that KWin can associate the D-Bus dbusmenu service with a specific
// wl_surface.  Without this, global menus are not shown on Wayland even if
// RegisterWindow is successfully called on com.canonical.AppMenu.Registrar.
//
// Protocol objects:
//   - org_kde_kwin_appmenu_manager: global manager (name in Wayland registry)
//   - org_kde_kwin_appmenu: per-surface object; set_address sets the D-Bus
//     service name and object path
//
// Uses the same C-compatible interface descriptor pattern as all other
// protocol files in this package.

import (
	"runtime"
	"sync"
	"unsafe"
)

// kdeAppmenuIfaces holds C-compatible interface descriptors for
// org_kde_kwin_appmenu_manager and org_kde_kwin_appmenu.
// Constructed once, live for program lifetime.
var kdeAppmenuIfaces struct {
	once sync.Once

	manager cWlInterface // org_kde_kwin_appmenu_manager
	appmenu cWlInterface // org_kde_kwin_appmenu

	// Method arrays (indexed by opcode)
	managerMethods [1]cWlMessage // get_appmenu(new_id, surface)
	appmenuMethods [2]cWlMessage // set_address(ss), release

	nullTypes [4]uintptr
}

func initKDEAppmenuInterfaces() {
	kdeAppmenuIfaces.once.Do(func() {
		nt := uintptr(unsafe.Pointer(&kdeAppmenuIfaces.nullTypes[0]))

		// org_kde_kwin_appmenu_manager methods
		// opcode 0: get_appmenu(id: new_id<org_kde_kwin_appmenu>, surface: object<wl_surface>)
		kdeAppmenuIfaces.managerMethods[0] = cWlMessage{cstr("get_appmenu\x00"), cstr("no\x00"), nt}

		kdeAppmenuIfaces.manager = cWlInterface{
			Name:        cstr("org_kde_kwin_appmenu_manager\x00"),
			Version:     1,
			MethodCount: 1,
			Methods:     uintptr(unsafe.Pointer(&kdeAppmenuIfaces.managerMethods[0])),
			EventCount:  0,
			Events:      0,
		}

		// org_kde_kwin_appmenu methods
		// opcode 0: set_address(service_name: string, object_path: string)
		kdeAppmenuIfaces.appmenuMethods[0] = cWlMessage{cstr("set_address\x00"), cstr("ss\x00"), nt}
		// opcode 1: release (destructor)
		kdeAppmenuIfaces.appmenuMethods[1] = cWlMessage{cstr("release\x00"), cstr("\x00"), nt}

		kdeAppmenuIfaces.appmenu = cWlInterface{
			Name:        cstr("org_kde_kwin_appmenu\x00"),
			Version:     1,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&kdeAppmenuIfaces.appmenuMethods[0])),
			EventCount:  0,
			Events:      0,
		}
	})
}

// SetupKDEAppmenu binds org_kde_kwin_appmenu_manager from the registry and
// calls get_appmenu(surface) to create the per-surface appmenu object.
// Must be called after the surface is created.
// No-op if the compositor does not advertise this global.
func (h *LibwaylandHandle) SetupKDEAppmenu(name, version uint32) error {
	initKDEAppmenuInterfaces()

	v := version
	if v > 1 {
		v = 1
	}

	mgr, err := h.registryBind(name, unsafe.Pointer(&kdeAppmenuIfaces.manager), v)
	if err != nil {
		return err
	}
	h.kdeAppmenuMgr = mgr

	// get_appmenu: opcode 0, signature "no" (new_id + wl_surface)
	appmenu, err := h.marshalConstructorObj(
		mgr, 0,
		unsafe.Pointer(&kdeAppmenuIfaces.appmenu),
		h.surface,
	)
	if err != nil {
		h.proxyDestroy(mgr)
		h.kdeAppmenuMgr = 0
		return err
	}
	h.kdeAppmenu = appmenu
	return nil
}

// HasKDEAppmenu reports whether org_kde_kwin_appmenu was successfully created.
func (h *LibwaylandHandle) HasKDEAppmenu() bool {
	return h.kdeAppmenu != 0
}

// SetKDEAppmenuAddress sends set_address(serviceName, objectPath) on the
// org_kde_kwin_appmenu object so KWin can find our dbusmenu service.
// serviceName is our D-Bus unique bus name (e.g. ":1.42").
// objectPath is the dbusmenu object path (e.g. "/com/gogpu/menu/window/1234").
func (h *LibwaylandHandle) SetKDEAppmenuAddress(serviceName, objectPath string) {
	if h.kdeAppmenu == 0 {
		return
	}
	// Null-terminate both strings as C strings on the stack.
	// runtime.KeepAlive prevents the GC from collecting them before the FFI call.
	sBuf := make([]byte, len(serviceName)+1)
	copy(sBuf, serviceName)
	oBuf := make([]byte, len(objectPath)+1)
	copy(oBuf, objectPath)

	// set_address: opcode 0, signature "ss"
	h.marshalVoid(h.kdeAppmenu, 0,
		uintptr(unsafe.Pointer(&sBuf[0])),
		uintptr(unsafe.Pointer(&oBuf[0])),
	)
	runtime.KeepAlive(sBuf)
	runtime.KeepAlive(oBuf)
}

// DestroyKDEAppmenu releases org_kde_kwin_appmenu and its manager.
// Called from Close().
func (h *LibwaylandHandle) DestroyKDEAppmenu() {
	if h.kdeAppmenu != 0 {
		h.marshalVoid(h.kdeAppmenu, 1) // release: opcode 1
		h.proxyDestroy(h.kdeAppmenu)
		h.kdeAppmenu = 0
	}
	if h.kdeAppmenuMgr != 0 {
		h.proxyDestroy(h.kdeAppmenuMgr)
		h.kdeAppmenuMgr = 0
	}
}
