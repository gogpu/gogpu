//go:build linux

package xkb

import (
	"fmt"
	"log/slog"
	"os"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// xkbcommon compose constants.
const (
	// XKB_COMPOSE_COMPILE_NO_FLAGS and XKB_COMPOSE_STATE_NO_FLAGS.
	xkbComposeNoFlags uint32 = 0

	// XKB_COMPOSE_FEED_IGNORED: the keysym, a modifier, leaves the sequence as it is.
	xkbComposeFeedIgnored uint32 = 0

	// XKB_COMPOSE_NOTHING and XKB_COMPOSE_COMPOSED, of enum xkb_compose_status.
	xkbComposeNothing  uint32 = 0
	xkbComposeComposed uint32 = 2
)

// KeyGetComposedUtf8 answers the UTF-8 text a pressed key types once it has
// gone through the compose table of the user's locale: nothing for a dead key,
// the Compose key or a key breaking a sequence off, the sequence's text for the
// key completing it, and KeyGetUtf8's text for any other key.
// keycode is the evdev keycode (NOT offset -- offset is applied internally).
func (h *Handle) KeyGetComposedUtf8(keycode uint32) string {
	if h != nil && h.composeState != 0 {
		if composed, taken := h.FeedCompose(h.KeyGetOneSym(keycode)); taken {
			return composed
		}
	}
	return h.KeyGetUtf8(keycode)
}

// FeedCompose feeds a pressed key's keysym to the compose table of the user's
// locale, which dead keys and the Compose key go through. It answers whether the
// table took the key, which then types composed alone: the text of the sequence
// it completes, or nothing. Without a compose table, and for a keysym the table
// ignores or no sequence holds, the key is not taken and types its own text.
func (h *Handle) FeedCompose(keysym uint32) (composed string, taken bool) {
	if h == nil || h.composeState == 0 || keysym == 0 {
		return "", false
	}
	if h.composeStateFeed(keysym) == xkbComposeFeedIgnored {
		return "", false
	}
	switch h.composeStateGetStatus() {
	case xkbComposeNothing:
		return "", false
	case xkbComposeComposed:
		return h.composeStateGetUtf8(), true
	}
	return "", true
}

// initCompose loads the compose table of the user's locale and a state over it.
// Non-fatal: without it, keys type their own text, dead keys none.
func (h *Handle) initCompose() {
	symbols := []struct {
		name string
		dst  *unsafe.Pointer
	}{
		{"xkb_compose_table_new_from_locale", &h.fnComposeTableNewFromLocale},
		{"xkb_compose_table_unref", &h.fnComposeTableUnref},
		{"xkb_compose_state_new", &h.fnComposeStateNew},
		{"xkb_compose_state_unref", &h.fnComposeStateUnref},
		{"xkb_compose_state_feed", &h.fnComposeStateFeed},
		{"xkb_compose_state_get_status", &h.fnComposeStateGetStatus},
		{"xkb_compose_state_get_utf8", &h.fnComposeStateGetUtf8},
	}
	for _, s := range symbols {
		sym, err := ffi.GetSymbol(h.lib, s.name)
		if err != nil || sym == nil {
			slog.Debug("xkbcommon: compose symbol not found, dead keys type nothing", "name", s.name, "err", err)
			return
		}
		*s.dst = sym
	}

	if err := h.prepareComposeCIFs(); err != nil {
		slog.Debug("xkbcommon: compose unavailable", "err", err)
		return
	}

	locale := composeLocale()
	table := h.composeTableNewFromLocale(locale)
	if table == 0 {
		slog.Debug("xkbcommon: no compose table for the locale, dead keys type nothing", "locale", locale)
		return
	}
	// The state holds its own reference to the table.
	h.composeState = h.composeStateNew(table)
	h.composeTableUnref(table)
	if h.composeState == 0 {
		slog.Debug("xkbcommon: xkb_compose_state_new returned NULL")
		return
	}
	slog.Debug("xkbcommon: compose table loaded", "locale", locale)
}

// destroyCompose releases the compose state, and with it the table.
func (h *Handle) destroyCompose() {
	if h.composeState != 0 {
		args := [1]unsafe.Pointer{unsafe.Pointer(&h.composeState)}
		_, _ = ffi.CallFunction(&h.cifComposeStateUnref, h.fnComposeStateUnref, nil, args[:])
		h.composeState = 0
	}
}

// composeLocale answers the locale whose compose table is loaded, found the
// way setlocale finds LC_CTYPE's.
func composeLocale() string {
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if locale := os.Getenv(name); locale != "" {
			return locale
		}
	}
	return "C"
}

