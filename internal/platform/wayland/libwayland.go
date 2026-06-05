//go:build linux

package wayland

import (
	"fmt"
	"log/slog"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// wlMethodDestroy is the Wayland protocol method name for wl_proxy_destroy.
const wlMethodDestroy = "destroy"

// LibwaylandHandle holds C pointers from libwayland-client.so.0 for Vulkan surface creation.
// Vulkan's VK_KHR_wayland_surface requires real wl_display* and wl_surface* from the C library.
// Our pure Go Wayland speaks wire protocol directly and doesn't have C structs.
//
// This follows the same pattern as x11/platform.go's xlibHandle: load the shared library
// via goffi, call minimal functions to get C pointers, use them for Vulkan only.
//
// Unlike X11 (where Window IDs are server-side and shared across connections), Wayland
// surfaces are client-side proxies, so we must create the surface on the C connection too.
//
// Key design: Global names from the pure Go Wayland registry are used to bind
// compositor and xdg_wm_base on the C connection. The xdg-shell role (xdg_surface +
// xdg_toplevel) is set up with goffi callbacks for configure/ping events.
// Callbacks are safe here because wl_display_roundtrip is called from Go — the
// callback fires on the same Go-managed thread (G is already loaded).
type LibwaylandHandle struct {
	lib     unsafe.Pointer // libwayland-client.so.0
	display uintptr        // wl_display* from wl_display_connect
	surface uintptr        // wl_surface* for Vulkan

	// Intermediate objects (kept for cleanup)
	registry   uintptr // wl_registry* proxy
	compositor uintptr // wl_compositor* proxy

	// xdg-shell objects (required for Vulkan presentation)
	xdgWmBase   uintptr // xdg_wm_base* proxy
	xdgSurface  uintptr // xdg_surface* proxy
	xdgToplevel uintptr // xdg_toplevel* proxy

	// Decoration objects (optional, zxdg_decoration_manager_v1)
	decorManager  uintptr // zxdg_decoration_manager_v1* proxy
	toplevelDecor uintptr // zxdg_toplevel_decoration_v1* proxy

	// Function symbols
	fnDisplayConnect   unsafe.Pointer
	fnDisplayDisconn   unsafe.Pointer
	fnDisplayFlush     unsafe.Pointer
	fnProxyMarshal     unsafe.Pointer // wl_proxy_marshal_array_constructor
	fnProxyMarshalV    unsafe.Pointer // wl_proxy_marshal_array_constructor_versioned
	fnProxyDestroy     unsafe.Pointer
	fnAddListener      unsafe.Pointer // wl_proxy_add_listener
	fnRoundtrip        unsafe.Pointer // wl_display_roundtrip
	fnDispatchPend     unsafe.Pointer // wl_display_dispatch_pending
	fnPrepareRead      unsafe.Pointer // wl_display_prepare_read
	fnReadEvents       unsafe.Pointer // wl_display_read_events
	fnCancelRead       unsafe.Pointer // wl_display_cancel_read
	fnGetFD            unsafe.Pointer // wl_display_get_fd
	fnCreateQueue      unsafe.Pointer // wl_display_create_queue
	fnDispatchQueueP   unsafe.Pointer // wl_display_dispatch_queue_pending
	fnProxySetQueue    unsafe.Pointer // wl_proxy_set_queue
	fnMarshalArray     unsafe.Pointer // wl_proxy_marshal_array (no new_id)
	fnCreateWrapper    unsafe.Pointer // wl_proxy_create_wrapper
	fnWrapperDestroy   unsafe.Pointer // wl_proxy_wrapper_destroy
	fnPrepareReadQueue unsafe.Pointer // wl_display_prepare_read_queue
	fnDestroyQueue     unsafe.Pointer // wl_event_queue_destroy
	fnRoundtripQueue   unsafe.Pointer // wl_display_roundtrip_queue

	// displayMu serializes wl_display access between the main thread (event dispatch)
	// and the render thread (Vulkan WSI present/acquire). libwayland docs state that
	// wl_display is NOT thread-safe — concurrent wl_display_flush (from Vulkan
	// vkQueuePresentKHR) and wl_display_read_events (from DispatchDefaultQueue) can
	// corrupt internal state. ADR-041 Phase 2.
	displayMu sync.Mutex

	// Configure gate — blocks until compositor sends initial xdg_surface.configure.
	// Without this, Vulkan present can race ahead of the compositor mapping the surface,
	// causing SIGSEGV in vkQueuePresentKHR (ADR-041 Phase 1).
	initialConfigureReceived bool

	// App event queue — ALL our Wayland objects (registry, compositor, surface,
	// xdg, seat, pointer, keyboard, etc.) live on this queue. The default queue
	// is left exclusively for Mesa Vulkan WSI's internal wl_buffer.release
	// callbacks. This separation prevents our dispatch from firing Mesa's
	// callbacks outside Mesa's expected context (ADR-041 Phase 4).
	appQueue uintptr // wl_event_queue* for all our objects

	// CSD objects (subsurfaces for client-side decorations)
	subcompositor uintptr    // wl_subcompositor* proxy
	shm           uintptr    // wl_shm* proxy
	csdSurfaces   [4]uintptr // wl_surface* for top/left/right/bottom
	csdSubsurf    [4]uintptr // wl_subsurface* for top/left/right/bottom
	csdPools      [4]uintptr // wl_shm_pool* for each decoration
	csdBuffers    [4]uintptr // wl_buffer* for each decoration
	csdFDs        [4]int     // shm file descriptors
	csdData       [4][]byte  // mmap'd pixel data
	csdSizes      [4][2]int  // [width, height] for each decoration
	csdContentW   int        // content area width (for resize delta detection)
	csdContentH   int        // content area height (for resize delta detection)
	configuredW   int        // last configure width (for set_window_geometry in ack_configure)
	configuredH   int        // last configure height
	csdActive     bool

	// CSD input (pointer events on C display for decoration subsurfaces)
	csdQueue          uintptr      // separate event queue for CSD pointer events
	csdSeat           uintptr      // wl_seat* on CSD queue (for pointer events)
	csdSeatDefault    uintptr      // wl_seat* on default queue (for move/resize)
	csdPointer        uintptr      // wl_pointer* on C display
	csdHitResult      CSDHitResult // current hit-test result under pointer
	csdPointerX       float64      // pointer x in current subsurface
	csdPointerY       float64      // pointer y in current subsurface
	csdPointerSurface uintptr      // which C surface pointer is over (0 = none)
	csdSerial         uint32       // last button press serial (for move/resize)
	csdPainter        CSDPainter   // painter for repaint on hover
	csdState          CSDState     // current decoration state
	onCSDClose        func()       // callback when close button clicked
	csdPendingAction  CSDHitResult // action to perform outside callback
	csdPendingSerial  uint32       // serial for pending move/resize
	csdPendingRepaint bool         // title bar needs repaint (deferred from callback)
	csdPendingCursor  bool         // CSD cursor shape needs update (deferred from callback)
	csdPendingResize  bool         // CSD needs resize on next xdg_surface.configure
	csdPendingResizeW int          // pending CSD content width for resize
	csdPendingResizeH int          // pending CSD content height for resize

	// Main surface input (pointer, keyboard, touch on default queue)
	inputSeat      uintptr         // wl_seat* for main input
	inputSeatCaps  uint32          // seat capabilities bitmask
	inputPointer   uintptr         // wl_pointer* for main surface
	inputKeyboard  uintptr         // wl_keyboard* for main surface
	inputTouch     uintptr         // wl_touch* for main surface
	inputCallbacks *InputCallbacks // Go callbacks for input events

	// Pointer constraints (zwp_pointer_constraints_v1 + zwp_relative_pointer_v1)
	pointerConstraintsMgr uintptr // zwp_pointer_constraints_v1* manager proxy
	relativePointerMgr    uintptr // zwp_relative_pointer_manager_v1* proxy
	lockedPointer         uintptr // current zwp_locked_pointer_v1* (or 0)
	confinedPointer       uintptr // current zwp_confined_pointer_v1* (or 0)
	relativePointer       uintptr // current zwp_relative_pointer_v1* (or 0)
	pointerEnterSerial    uint32  // last wl_pointer.enter serial (needed for set_cursor)

	// Cursor shape (wp_cursor_shape_manager_v1)
	cursorShapeMgr       uintptr // wp_cursor_shape_manager_v1* manager proxy (or 0)
	cursorShapeDevice    uintptr // wp_cursor_shape_device_v1* for main pointer (or 0)
	csdCursorShapeDevice uintptr // wp_cursor_shape_device_v1* for CSD pointer (or 0)

	// Clipboard (wl_data_device_manager protocol)
	clipboardMgr          uintptr    // wl_data_device_manager* proxy (or 0)
	clipboardDevice       uintptr    // wl_data_device* proxy (or 0)
	clipboardSource       uintptr    // current wl_data_source* we own (or 0)
	clipboardOffer        uintptr    // current wl_data_offer* from selection event (or 0)
	clipboardPendingOffer uintptr    // offer introduced by data_offer event, not yet confirmed
	clipboardText         string     // locally stored clipboard text (what we copied)
	ownsClipboard         bool       // true if our source is the active clipboard
	clipboardOfferHasText bool       // true if current offer advertised text/plain mime
	clipboardMu           sync.Mutex // protects clipboard state

	// Data symbols (interface descriptors — pointers to static C structs)
	registryInterface      unsafe.Pointer // &wl_registry_interface
	compositorInterface    unsafe.Pointer // &wl_compositor_interface
	surfaceInterface       unsafe.Pointer // &wl_surface_interface
	subcompositorInterface unsafe.Pointer // &wl_subcompositor_interface
	subsurfaceInterface    unsafe.Pointer // &wl_subsurface_interface
	shmInterface           unsafe.Pointer // &wl_shm_interface
	shmPoolInterface       unsafe.Pointer // &wl_shm_pool_interface
	bufferInterface        unsafe.Pointer // &wl_buffer_interface
	seatInterface          unsafe.Pointer // &wl_seat_interface
	pointerInterface       unsafe.Pointer // &wl_pointer_interface
	keyboardInterface      unsafe.Pointer // &wl_keyboard_interface
	touchInterface         unsafe.Pointer // &wl_touch_interface

	// Call interfaces (goffi call descriptors, prepared once)
	cifConnect        types.CallInterface
	cifDisconn        types.CallInterface
	cifFlush          types.CallInterface
	cifMarshal        types.CallInterface
	cifMarshalV       types.CallInterface
	cifDestroy        types.CallInterface
	cifAddListener    types.CallInterface
	cifRoundtrip      types.CallInterface
	cifDispatchP      types.CallInterface // wl_display_dispatch_pending(display) -> int
	cifPrepareRead    types.CallInterface // wl_display_prepare_read(display) -> int
	cifReadEvents     types.CallInterface // wl_display_read_events(display) -> int
	cifCreateQueue    types.CallInterface // wl_display_create_queue(display) -> queue*
	cifDispatchQP     types.CallInterface // wl_display_dispatch_queue_pending(display, queue) -> int
	cifSetQueue       types.CallInterface // wl_proxy_set_queue(proxy, queue) -> void
	cifMarshalArr     types.CallInterface // wl_proxy_marshal_array(proxy, opcode, args) -> void
	cifCreateWrapper  types.CallInterface // wl_proxy_create_wrapper(proxy) -> proxy*
	cifWrapperDestroy types.CallInterface // wl_proxy_wrapper_destroy(wrapper) -> void
	cifPrepareReadQ   types.CallInterface // wl_display_prepare_read_queue(display, queue) -> int
	cifDestroyQueue   types.CallInterface // wl_event_queue_destroy(queue) -> void
	cifRoundtripQueue types.CallInterface // wl_display_roundtrip_queue(display, queue) -> int
}

// Display returns the wl_display* C pointer for Vulkan surface creation.
func (h *LibwaylandHandle) Display() uintptr { return h.display }

// Surface returns the wl_surface* C pointer for Vulkan surface creation.
func (h *LibwaylandHandle) Surface() uintptr { return h.surface }

// DisplayLock acquires the wl_display mutex. The render thread must hold this
// lock around Vulkan WSI operations (present, acquire) that internally call
// wl_surface_attach / wl_surface_commit / wl_display_flush. Without this lock,
// the main thread's DispatchDefaultQueue can race with Vulkan WSI calls,
// corrupting libwayland internal state (ADR-041 Phase 2).
func (h *LibwaylandHandle) DisplayLock() { h.displayMu.Lock() }

// DisplayUnlock releases the wl_display mutex.
func (h *LibwaylandHandle) DisplayUnlock() { h.displayMu.Unlock() }

// OpenLibwayland loads libwayland-client.so.0 and creates Vulkan-ready C pointers.
// compositorName/Version and xdgWmBaseName/Version come from the pure Go Wayland
// registry — global names are server-assigned and identical across all client connections.
// The xdg-shell role is required for Vulkan presentation (without it, the compositor
// won't composite the surface and vkQueuePresentKHR blocks forever).
func OpenLibwayland(compositorName, compositorVersion, xdgWmBaseName, xdgWmBaseVersion, decorName, decorVersion uint32) (*LibwaylandHandle, error) {
	h := &LibwaylandHandle{}

	// Step 1: Load library
	lib, err := ffi.LoadLibrary("libwayland-client.so.0")
	if err != nil {
		return nil, fmt.Errorf("wayland: failed to load libwayland-client.so.0: %w", err)
	}
	h.lib = lib

	// Step 2: Resolve function symbols
	if err := h.resolveSymbols(); err != nil {
		return nil, err
	}

	// Step 3: Prepare call interfaces
	if err := h.prepareCIFs(); err != nil {
		return nil, err
	}

	// Step 4: wl_display_connect(NULL) → wl_display*
	var displayArg uintptr // NULL = use WAYLAND_DISPLAY env
	var display uintptr
	connectArgs := [1]unsafe.Pointer{unsafe.Pointer(&displayArg)}
	_ = ffi.CallFunction(&h.cifConnect, h.fnDisplayConnect, unsafe.Pointer(&display), connectArgs[:])
	if display == 0 {
		return nil, fmt.Errorf("wayland: wl_display_connect(NULL) returned NULL")
	}
	h.display = display

	// Step 4b: Create app event queue — ALL our objects live here.
	// The default queue is left exclusively for Mesa Vulkan WSI's internal
	// wl_buffer.release callbacks. This prevents our dispatch from firing
	// Mesa's callbacks outside Mesa's expected context (GLFW/SDL3 pattern,
	// ADR-041 Phase 4).
	var appQueue uintptr
	queueArgs := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	ffi.CallFunction(&h.cifCreateQueue, h.fnCreateQueue, unsafe.Pointer(&appQueue), queueArgs[:])
	if appQueue == 0 {
		h.disconnectDisplay()
		return nil, fmt.Errorf("wayland: wl_display_create_queue returned NULL")
	}
	h.appQueue = appQueue

	// Step 4c: Create ephemeral display wrapper assigned to appQueue.
	// wl_proxy_create_wrapper creates a lightweight proxy that inherits
	// the wrapper's queue to all child objects created through it.
	// This is the GLFW pattern: wrapper → get_registry → registry on appQueue
	// → all registry.bind results on appQueue → all child objects on appQueue.
	var displayWrapper uintptr
	wrapperArgs := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	ffi.CallFunction(&h.cifCreateWrapper, h.fnCreateWrapper, unsafe.Pointer(&displayWrapper), wrapperArgs[:])
	if displayWrapper == 0 {
		h.destroyAppQueue()
		h.disconnectDisplay()
		return nil, fmt.Errorf("wayland: wl_proxy_create_wrapper returned NULL")
	}
	// Assign the wrapper to our app queue — children inherit this queue.
	setQArgs := [2]unsafe.Pointer{unsafe.Pointer(&displayWrapper), unsafe.Pointer(&appQueue)}
	ffi.CallFunction(&h.cifSetQueue, h.fnProxySetQueue, nil, setQArgs[:])

	// Step 5: Get registry THROUGH the wrapper so it inherits appQueue.
	// Opcode 1 = wl_display::get_registry. Arg: one new_id (NULL placeholder).
	registry, err := h.marshalConstructor(displayWrapper, 1, h.registryInterface)

	// Destroy the wrapper — child objects (registry) retain the queue assignment.
	destroyWrapperArgs := [1]unsafe.Pointer{unsafe.Pointer(&displayWrapper)}
	ffi.CallFunction(&h.cifWrapperDestroy, h.fnWrapperDestroy, nil, destroyWrapperArgs[:])

	if err != nil {
		h.destroyAppQueue()
		h.disconnectDisplay()
		return nil, fmt.Errorf("wayland: failed to get registry: %w", err)
	}
	h.registry = registry

	// Step 6: Bind to wl_compositor using the global name from pure Go registry.
	// No callbacks needed — we already know the compositor's name and version.
	version := compositorVersion
	if version > 4 {
		version = 4
	}
	compositor, err := h.registryBind(compositorName, h.compositorInterface, version)
	if err != nil {
		h.disconnectDisplay()
		return nil, fmt.Errorf("wayland: failed to bind compositor: %w", err)
	}
	h.compositor = compositor

	// Step 7: Create wl_surface — wl_proxy_marshal_array_constructor(compositor, 0, [NULL], &wl_surface_interface)
	// Opcode 0 = wl_compositor::create_surface
	surface, err := h.marshalConstructor(compositor, 0, h.surfaceInterface)
	if err != nil {
		h.disconnectDisplay()
		return nil, fmt.Errorf("wayland: failed to create surface: %w", err)
	}
	h.surface = surface

	// Step 8: Flush to ensure all requests reach the compositor.
	if err := h.flush(); err != nil {
		h.disconnectDisplay()
		return nil, fmt.Errorf("wayland: flush failed: %w", err)
	}

	// Step 9: Set up xdg-shell role (xdg_surface + xdg_toplevel).
	// Without a role, the compositor won't composite the surface,
	// buffer release events never arrive, and vkQueuePresentKHR blocks forever.
	if err := h.setupXdgRole(xdgWmBaseName, xdgWmBaseVersion, decorName, decorVersion); err != nil {
		h.disconnectDisplay()
		return nil, err
	}

	return h, nil
}

// Close disconnects from the Wayland display and frees C resources.
// Destroys objects in reverse creation order.
func (h *LibwaylandHandle) Close() {
	if h == nil {
		return
	}

	// Destroy clipboard objects (before cursor shape, pointer constraints, and input)
	h.DestroyClipboard()

	// Destroy cursor shape objects (before pointer constraints and input)
	h.DestroyCursorShapeDevices()
	if h.cursorShapeMgr != 0 {
		h.marshalVoid(h.cursorShapeMgr, 0) // wp_cursor_shape_manager_v1::destroy (opcode 0)
		h.proxyDestroy(h.cursorShapeMgr)
		h.cursorShapeMgr = 0
	}

	// Destroy pointer constraint objects (before input, in reverse creation order)
	h.DestroyRelativePointer()
	h.DestroyLockedPointer()
	h.DestroyConfinedPointer()
	if h.relativePointerMgr != 0 {
		h.proxyDestroy(h.relativePointerMgr)
		h.relativePointerMgr = 0
	}
	if h.pointerConstraintsMgr != 0 {
		h.proxyDestroy(h.pointerConstraintsMgr)
		h.pointerConstraintsMgr = 0
	}

	// Destroy input objects
	if h.inputTouch != 0 {
		h.proxyDestroy(h.inputTouch)
		h.inputTouch = 0
	}
	if h.inputKeyboard != 0 {
		h.proxyDestroy(h.inputKeyboard)
		h.inputKeyboard = 0
	}
	if h.inputPointer != 0 {
		h.proxyDestroy(h.inputPointer)
		h.inputPointer = 0
	}
	if h.inputSeat != 0 {
		h.proxyDestroy(h.inputSeat)
		h.inputSeat = 0
	}

	// Destroy decoration objects (reverse order: decoration → manager)
	if h.toplevelDecor != 0 {
		h.proxyDestroy(h.toplevelDecor)
		h.toplevelDecor = 0
	}
	if h.decorManager != 0 {
		h.proxyDestroy(h.decorManager)
		h.decorManager = 0
	}

	// Destroy xdg objects (reverse order: toplevel → surface → wm_base)
	if h.xdgToplevel != 0 {
		h.proxyDestroy(h.xdgToplevel)
		h.xdgToplevel = 0
	}
	if h.xdgSurface != 0 {
		h.proxyDestroy(h.xdgSurface)
		h.xdgSurface = 0
	}
	if h.xdgWmBase != 0 {
		h.proxyDestroy(h.xdgWmBase)
		h.xdgWmBase = 0
	}

	// Destroy wl_surface
	if h.surface != 0 {
		h.proxyDestroy(h.surface)
		h.surface = 0
	}

	// Destroy compositor
	if h.compositor != 0 {
		h.proxyDestroy(h.compositor)
		h.compositor = 0
	}

	// Destroy registry
	if h.registry != 0 {
		h.proxyDestroy(h.registry)
		h.registry = 0
	}

	// Flush all destroy requests and roundtrip to ensure compositor processes them.
	// Without this, WSLg leaves a ghost window on screen.
	_ = h.flush()
	h.Roundtrip()

	// Destroy app event queue after all objects are gone (ADR-041 Phase 4).
	h.destroyAppQueue()

	h.disconnectDisplay()
}

// resolveSymbols loads all required function and data symbols from libwayland-client.so.0.
func (h *LibwaylandHandle) resolveSymbols() error {
	syms := []struct {
		name string
		dst  *unsafe.Pointer
	}{
		{"wl_display_connect", &h.fnDisplayConnect},
		{"wl_display_disconnect", &h.fnDisplayDisconn},
		{"wl_display_flush", &h.fnDisplayFlush},
		{"wl_proxy_marshal_array_constructor", &h.fnProxyMarshal},
		{"wl_proxy_marshal_array_constructor_versioned", &h.fnProxyMarshalV},
		{"wl_proxy_destroy", &h.fnProxyDestroy},
		{"wl_proxy_add_listener", &h.fnAddListener},
		{"wl_display_roundtrip", &h.fnRoundtrip},
		{"wl_display_dispatch_pending", &h.fnDispatchPend},
		{"wl_display_prepare_read", &h.fnPrepareRead},
		{"wl_display_read_events", &h.fnReadEvents},
		{"wl_display_cancel_read", &h.fnCancelRead},
		{"wl_display_get_fd", &h.fnGetFD},
		{"wl_display_create_queue", &h.fnCreateQueue},
		{"wl_display_dispatch_queue_pending", &h.fnDispatchQueueP},
		{"wl_proxy_set_queue", &h.fnProxySetQueue},
		{"wl_proxy_marshal_array", &h.fnMarshalArray},
		{"wl_proxy_create_wrapper", &h.fnCreateWrapper},
		{"wl_proxy_wrapper_destroy", &h.fnWrapperDestroy},
		{"wl_display_prepare_read_queue", &h.fnPrepareReadQueue},
		{"wl_event_queue_destroy", &h.fnDestroyQueue},
		{"wl_display_roundtrip_queue", &h.fnRoundtripQueue},
	}

	for _, s := range syms {
		sym, err := ffi.GetSymbol(h.lib, s.name)
		if err != nil {
			return fmt.Errorf("wayland: symbol %s not found: %w", s.name, err)
		}
		if sym == nil {
			return fmt.Errorf("wayland: symbol %s is nil", s.name)
		}
		*s.dst = sym
	}

	// Data symbols — these are pointers TO the interface struct (we need the address)
	datasyms := []struct {
		name string
		dst  *unsafe.Pointer
	}{
		{"wl_registry_interface", &h.registryInterface},
		{"wl_compositor_interface", &h.compositorInterface},
		{"wl_surface_interface", &h.surfaceInterface},
		{"wl_subcompositor_interface", &h.subcompositorInterface},
		{"wl_subsurface_interface", &h.subsurfaceInterface},
		{"wl_shm_interface", &h.shmInterface},
		{"wl_shm_pool_interface", &h.shmPoolInterface},
		{"wl_buffer_interface", &h.bufferInterface},
		{"wl_seat_interface", &h.seatInterface},
		{"wl_pointer_interface", &h.pointerInterface},
		{"wl_keyboard_interface", &h.keyboardInterface},
		{"wl_touch_interface", &h.touchInterface},
	}

	for _, s := range datasyms {
		sym, err := ffi.GetSymbol(h.lib, s.name)
		if err != nil {
			return fmt.Errorf("wayland: symbol %s not found: %w", s.name, err)
		}
		if sym == nil {
			return fmt.Errorf("wayland: symbol %s is nil", s.name)
		}
		*s.dst = sym
	}

	return nil
}

// prepareCIFs prepares goffi call interfaces for each function signature.
func (h *LibwaylandHandle) prepareCIFs() error {
	ptr := types.PointerTypeDescriptor
	i32 := types.SInt32TypeDescriptor

	cifDefs := []struct {
		name string
		cif  *types.CallInterface
		ret  *types.TypeDescriptor
		args []*types.TypeDescriptor
	}{
		// wl_display* wl_display_connect(const char* name)
		{"connect", &h.cifConnect, ptr, []*types.TypeDescriptor{ptr}},
		// void wl_display_disconnect(wl_display*)
		{"disconnect", &h.cifDisconn, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr}},
		// int wl_display_flush(wl_display*)
		{"flush", &h.cifFlush, i32, []*types.TypeDescriptor{ptr}},
		// wl_proxy* wl_proxy_marshal_array_constructor(wl_proxy*, uint32_t opcode, union wl_argument*, const wl_interface*)
		{"marshal", &h.cifMarshal, ptr, []*types.TypeDescriptor{ptr, types.UInt32TypeDescriptor, ptr, ptr}},
		// wl_proxy* wl_proxy_marshal_array_constructor_versioned(wl_proxy*, uint32_t opcode, union wl_argument*, const wl_interface*, uint32_t version)
		{"marshal_v", &h.cifMarshalV, ptr, []*types.TypeDescriptor{ptr, types.UInt32TypeDescriptor, ptr, ptr, types.UInt32TypeDescriptor}},
		// void wl_proxy_destroy(wl_proxy*)
		{wlMethodDestroy, &h.cifDestroy, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr}},
		// int wl_proxy_add_listener(wl_proxy*, void(**)(void), void* data)
		{"add_listener", &h.cifAddListener, i32, []*types.TypeDescriptor{ptr, ptr, ptr}},
		// int wl_display_roundtrip(wl_display*)
		{"roundtrip", &h.cifRoundtrip, i32, []*types.TypeDescriptor{ptr}},
		// int wl_display_dispatch_pending(wl_display*)
		{"dispatch_pending", &h.cifDispatchP, i32, []*types.TypeDescriptor{ptr}},
		// int wl_display_prepare_read(wl_display*)
		{"prepare_read", &h.cifPrepareRead, i32, []*types.TypeDescriptor{ptr}},
		// int wl_display_read_events(wl_display*)
		{"read_events", &h.cifReadEvents, i32, []*types.TypeDescriptor{ptr}},
		// wl_event_queue* wl_display_create_queue(wl_display*)
		{"create_queue", &h.cifCreateQueue, ptr, []*types.TypeDescriptor{ptr}},
		// int wl_display_dispatch_queue_pending(wl_display*, wl_event_queue*)
		{"dispatch_queue_pending", &h.cifDispatchQP, i32, []*types.TypeDescriptor{ptr, ptr}},
		// void wl_proxy_set_queue(wl_proxy*, wl_event_queue*)
		{"set_queue", &h.cifSetQueue, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr, ptr}},
		// void wl_proxy_marshal_array(wl_proxy*, uint32_t opcode, union wl_argument*)
		{"marshal_array", &h.cifMarshalArr, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr, types.UInt32TypeDescriptor, ptr}},
		// wl_proxy* wl_proxy_create_wrapper(void* proxy)
		{"create_wrapper", &h.cifCreateWrapper, ptr, []*types.TypeDescriptor{ptr}},
		// void wl_proxy_wrapper_destroy(void* wrapper)
		{"wrapper_destroy", &h.cifWrapperDestroy, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr}},
		// int wl_display_prepare_read_queue(wl_display*, wl_event_queue*)
		{"prepare_read_queue", &h.cifPrepareReadQ, i32, []*types.TypeDescriptor{ptr, ptr}},
		// void wl_event_queue_destroy(wl_event_queue*)
		{"destroy_queue", &h.cifDestroyQueue, types.VoidTypeDescriptor, []*types.TypeDescriptor{ptr}},
		// int wl_display_roundtrip_queue(wl_display*, wl_event_queue*)
		{"roundtrip_queue", &h.cifRoundtripQueue, i32, []*types.TypeDescriptor{ptr, ptr}},
	}

	for _, d := range cifDefs {
		if err := ffi.PrepareCallInterface(d.cif, types.DefaultCall, d.ret, d.args); err != nil {
			return fmt.Errorf("wayland: failed to prepare CIF %s: %w", d.name, err)
		}
	}

	return nil
}

