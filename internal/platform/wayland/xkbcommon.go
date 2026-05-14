//go:build linux

package wayland

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"golang.org/x/sys/unix"
)

// XKBHandle holds libxkbcommon state for keyboard layout handling.
// Loaded via dlopen from libxkbcommon.so.0 — available on every Wayland desktop.
// Non-fatal: if loading fails, the platform falls back to evdevKeycodeToRune (English only).
type XKBHandle struct {
	lib unsafe.Pointer // dlopen("libxkbcommon.so.0")

	// xkb objects (opaque pointers)
	context uintptr // xkb_context*
	keymap  uintptr // xkb_keymap*
	state   uintptr // xkb_state*

	// Function pointers
	fnContextNew          unsafe.Pointer
	fnContextUnref        unsafe.Pointer
	fnKeymapNewFromString unsafe.Pointer
	fnKeymapUnref         unsafe.Pointer
	fnStateNew            unsafe.Pointer
	fnStateUnref          unsafe.Pointer
	fnStateKeyGetUtf8     unsafe.Pointer
	fnStateUpdateMask     unsafe.Pointer

	// Call interfaces (prepared once)
	cifContextNew          types.CallInterface
	cifContextUnref        types.CallInterface
	cifKeymapNewFromString types.CallInterface
	cifKeymapUnref         types.CallInterface
	cifStateNew            types.CallInterface
	cifStateUnref          types.CallInterface
	cifStateKeyGetUtf8     types.CallInterface
	cifStateUpdateMask     types.CallInterface
}

// xkbcommon constants.
const (
	// XKB_CONTEXT_NO_FLAGS = 0
	xkbContextNoFlags uint32 = 0

	// XKB_KEYMAP_FORMAT_TEXT_V1 = 1
	xkbKeymapFormatTextV1 uint32 = 1

	// XKB_KEYMAP_COMPILE_NO_FLAGS = 0
	xkbKeymapCompileNoFlags uint32 = 0

	// Evdev keycode offset: Wayland sends evdev keycodes, xkbcommon expects evdev+8.
	xkbEvdevOffset uint32 = 8
)

// LoadXKBCommon loads libxkbcommon.so.0 and creates an xkb_context.
// Returns (nil, error) if the library is not available — caller should fall back gracefully.
func LoadXKBCommon() (*XKBHandle, error) {
	h := &XKBHandle{}

	lib, err := ffi.LoadLibrary("libxkbcommon.so.0")
	if err != nil {
		return nil, fmt.Errorf("xkbcommon: failed to load libxkbcommon.so.0: %w", err)
	}
	h.lib = lib

	if err := h.resolveSymbols(); err != nil {
		return nil, err
	}

	if err := h.prepareCIFs(); err != nil {
		return nil, err
	}

	// Create xkb_context
	ctx, err := h.contextNew()
	if err != nil {
		return nil, err
	}
	h.context = ctx

	slog.Debug("xkbcommon loaded successfully")
	return h, nil
}

// SetKeymapFromFD reads a keymap from a file descriptor (sent by compositor via wl_keyboard.keymap).
// The fd contains XKB keymap text (format XKB_KEYMAP_FORMAT_TEXT_V1).
// After this call, KeyGetUtf8 and UpdateMask are ready to use.
func (h *XKBHandle) SetKeymapFromFD(fd int, size uint32) error {
	if h == nil {
		return fmt.Errorf("xkbcommon: handle is nil")
	}

	// mmap the fd to read the keymap text
	data, err := unix.Mmap(fd, 0, int(size), unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return fmt.Errorf("xkbcommon: mmap keymap fd: %w", err)
	}
	defer func() {
		_ = unix.Munmap(data)
	}()

	// Find null terminator (xkb_keymap_new_from_string expects C string)
	keymapStr := data
	for i, b := range keymapStr {
		if b == 0 {
			keymapStr = keymapStr[:i]
			break
		}
	}

	// Destroy old keymap/state if present
	h.destroyState()
	h.destroyKeymap()

	// xkb_keymap_new_from_string(context, string, format, flags)
	keymap, err := h.keymapNewFromString(keymapStr)
	if err != nil {
		return err
	}
	h.keymap = keymap

	// xkb_state_new(keymap)
	state, err := h.stateNew(keymap)
	if err != nil {
		h.destroyKeymap()
		return err
	}
	h.state = state

	slog.Debug("xkbcommon: keymap loaded from compositor fd", "size", size)
	return nil
}