func (h *Handle) prepareComposeCIFs() error {
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
		// struct xkb_compose_table* xkb_compose_table_new_from_locale(context, locale, flags)
		{"compose_table_new_from_locale", &h.cifComposeTableNewFromLocale, ptr, []*types.TypeDescriptor{ptr, ptr, u32}},
		// void xkb_compose_table_unref(struct xkb_compose_table *table)
		{"compose_table_unref", &h.cifComposeTableUnref, void, []*types.TypeDescriptor{ptr}},
		// struct xkb_compose_state* xkb_compose_state_new(table, flags)
		{"compose_state_new", &h.cifComposeStateNew, ptr, []*types.TypeDescriptor{ptr, u32}},
		// void xkb_compose_state_unref(struct xkb_compose_state *state)
		{"compose_state_unref", &h.cifComposeStateUnref, void, []*types.TypeDescriptor{ptr}},
		// enum xkb_compose_feed_result xkb_compose_state_feed(state, keysym)
		{"compose_state_feed", &h.cifComposeStateFeed, u32, []*types.TypeDescriptor{ptr, u32}},
		// enum xkb_compose_status xkb_compose_state_get_status(state)
		{"compose_state_get_status", &h.cifComposeStateGetStatus, u32, []*types.TypeDescriptor{ptr}},
		// int xkb_compose_state_get_utf8(state, buffer, size_t size)
		{"compose_state_get_utf8", &h.cifComposeStateGetUtf8, i32, []*types.TypeDescriptor{ptr, ptr, types.UInt64TypeDescriptor}},
	}
	for _, d := range cifDefs {
		if err := ffi.PrepareCallInterface(d.cif, types.DefaultCall, d.ret, d.args); err != nil {
			return fmt.Errorf("xkbcommon: failed to prepare CIF %s: %w", d.name, err)
		}
	}
	return nil
}

// composeTableNewFromLocale calls xkb_compose_table_new_from_locale, and
// answers 0 when the locale has no compose table.
func (h *Handle) composeTableNewFromLocale(locale string) uintptr {
	cstr := append([]byte(locale), 0)
	// unsafe.Pointer, not uintptr: the GC and the stack copier see it (P1-FFI-001).
	localePtr := unsafe.Pointer(&cstr[0])
	flags := xkbComposeNoFlags

	var result uintptr
	args := [3]unsafe.Pointer{
		unsafe.Pointer(&h.context),
		unsafe.Pointer(&localePtr),
		unsafe.Pointer(&flags),
	}
	_, _ = ffi.CallFunction(&h.cifComposeTableNewFromLocale, h.fnComposeTableNewFromLocale, unsafe.Pointer(&result), args[:])
	return result
}

// composeTableUnref calls xkb_compose_table_unref.
func (h *Handle) composeTableUnref(table uintptr) {
	args := [1]unsafe.Pointer{unsafe.Pointer(&table)}
	_, _ = ffi.CallFunction(&h.cifComposeTableUnref, h.fnComposeTableUnref, nil, args[:])
}

// composeStateNew calls xkb_compose_state_new.
func (h *Handle) composeStateNew(table uintptr) uintptr {
	flags := xkbComposeNoFlags
	var result uintptr
	args := [2]unsafe.Pointer{
		unsafe.Pointer(&table),
		unsafe.Pointer(&flags),
	}
	_, _ = ffi.CallFunction(&h.cifComposeStateNew, h.fnComposeStateNew, unsafe.Pointer(&result), args[:])
	return result
}

// composeStateFeed calls xkb_compose_state_feed, and answers its feed result.
func (h *Handle) composeStateFeed(keysym uint32) uint32 {
	var result uint32
	args := [2]unsafe.Pointer{
		unsafe.Pointer(&h.composeState),
		unsafe.Pointer(&keysym),
	}
	_, _ = ffi.CallFunction(&h.cifComposeStateFeed, h.fnComposeStateFeed, unsafe.Pointer(&result), args[:])
	return result
}

// composeStateGetStatus calls xkb_compose_state_get_status.
func (h *Handle) composeStateGetStatus() uint32 {
	var result uint32
	args := [1]unsafe.Pointer{unsafe.Pointer(&h.composeState)}
	_, _ = ffi.CallFunction(&h.cifComposeStateGetStatus, h.fnComposeStateGetStatus, unsafe.Pointer(&result), args[:])
	return result
}

// composeStateGetUtf8 answers the text of the sequence just composed, through
// xkb_compose_state_get_utf8.
func (h *Handle) composeStateGetUtf8() string {
	size := h.composeStateGetUtf8Into(nil)
	if size <= 0 {
		return ""
	}
	buf := make([]byte, size+1)
	written := h.composeStateGetUtf8Into(buf)
	if written <= 0 {
		return ""
	}
	return string(buf[:written])
}

// composeStateGetUtf8Into calls xkb_compose_state_get_utf8 with buf, and
// answers the length of the text without its NUL.
func (h *Handle) composeStateGetUtf8Into(buf []byte) int32 {
	// unsafe.Pointer, not uintptr: xkbcommon writes into buf, which must stay in
	// place and alive for the call (P1-FFI-001).
	var bufPtr unsafe.Pointer
	if len(buf) > 0 {
		bufPtr = unsafe.Pointer(&buf[0])
	}
	size := uint64(len(buf))

	var result int32
	args := [3]unsafe.Pointer{
		unsafe.Pointer(&h.composeState),
		unsafe.Pointer(&bufPtr),
		unsafe.Pointer(&size),
	}
	_, _ = ffi.CallFunction(&h.cifComposeStateGetUtf8, h.fnComposeStateGetUtf8, unsafe.Pointer(&result), args[:])
	return result
}