// marshalConstructor calls wl_proxy_marshal_array_constructor for typed new_id requests.
// Used for get_registry (display opcode 1) and create_surface (compositor opcode 0).
// The single argument is a NULL placeholder for the new object ID (filled by the constructor).
func (h *LibwaylandHandle) marshalConstructor(proxy uintptr, opcode uint32, iface unsafe.Pointer) (uintptr, error) {
	// wl_argument array: one entry = NULL (new_id placeholder)
	var argBuf [1]uintptr // wl_argument is union of 8 bytes; uintptr on 64-bit
	argPtr := uintptr(unsafe.Pointer(&argBuf[0]))
	ifaceAddr := uintptr(iface)

	var result uintptr
	args := [4]unsafe.Pointer{
		unsafe.Pointer(&proxy),
		unsafe.Pointer(&opcode),
		unsafe.Pointer(&argPtr),
		unsafe.Pointer(&ifaceAddr),
	}
	_ = ffi.CallFunction(&h.cifMarshal, h.fnProxyMarshal, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("wl_proxy_marshal_array_constructor returned NULL (opcode %d)", opcode)
	}
	return result, nil
}

// registryBind calls wl_proxy_marshal_array_constructor_versioned for untyped new_id (registry.bind).
// Wire format "usun": name(uint), interface(string), version(uint), new_id.
func (h *LibwaylandHandle) registryBind(name uint32, iface unsafe.Pointer, version uint32) (uintptr, error) {
	// Build wl_argument array for registry::bind (opcode 0).
	// Wire format "usun" = 4 arguments:
	//   [0] uint32: global name
	//   [1] string: interface name (C string pointer, read from wl_interface.name — first field)
	//   [2] uint32: version
	//   [3] new_id: NULL placeholder (filled by constructor)
	//
	// wl_interface struct layout: { const char *name; int version; ... }
	// So *(const char**)iface gives us the interface name C string.
	ifaceNamePtr := *(*uintptr)(iface)

	var argBuf [4]uintptr
	argBuf[0] = uintptr(name)
	argBuf[1] = ifaceNamePtr // interface name C string (required by libwayland validation)
	argBuf[2] = uintptr(version)
	argBuf[3] = 0 // new_id placeholder

	opcode := uint32(0) // wl_registry::bind
	argPtr := uintptr(unsafe.Pointer(&argBuf[0]))
	ifaceAddr := uintptr(iface)

	var result uintptr
	args := [5]unsafe.Pointer{
		unsafe.Pointer(&h.registry),
		unsafe.Pointer(&opcode),
		unsafe.Pointer(&argPtr),
		unsafe.Pointer(&ifaceAddr),
		unsafe.Pointer(&version),
	}
	_ = ffi.CallFunction(&h.cifMarshalV, h.fnProxyMarshalV, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("registry bind failed for global %d version %d", name, version)
	}
	return result, nil
}

