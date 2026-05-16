//go:build linux

// Package xkb provides a shared libxkbcommon wrapper for keyboard layout handling.
// Used by both X11 and Wayland platforms. Loaded via dlopen from libxkbcommon.so.0.
//
// On Wayland, the compositor sends keymap data via file descriptor (SetKeymapFromFD).
// On X11, the system default keymap is loaded via xkb_keymap_new_from_names (SetKeymapFromNames).
//
// Both paths produce the same xkb_state object used for text input via KeyGetUtf8.
// This eliminates the need for manual keysym-to-rune tables and handles AltGr/Level3
// correctly on all keyboard layouts.
package xkb

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"golang.org/x/sys/unix"
)

// ModsIndices caches XKB modifier indices resolved by name at keymap creation.
// This avoids hardcoding modifier bit positions (which vary across keymaps).
// Follows the winit pattern: resolve all 8 standard modifiers by name.
type ModsIndices struct {
	Shift    int32 // "Shift"   -- XKB_MOD_NAME_SHIFT
	CapsLock int32 // "Lock"    -- XKB_MOD_NAME_CAPS
	Control  int32 // "Control" -- XKB_MOD_NAME_CTRL
	Alt      int32 // "Mod1"    -- XKB_MOD_NAME_ALT
	NumLock  int32 // "Mod2"    -- XKB_MOD_NAME_NUM
	Mod3     int32 // "Mod3"
	Super    int32 // "Mod4"    -- XKB_MOD_NAME_LOGO
	Mod5     int32 // "Mod5"    -- usually AltGr/ISO_Level3_Shift
}

// Handle holds libxkbcommon state for keyboard layout handling.
// Loaded via dlopen from libxkbcommon.so.0 -- available on every modern Linux desktop.
// Non-fatal: if loading fails, the platform falls back to manual keysym lookup.
type Handle struct {
	lib unsafe.Pointer // dlopen("libxkbcommon.so.0")

	// xkb objects (opaque pointers)
	context uintptr // xkb_context*
	keymap  uintptr // xkb_keymap*
	state   uintptr // xkb_state*

	// Cached modifier indices (resolved at keymap creation)
	modsIndices ModsIndices

	// Function pointers -- existing (from Wayland path)
	fnContextNew          unsafe.Pointer
	fnContextUnref        unsafe.Pointer
	fnKeymapNewFromString unsafe.Pointer
	fnKeymapUnref         unsafe.Pointer
	fnStateNew            unsafe.Pointer
	fnStateUnref          unsafe.Pointer
	fnStateKeyGetUtf8     unsafe.Pointer
	fnStateUpdateMask     unsafe.Pointer

	// Function pointers -- Phase 1: X11 and enhanced text input
	fnKeymapNewFromNames unsafe.Pointer // xkb_keymap_new_from_names(ctx, names, flags) -> keymap
	fnStateKeyGetOneSym  unsafe.Pointer // xkb_state_key_get_one_sym(state, key) -> keysym
	fnStateUpdateKey     unsafe.Pointer // xkb_state_update_key(state, key, direction) -> component

	// Function pointers -- Phase 2: shortcut detection
	fnKeymapKeyGetSymsByLevel unsafe.Pointer // xkb_keymap_key_get_syms_by_level(keymap, key, layout, level, &syms) -> count
	fnKeymapModGetIndex       unsafe.Pointer // xkb_keymap_mod_get_index(keymap, name) -> mod_index
	fnStateModNameIsActive    unsafe.Pointer // xkb_state_mod_name_is_active(state, name, type) -> int
	fnStateKeyGetLayout       unsafe.Pointer // xkb_state_key_get_layout(state, key) -> layout_index

	// Call interfaces (prepared once) -- existing
	cifContextNew          types.CallInterface
	cifContextUnref        types.CallInterface
	cifKeymapNewFromString types.CallInterface
	cifKeymapUnref         types.CallInterface
	cifStateNew            types.CallInterface
	cifStateUnref          types.CallInterface
	cifStateKeyGetUtf8     types.CallInterface
	cifStateUpdateMask     types.CallInterface

	// Call interfaces -- Phase 1
	cifKeymapNewFromNames types.CallInterface
	cifStateKeyGetOneSym  types.CallInterface
	cifStateUpdateKey     types.CallInterface

	// Call interfaces -- Phase 2
	cifKeymapKeyGetSymsByLevel types.CallInterface
	cifKeymapModGetIndex       types.CallInterface
	cifStateModNameIsActive    types.CallInterface
	cifStateKeyGetLayout       types.CallInterface
}