// KeyGetUtf8 converts a key press to UTF-8 text using the current xkb state.
// keycode is the evdev keycode (NOT offset — offset is applied internally).
// Returns empty string for non-printable keys.
func (h *XKBHandle) KeyGetUtf8(keycode uint32) string {
	if h == nil || h.state == 0 {
		return ""
	}

	// xkbcommon expects evdev keycode + 8
	xkbKey := keycode + xkbEvdevOffset

	// First call with NULL buffer to get required size
	size := h.stateKeyGetUtf8(xkbKey, nil, 0)
	if size <= 0 {
		return ""
	}

	// Allocate buffer (size includes null terminator space needed)
	buf := make([]byte, size+1)
	written := h.stateKeyGetUtf8(xkbKey, buf, uint32(len(buf)))
	if written <= 0 {
		return ""
	}

	return string(buf[:written])
}

// UpdateMask updates the xkb state with modifier/group changes from the compositor.
// Called from wl_keyboard.modifiers callback.
func (h *XKBHandle) UpdateMask(modsDepressed, modsLatched, modsLocked, group uint32) {
	if h == nil || h.state == 0 {
		return
	}
	h.stateUpdateMask(modsDepressed, modsLatched, modsLocked, 0, 0, group)
}

// Ready returns true if xkbcommon is loaded and a keymap is active.
func (h *XKBHandle) Ready() bool {
	return h != nil && h.state != 0
}

// Close destroys all xkb objects and unloads the library.
func (h *XKBHandle) Close() {
	if h == nil {
		return
	}
	h.destroyState()
	h.destroyKeymap()
	h.destroyContext()
	// Note: we don't unload the library (ffi doesn't expose dlclose),
	// it stays loaded for the process lifetime.
}

// --- Internal methods ---

func (h *XKBHandle) resolveSymbols() error {
	syms := []struct {
		name string
		dst  *unsafe.Pointer
	}{
		{"xkb_context_new", &h.fnContextNew},
		{"xkb_context_unref", &h.fnContextUnref},
		{"xkb_keymap_new_from_string", &h.fnKeymapNewFromString},
		{"xkb_keymap_unref", &h.fnKeymapUnref},
		{"xkb_state_new", &h.fnStateNew},
		{"xkb_state_unref", &h.fnStateUnref},
		{"xkb_state_key_get_utf8", &h.fnStateKeyGetUtf8},
		{"xkb_state_update_mask", &h.fnStateUpdateMask},
	}

	for _, s := range syms {
		sym, err := ffi.GetSymbol(h.lib, s.name)
		if err != nil {
			return fmt.Errorf("xkbcommon: symbol %s not found: %w", s.name, err)
		}
		if sym == nil {
			return fmt.Errorf("xkbcommon: symbol %s is nil", s.name)
		}
		*s.dst = sym
	}

	return nil
}

func (h *XKBHandle) prepareCIFs() error {
	ptr := types.PointerTypeDescriptor
	u32 := types.UInt32TypeDescriptor
	i32 := types.SInt32TypeDescriptor
	void := types.VoidTypeDescriptor

	cifDefs := []struct {
		name string
		cif  *types.CallInterface
		ret  *types.TypeDescriptor
		args []*types.TypeDescriptor
	}{
		// struct xkb_context* xkb_context_new(enum xkb_context_flags flags)
		{"context_new", &h.cifContextNew, ptr, []*types.TypeDescriptor{u32}},
		// void xkb_context_unref(struct xkb_context *context)
		{"context_unref", &h.cifContextUnref, void, []*types.TypeDescriptor{ptr}},
		// struct xkb_keymap* xkb_keymap_new_from_string(context, string, format, flags)
		{"keymap_new_from_string", &h.cifKeymapNewFromString, ptr, []*types.TypeDescriptor{ptr, ptr, u32, u32}},
		// void xkb_keymap_unref(struct xkb_keymap *keymap)
		{"keymap_unref", &h.cifKeymapUnref, void, []*types.TypeDescriptor{ptr}},
		// struct xkb_state* xkb_state_new(struct xkb_keymap *keymap)
		{"state_new", &h.cifStateNew, ptr, []*types.TypeDescriptor{ptr}},
		// void xkb_state_unref(struct xkb_state *state)
		{"state_unref", &h.cifStateUnref, void, []*types.TypeDescriptor{ptr}},
		// int xkb_state_key_get_utf8(state, key, buffer, size)
		{"state_key_get_utf8", &h.cifStateKeyGetUtf8, i32, []*types.TypeDescriptor{ptr, u32, ptr, u32}},
		// enum xkb_state_component xkb_state_update_mask(state, mods_depressed, mods_latched, mods_locked, layout_depressed, layout_latched, layout_locked)
		{"state_update_mask", &h.cifStateUpdateMask, u32, []*types.TypeDescriptor{ptr, u32, u32, u32, u32, u32, u32}},
	}

	for _, d := range cifDefs {
		if err := ffi.PrepareCallInterface(d.cif, types.DefaultCall, d.ret, d.args); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF %s: %w", d.name, err)
		}
	}

	return nil
}