// flush calls wl_display_flush(display) to send all buffered requests to the compositor.
// Unlike wl_display_roundtrip, this does NOT read responses or trigger callbacks.
func (h *LibwaylandHandle) flush() error {
	var result int32
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	_ = ffi.CallFunction(&h.cifFlush, h.fnDisplayFlush, unsafe.Pointer(&result), args[:])
	if result < 0 {
		return fmt.Errorf("wl_display_flush failed: %d", result)
	}
	return nil
}

// proxyDestroy calls wl_proxy_destroy on a proxy.
func (h *LibwaylandHandle) proxyDestroy(proxy uintptr) {
	if proxy == 0 || h.fnProxyDestroy == nil {
		return
	}
	args := [1]unsafe.Pointer{unsafe.Pointer(&proxy)}
	_ = ffi.CallFunction(&h.cifDestroy, h.fnProxyDestroy, nil, args[:])
}

// destroyAppQueue destroys the app event queue.
func (h *LibwaylandHandle) destroyAppQueue() {
	if h.appQueue == 0 || h.fnDestroyQueue == nil {
		return
	}
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.appQueue)}
	ffi.CallFunction(&h.cifDestroyQueue, h.fnDestroyQueue, nil, args[:])
	h.appQueue = 0
}

// disconnectDisplay calls wl_display_disconnect.
func (h *LibwaylandHandle) disconnectDisplay() {
	if h.display == 0 || h.fnDisplayDisconn == nil {
		return
	}
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.display)}
	_ = ffi.CallFunction(&h.cifDisconn, h.fnDisplayDisconn, nil, args[:])
	h.display = 0
}