// xkbcommon constants.
const (
	// XKB_CONTEXT_NO_FLAGS = 0
	XKBContextNoFlags uint32 = 0

	// XKB_KEYMAP_FORMAT_TEXT_V1 = 1
	XKBKeymapFormatTextV1 uint32 = 1

	// XKB_KEYMAP_COMPILE_NO_FLAGS = 0
	XKBKeymapCompileNoFlags uint32 = 0

	// Evdev keycode offset: evdev keycodes need +8 for xkbcommon.
	XKBEvdevOffset uint32 = 8

	// xkb_state_update_key directions.
	xkbKeyUp   uint32 = 0
	xkbKeyDown uint32 = 1

	// XKBStateModsEffective combines depressed | latched | locked modifiers.
	// Used with xkb_state_mod_name_is_active to check effective modifier state.
	// Value: XKB_STATE_MODS_DEPRESSED(0x08) | XKB_STATE_MODS_LATCHED(0x10) | XKB_STATE_MODS_LOCKED(0x20)
	XKBStateModsEffective uint32 = 0x38

	// xkbModInvalid is the sentinel for "modifier not found".
	// xkbcommon uses XKB_MOD_INVALID = 0xFFFFFFFF (uint32 max).
	// We use int32 -1 since Go doesn't have unsigned enum values.
	xkbModInvalid int32 = -1
)