// contextNew calls xkb_context_new(XKB_CONTEXT_NO_FLAGS).
func (h *XKBHandle) contextNew() (uintptr, error) {
	flags := xkbContextNoFlags
	var result uintptr
	args := [1]unsafe.Pointer{unsafe.Pointer(&flags)}
	_ = ffi.CallFunction(&h.cifContextNew, h.fnContextNew, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("xkbcommon: xkb_context_new returned NULL")
	}
	return result, nil
}

// keymapNewFromString calls xkb_keymap_new_from_string.
func (h *XKBHandle) keymapNewFromString(keymapStr []byte) (uintptr, error) {
	// Ensure null terminator (allocate new slice to avoid mutating input)
	cstr := make([]byte, len(keymapStr)+1)
	copy(cstr, keymapStr)
	strPtr := uintptr(unsafe.Pointer(&cstr[0]))
	format := xkbKeymapFormatTextV1
	flags := xkbKeymapCompileNoFlags

	var result uintptr
	args := [4]unsafe.Pointer{
		unsafe.Pointer(&h.context),
		unsafe.Pointer(&strPtr),
		unsafe.Pointer(&format),
		unsafe.Pointer(&flags),
	}
	_ = ffi.CallFunction(&h.cifKeymapNewFromString, h.fnKeymapNewFromString, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("xkbcommon: xkb_keymap_new_from_string returned NULL (invalid keymap?)")
	}
	return result, nil
}

// stateNew calls xkb_state_new(keymap).
func (h *XKBHandle) stateNew(keymap uintptr) (uintptr, error) {
	var result uintptr
	args := [1]unsafe.Pointer{unsafe.Pointer(&keymap)}
	_ = ffi.CallFunction(&h.cifStateNew, h.fnStateNew, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("xkbcommon: xkb_state_new returned NULL")
	}
	return result, nil
}

// stateKeyGetUtf8 calls xkb_state_key_get_utf8(state, key, buffer, size) -> int.
func (h *XKBHandle) stateKeyGetUtf8(key uint32, buf []byte, bufSize uint32) int32 {
	var bufPtr uintptr
	if len(buf) > 0 {
		bufPtr = uintptr(unsafe.Pointer(&buf[0]))
	}

	var result int32
	args := [4]unsafe.Pointer{
		unsafe.Pointer(&h.state),
		unsafe.Pointer(&key),
		unsafe.Pointer(&bufPtr),
		unsafe.Pointer(&bufSize),
	}
	_ = ffi.CallFunction(&h.cifStateKeyGetUtf8, h.fnStateKeyGetUtf8, unsafe.Pointer(&result), args[:])
	return result
}

// stateUpdateMask calls xkb_state_update_mask.
func (h *XKBHandle) stateUpdateMask(modsDepressed, modsLatched, modsLocked, layoutDepressed, layoutLatched, layoutLocked uint32) {
	var result uint32
	args := [7]unsafe.Pointer{
		unsafe.Pointer(&h.state),
		unsafe.Pointer(&modsDepressed),
		unsafe.Pointer(&modsLatched),
		unsafe.Pointer(&modsLocked),
		unsafe.Pointer(&layoutDepressed),
		unsafe.Pointer(&layoutLatched),
		unsafe.Pointer(&layoutLocked),
	}
	_ = ffi.CallFunction(&h.cifStateUpdateMask, h.fnStateUpdateMask, unsafe.Pointer(&result), args[:])
}

func (h *XKBHandle) destroyState() {
	if h.state != 0 {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.state)}
		ffi.CallFunction(&h.cifStateUnref, h.fnStateUnref, nil, args[:])
		h.state = 0
	}
}

func (h *XKBHandle) destroyKeymap() {
	if h.keymap != 0 {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.keymap)}
		ffi.CallFunction(&h.cifKeymapUnref, h.fnKeymapUnref, nil, args[:])
		h.keymap = 0
	}
}

func (h *XKBHandle) destroyContext() {
	if h.context != 0 {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.context)}
		ffi.CallFunction(&h.cifContextUnref, h.fnContextUnref, nil, args[:])
		h.context = 0
	}
}