// WaitForConfigure performs blocking roundtrips until the compositor sends
// the initial xdg_surface.configure event. Without this gate, Vulkan
// presentation races ahead of surface mapping and crashes with SIGSEGV
// in vkQueuePresentKHR (ADR-041).
//
// The xdg-shell spec requires the compositor to send configure after the
// initial commit, but a single roundtrip is not guaranteed to deliver it
// (compositor may batch events or delay). This loop is bounded: the
// compositor MUST send configure eventually or the connection is broken.
func (h *LibwaylandHandle) WaitForConfigure() error {
	const maxRoundtrips = 50 // safety limit — compositor should respond in 1-3
	for i := range maxRoundtrips {
		if h.initialConfigureReceived {
			slog.Debug("wayland: initial configure received", "roundtrips", i)
			return nil
		}
		if err := h.roundtrip(); err != nil {
			return fmt.Errorf("wayland: wait for configure: %w", err)
		}
	}
	// If we exhausted retries, the compositor never sent configure.
	// This should not happen with a conformant compositor, but proceeding
	// without configure is better than hanging indefinitely.
	slog.Warn("wayland: configure not received after max roundtrips, proceeding anyway",
		"maxRoundtrips", maxRoundtrips)
	return nil
}

// InitialConfigureReceived reports whether the compositor has sent the
// initial xdg_surface.configure event. Used by the platform layer to
// confirm that the surface is ready for Vulkan presentation.
func (h *LibwaylandHandle) InitialConfigureReceived() bool {
	return h.initialConfigureReceived
}