// New loads libxkbcommon.so.0 and creates an xkb_context.
// Returns (nil, error) if the library is not available -- caller should fall back gracefully.
func New() (*Handle, error) {
	h := &Handle{}

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
func (h *Handle) SetKeymapFromFD(fd int, size uint32) error {
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

	// Resolve modifier indices for shortcut detection (Phase 2)
	h.resolveModsIndices()

	slog.Debug("xkbcommon: keymap loaded from compositor fd", "size", size)
	return nil
}

// SetKeymapFromNames creates a keymap from system default XKB configuration.
// Calls xkb_keymap_new_from_names(context, NULL, 0) which reads RMLVO from:
// - XKB_DEFAULT_* environment variables
// - /etc/default/keyboard
// - System defaults
//
// This is the X11 path. After this call, KeyGetUtf8 and UpdateKey are ready to use.
func (h *Handle) SetKeymapFromNames() error {
	if h == nil {
		return fmt.Errorf("xkbcommon: handle is nil")
	}

	if h.fnKeymapNewFromNames == nil {
		return fmt.Errorf("xkbcommon: xkb_keymap_new_from_names not available")
	}

	// Destroy old keymap/state if present
	h.destroyState()
	h.destroyKeymap()

	// xkb_keymap_new_from_names(context, NULL, XKB_KEYMAP_COMPILE_NO_FLAGS)
	var namesPtr uintptr // NULL = use system defaults
	flags := XKBKeymapCompileNoFlags

	var result uintptr
	args := [3]unsafe.Pointer{
		unsafe.Pointer(&h.context),
		unsafe.Pointer(&namesPtr),
		unsafe.Pointer(&flags),
	}
	_ = ffi.CallFunction(&h.cifKeymapNewFromNames, h.fnKeymapNewFromNames, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return fmt.Errorf("xkbcommon: xkb_keymap_new_from_names returned NULL (no system keymap?)")
	}
	h.keymap = result

	// xkb_state_new(keymap)
	state, err := h.stateNew(result)
	if err != nil {
		h.destroyKeymap()
		return err
	}
	h.state = state

	// Resolve modifier indices for shortcut detection (Phase 2)
	h.resolveModsIndices()

	slog.Debug("xkbcommon: keymap loaded from system defaults (xkb_keymap_new_from_names)")
	return nil
}

// KeyGetUtf8 converts a key press to UTF-8 text using the current xkb state.
// keycode is the evdev keycode (NOT offset -- offset is applied internally).
// Returns empty string for non-printable keys.
func (h *Handle) KeyGetUtf8(keycode uint32) string {
	if h == nil || h.state == 0 {
		return ""
	}

	// xkbcommon expects evdev keycode + 8
	xkbKey := keycode + XKBEvdevOffset

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

// KeyGetOneSym returns the keysym for a key press using the current xkb state.
// keycode is the evdev keycode (NOT offset -- offset is applied internally).
// Returns 0 if no keysym is found or xkb_state_key_get_one_sym is not available.
func (h *Handle) KeyGetOneSym(keycode uint32) uint32 {
	if h == nil || h.state == 0 || h.fnStateKeyGetOneSym == nil {
		return 0
	}

	xkbKey := keycode + XKBEvdevOffset

	var result uint32
	args := [2]unsafe.Pointer{
		unsafe.Pointer(&h.state),
		unsafe.Pointer(&xkbKey),
	}
	_ = ffi.CallFunction(&h.cifStateKeyGetOneSym, h.fnStateKeyGetOneSym, unsafe.Pointer(&result), args[:])
	return result
}

// UpdateMask updates the xkb state with modifier and layout group changes.
// All 6 parameters match xkb_state_update_mask exactly.
// On Wayland: called from wl_keyboard.modifiers (layoutDepressed/layoutLatched = 0).
// On X11: called from XkbStateNotify with full baseMods/latchedMods/lockedMods + group fields.
func (h *Handle) UpdateMask(modsDepressed, modsLatched, modsLocked, layoutDepressed, layoutLatched, layoutLocked uint32) {
	if h == nil || h.state == 0 {
		return
	}
	h.stateUpdateMask(modsDepressed, modsLatched, modsLocked, layoutDepressed, layoutLatched, layoutLocked)
}

// UpdateKey updates the xkb state for a key press or release.
// Used on X11 where we track key state directly instead of receiving modifier masks.
// keycode is the evdev keycode (NOT offset -- offset is applied internally).
// pressed=true for key down, false for key up.
func (h *Handle) UpdateKey(keycode uint32, pressed bool) {
	if h == nil || h.state == 0 || h.fnStateUpdateKey == nil {
		return
	}

	xkbKey := keycode + XKBEvdevOffset
	direction := xkbKeyUp
	if pressed {
		direction = xkbKeyDown
	}

	var result uint32
	args := [3]unsafe.Pointer{
		unsafe.Pointer(&h.state),
		unsafe.Pointer(&xkbKey),
		unsafe.Pointer(&direction),
	}
	_ = ffi.CallFunction(&h.cifStateUpdateKey, h.fnStateUpdateKey, unsafe.Pointer(&result), args[:])
}

// Ready returns true if xkbcommon is loaded and a keymap is active.
func (h *Handle) Ready() bool {
	return h != nil && h.state != 0
}

// Close destroys all xkb objects and unloads the library.
func (h *Handle) Close() {
	if h == nil {
		return
	}
	h.destroyState()
	h.destroyKeymap()
	h.destroyContext()
	// Note: we don't unload the library (ffi doesn't expose dlclose),
	// it stays loaded for the process lifetime.
}

// --- Phase 2: Shortcut detection public API ---

// KeyGetLayout returns the active layout index for a keycode.
// keycode is the evdev keycode (NOT offset -- offset is applied internally).
// Returns 0 when no keymap/state is loaded or the function is unavailable.
func (h *Handle) KeyGetLayout(keycode uint32) uint32 {
	if h == nil || h.state == 0 || h.fnStateKeyGetLayout == nil {
		return 0
	}
	xkbKey := keycode + XKBEvdevOffset
	return h.stateKeyGetLayout(xkbKey)
}

// KeyWithoutModifiers returns the base keysym (level 0) for a keycode on the active layout.
// Used for shortcut matching: matches the key's identity without modifier effects.
// keycode is the evdev keycode (NOT offset -- offset is applied internally).
// Returns 0 when no keymap is loaded or the function is unavailable.
func (h *Handle) KeyWithoutModifiers(keycode uint32) uint32 {
	if h == nil || h.keymap == 0 || h.fnKeymapKeyGetSymsByLevel == nil || h.fnStateKeyGetLayout == nil {
		return 0
	}

	xkbKey := keycode + XKBEvdevOffset

	// Get active layout for this keycode
	layout := h.stateKeyGetLayout(xkbKey)

	// Get keysym at level 0 (no modifiers) on the active layout
	keysym, count := h.keymapKeyGetSymsByLevel(xkbKey, layout, 0)
	if count >= 1 {
		return keysym
	}
	return 0
}

// ModNameIsActive returns true if the named modifier is currently active.
// name must be a null-terminated C string (e.g., "Shift\x00").
// Uses XKB_STATE_MODS_EFFECTIVE (depressed | latched | locked).
// Returns false when no state is loaded or the function is unavailable.
func (h *Handle) ModNameIsActive(name string) bool {
	if h == nil || h.state == 0 || h.fnStateModNameIsActive == nil {
		return false
	}
	return h.stateModNameIsActive(name) == 1
}

// GetModsIndices returns the cached modifier indices resolved at keymap creation.
func (h *Handle) GetModsIndices() ModsIndices {
	if h == nil {
		return ModsIndices{
			Shift: xkbModInvalid, CapsLock: xkbModInvalid, Control: xkbModInvalid,
			Alt: xkbModInvalid, NumLock: xkbModInvalid, Mod3: xkbModInvalid,
			Super: xkbModInvalid, Mod5: xkbModInvalid,
		}
	}
	return h.modsIndices
}

// --- Internal methods ---

// resolveModsIndices resolves all 8 standard modifier indices by name at keymap creation.
// Called from SetKeymapFromFD and SetKeymapFromNames after the state is created.
func (h *Handle) resolveModsIndices() {
	if h.fnKeymapModGetIndex == nil || h.keymap == 0 {
		// All indices stay at xkbModInvalid
		h.modsIndices = ModsIndices{
			Shift: xkbModInvalid, CapsLock: xkbModInvalid, Control: xkbModInvalid,
			Alt: xkbModInvalid, NumLock: xkbModInvalid, Mod3: xkbModInvalid,
			Super: xkbModInvalid, Mod5: xkbModInvalid,
		}
		return
	}

	h.modsIndices = ModsIndices{
		Shift:    h.keymapModGetIndex("Shift\x00"),
		CapsLock: h.keymapModGetIndex("Lock\x00"),
		Control:  h.keymapModGetIndex("Control\x00"),
		Alt:      h.keymapModGetIndex("Mod1\x00"),
		NumLock:  h.keymapModGetIndex("Mod2\x00"),
		Mod3:     h.keymapModGetIndex("Mod3\x00"),
		Super:    h.keymapModGetIndex("Mod4\x00"),
		Mod5:     h.keymapModGetIndex("Mod5\x00"),
	}
}

// --- Phase 2: internal FFI wrappers ---

func (h *Handle) resolveSymbols() error {
	// Required symbols (must exist in libxkbcommon.so.0)
	required := []struct {
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

	for _, s := range required {
		sym, err := ffi.GetSymbol(h.lib, s.name)
		if err != nil {
			return fmt.Errorf("xkbcommon: symbol %s not found: %w", s.name, err)
		}
		if sym == nil {
			return fmt.Errorf("xkbcommon: symbol %s is nil", s.name)
		}
		*s.dst = sym
	}

	// Optional symbols (new bindings for X11 path -- not fatal if missing)
	optional := []struct {
		name string
		dst  *unsafe.Pointer
	}{
		{"xkb_keymap_new_from_names", &h.fnKeymapNewFromNames},
		{"xkb_state_key_get_one_sym", &h.fnStateKeyGetOneSym},
		{"xkb_state_update_key", &h.fnStateUpdateKey},
	}

	for _, s := range optional {
		sym, err := ffi.GetSymbol(h.lib, s.name)
		if err != nil {
			slog.Debug("xkbcommon: optional symbol not found", "name", s.name, "err", err)
			continue
		}
		if sym == nil {
			continue
		}
		*s.dst = sym
	}

	// Optional symbols -- Phase 2: shortcut detection (not fatal if missing)
	phase2 := []struct {
		name string
		dst  *unsafe.Pointer
	}{
		{"xkb_keymap_key_get_syms_by_level", &h.fnKeymapKeyGetSymsByLevel},
		{"xkb_keymap_mod_get_index", &h.fnKeymapModGetIndex},
		{"xkb_state_mod_name_is_active", &h.fnStateModNameIsActive},
		{"xkb_state_key_get_layout", &h.fnStateKeyGetLayout},
	}

	for _, s := range phase2 {
		sym, err := ffi.GetSymbol(h.lib, s.name)
		if err != nil {
			slog.Debug("xkbcommon: optional symbol not found (Phase 2)", "name", s.name, "err", err)
			continue
		}
		if sym == nil {
			continue
		}
		*s.dst = sym
	}

	return nil
}

func (h *Handle) prepareCIFs() error {
	ptr := types.PointerTypeDescriptor
	u32 := types.UInt32TypeDescriptor
	i32 := types.SInt32TypeDescriptor
	void := types.VoidTypeDescriptor

	// Required CIFs
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
		// enum xkb_state_component xkb_state_update_mask(state, dep, lat, locked, ldep, llat, llocked)
		{"state_update_mask", &h.cifStateUpdateMask, u32, []*types.TypeDescriptor{ptr, u32, u32, u32, u32, u32, u32}},
	}

	for _, d := range cifDefs {
		if err := ffi.PrepareCallInterface(d.cif, types.DefaultCall, d.ret, d.args); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF %s: %w", d.name, err)
		}
	}

	// Optional CIFs (only prepare if symbol was resolved)
	if h.fnKeymapNewFromNames != nil {
		// struct xkb_keymap* xkb_keymap_new_from_names(context, rmlvo_names*, flags)
		if err := ffi.PrepareCallInterface(&h.cifKeymapNewFromNames, types.DefaultCall, ptr, []*types.TypeDescriptor{ptr, ptr, u32}); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF keymap_new_from_names: %w", err)
		}
	}

	if h.fnStateKeyGetOneSym != nil {
		// xkb_keysym_t xkb_state_key_get_one_sym(state, key) -> uint32
		if err := ffi.PrepareCallInterface(&h.cifStateKeyGetOneSym, types.DefaultCall, u32, []*types.TypeDescriptor{ptr, u32}); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF state_key_get_one_sym: %w", err)
		}
	}

	if h.fnStateUpdateKey != nil {
		// enum xkb_state_component xkb_state_update_key(state, key, direction) -> uint32
		if err := ffi.PrepareCallInterface(&h.cifStateUpdateKey, types.DefaultCall, u32, []*types.TypeDescriptor{ptr, u32, u32}); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF state_update_key: %w", err)
		}
	}

	// Phase 2 optional CIFs -- shortcut detection
	if h.fnKeymapKeyGetSymsByLevel != nil {
		// int xkb_keymap_key_get_syms_by_level(keymap, key, layout, level, &syms_out) -> int32
		if err := ffi.PrepareCallInterface(&h.cifKeymapKeyGetSymsByLevel, types.DefaultCall, i32, []*types.TypeDescriptor{ptr, u32, u32, u32, ptr}); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF keymap_key_get_syms_by_level: %w", err)
		}
	}

	if h.fnKeymapModGetIndex != nil {
		// xkb_mod_index_t xkb_keymap_mod_get_index(keymap, name) -> uint32
		if err := ffi.PrepareCallInterface(&h.cifKeymapModGetIndex, types.DefaultCall, u32, []*types.TypeDescriptor{ptr, ptr}); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF keymap_mod_get_index: %w", err)
		}
	}

	if h.fnStateModNameIsActive != nil {
		// int xkb_state_mod_name_is_active(state, name, type) -> int32
		if err := ffi.PrepareCallInterface(&h.cifStateModNameIsActive, types.DefaultCall, i32, []*types.TypeDescriptor{ptr, ptr, u32}); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF state_mod_name_is_active: %w", err)
		}
	}

	if h.fnStateKeyGetLayout != nil {
		// xkb_layout_index_t xkb_state_key_get_layout(state, key) -> uint32
		if err := ffi.PrepareCallInterface(&h.cifStateKeyGetLayout, types.DefaultCall, u32, []*types.TypeDescriptor{ptr, u32}); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF state_key_get_layout: %w", err)
		}
	}

	return nil
}

// contextNew calls xkb_context_new(XKB_CONTEXT_NO_FLAGS).
func (h *Handle) contextNew() (uintptr, error) {
	flags := XKBContextNoFlags
	var result uintptr
	args := [1]unsafe.Pointer{unsafe.Pointer(&flags)}
	_ = ffi.CallFunction(&h.cifContextNew, h.fnContextNew, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("xkbcommon: xkb_context_new returned NULL")
	}
	return result, nil
}

// keymapNewFromString calls xkb_keymap_new_from_string.
func (h *Handle) keymapNewFromString(keymapStr []byte) (uintptr, error) {
	// Ensure null terminator (allocate new slice to avoid mutating input)
	cstr := make([]byte, len(keymapStr)+1)
	copy(cstr, keymapStr)
	strPtr := uintptr(unsafe.Pointer(&cstr[0]))
	format := XKBKeymapFormatTextV1
	flags := XKBKeymapCompileNoFlags

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
func (h *Handle) stateNew(keymap uintptr) (uintptr, error) {
	var result uintptr
	args := [1]unsafe.Pointer{unsafe.Pointer(&keymap)}
	_ = ffi.CallFunction(&h.cifStateNew, h.fnStateNew, unsafe.Pointer(&result), args[:])
	if result == 0 {
		return 0, fmt.Errorf("xkbcommon: xkb_state_new returned NULL")
	}
	return result, nil
}

// stateKeyGetUtf8 calls xkb_state_key_get_utf8(state, key, buffer, size) -> int.
func (h *Handle) stateKeyGetUtf8(key uint32, buf []byte, bufSize uint32) int32 {
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
func (h *Handle) stateUpdateMask(modsDepressed, modsLatched, modsLocked, layoutDepressed, layoutLatched, layoutLocked uint32) {
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

func (h *Handle) destroyState() {
	if h.state != 0 {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.state)}
		ffi.CallFunction(&h.cifStateUnref, h.fnStateUnref, nil, args[:])
		h.state = 0
	}
}

func (h *Handle) destroyKeymap() {
	if h.keymap != 0 {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.keymap)}
		ffi.CallFunction(&h.cifKeymapUnref, h.fnKeymapUnref, nil, args[:])
		h.keymap = 0
	}
}

func (h *Handle) destroyContext() {
	if h.context != 0 {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.context)}
		ffi.CallFunction(&h.cifContextUnref, h.fnContextUnref, nil, args[:])
		h.context = 0
	}
}

// keymapModGetIndex calls xkb_keymap_mod_get_index(keymap, name) -> uint32.
// name must be a null-terminated C string. Returns xkbModInvalid (-1) if not found.
// xkbcommon returns XKB_MOD_INVALID (0xFFFFFFFF) for unknown modifiers.
func (h *Handle) keymapModGetIndex(name string) int32 {
	nameBytes := []byte(name)
	namePtr := uintptr(unsafe.Pointer(&nameBytes[0]))

	var result uint32
	args := [2]unsafe.Pointer{
		unsafe.Pointer(&h.keymap),
		unsafe.Pointer(&namePtr),
	}
	_ = ffi.CallFunction(&h.cifKeymapModGetIndex, h.fnKeymapModGetIndex, unsafe.Pointer(&result), args[:])

	// XKB_MOD_INVALID = 0xFFFFFFFF -> convert to int32 -1
	if result == 0xFFFFFFFF {
		return xkbModInvalid
	}
	return int32(result)
}

// stateModNameIsActive calls xkb_state_mod_name_is_active(state, name, type) -> int32.
// name must be a null-terminated C string. Returns 1 if active, 0 if not.
func (h *Handle) stateModNameIsActive(name string) int32 {
	nameBytes := []byte(name)
	namePtr := uintptr(unsafe.Pointer(&nameBytes[0]))
	stateType := XKBStateModsEffective

	var result int32
	args := [3]unsafe.Pointer{
		unsafe.Pointer(&h.state),
		unsafe.Pointer(&namePtr),
		unsafe.Pointer(&stateType),
	}
	_ = ffi.CallFunction(&h.cifStateModNameIsActive, h.fnStateModNameIsActive, unsafe.Pointer(&result), args[:])
	return result
}

// stateKeyGetLayout calls xkb_state_key_get_layout(state, key) -> uint32.
// key is the xkbcommon keycode (evdev + 8).
func (h *Handle) stateKeyGetLayout(key uint32) uint32 {
	var result uint32
	args := [2]unsafe.Pointer{
		unsafe.Pointer(&h.state),
		unsafe.Pointer(&key),
	}
	_ = ffi.CallFunction(&h.cifStateKeyGetLayout, h.fnStateKeyGetLayout, unsafe.Pointer(&result), args[:])
	return result
}

// keymapKeyGetSymsByLevel calls xkb_keymap_key_get_syms_by_level(keymap, key, layout, level, &syms_out).
// Returns (first_keysym, count). syms_out points to xkbcommon-owned memory valid until keymap is destroyed.
// key is the xkbcommon keycode (evdev + 8).
func (h *Handle) keymapKeyGetSymsByLevel(key, layout, level uint32) (uint32, int32) {
	var symsPtr unsafe.Pointer
	var count int32
	args := [5]unsafe.Pointer{
		unsafe.Pointer(&h.keymap),
		unsafe.Pointer(&key),
		unsafe.Pointer(&layout),
		unsafe.Pointer(&level),
		unsafe.Pointer(&symsPtr),
	}
	_ = ffi.CallFunction(&h.cifKeymapKeyGetSymsByLevel, h.fnKeymapKeyGetSymsByLevel, unsafe.Pointer(&count), args[:])

	if count >= 1 && symsPtr != nil {
		keysym := *(*uint32)(symsPtr)
		return keysym, count
	}
	return 0, count
}
