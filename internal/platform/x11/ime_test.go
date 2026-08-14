//go:build linux

package x11

import (
	"encoding/binary"
	"strconv"
	"testing"
	"unicode/utf8"
	"unsafe"

	"github.com/gogpu/gpucontext"
)

func TestXIMPreeditReplacementUsesUTF8Ranges(t *testing.T) {
	updated, start, end := replaceXIMPreedit("a界c", 1, 1, "🙂")
	if updated != "a🙂c" {
		t.Fatalf("updated preedit = %q, want %q", updated, "a🙂c")
	}
	if start != len("a") || end != len("a🙂") {
		t.Fatalf("replacement range = [%d,%d), want [%d,%d)", start, end, len("a"), len("a🙂"))
	}
	composition := gpucontext.IMEComposition{
		CompositionText: updated,
		CursorBegin:     end,
		CursorEnd:       end,
		SelectionStart:  start,
		SelectionEnd:    end,
	}
	if !composition.IsValid() {
		t.Fatal("replacement produced invalid UTF-8 composition ranges")
	}
}

func TestXIMPreeditReplacementClampsChanges(t *testing.T) {
	updated, start, end := replaceXIMPreedit("abc", 100, 500, "x")
	if updated != "abcx" || start != len("abc") || end != len("abcx") {
		t.Fatalf("clamped replacement = (%q,%d,%d), want (abcx,3,4)", updated, start, end)
	}
	updated, start, end = replaceXIMPreedit("abc", -1, -5, "")
	if updated != "abc" || start != 0 || end != 0 {
		t.Fatalf("negative replacement = (%q,%d,%d), want (abc,0,0)", updated, start, end)
	}
}

func TestXIMRuneByteOffset(t *testing.T) {
	text := "a界🙂z"
	for i, want := range []int{0, 1, len("a界"), len("a界🙂"), len(text), len(text)} {
		if got := runeByteOffset(text, i); got != want {
			t.Errorf("runeByteOffset(%d) = %d, want %d", i, got, want)
		}
	}
	if !utf8.ValidString(text) {
		t.Fatal("test text unexpectedly invalid")
	}
}

func TestXIMTextStringRejectsWideAndInvalidPayloads(t *testing.T) {
	if got := ximTextString(0); got != "" {
		t.Fatalf("nil XIMText = %q, want empty", got)
	}
	// XIMText with encoding_is_wchar is intentionally left to the locale/
	// xkb fallback because its wchar_t width varies by platform ABI.
	wide := ximText{Length: 1, EncodingWide: 1, String: 1}
	if got := ximTextString(uintptr(unsafe.Pointer(&wide))); got != "" {
		t.Fatalf("wide XIMText = %q, want empty", got)
	}
}

func TestXIMTextStringReadsUTF8MultiBytePayload(t *testing.T) {
	if strconv.IntSize == 32 {
		if unsafe.Sizeof(ximText{}) != 16 || unsafe.Offsetof(ximText{}.Feedback) != 4 || unsafe.Offsetof(ximText{}.String) != 12 {
			t.Fatalf("ILP32 XIMText layout = size %d, feedback %d, string %d", unsafe.Sizeof(ximText{}), unsafe.Offsetof(ximText{}.Feedback), unsafe.Offsetof(ximText{}.String))
		}
	} else if unsafe.Sizeof(ximText{}) != 32 || unsafe.Offsetof(ximText{}.Feedback) != 8 || unsafe.Offsetof(ximText{}.String) != 24 {
		t.Fatalf("LP64 XIMText layout = size %d, feedback %d, string %d", unsafe.Sizeof(ximText{}), unsafe.Offsetof(ximText{}.Feedback), unsafe.Offsetof(ximText{}.String))
	}
	payload := append([]byte("界🙂"), 0)
	text := ximText{
		Length: uint16(utf8.RuneCount(payload[:len(payload)-1])),
		String: uintptr(unsafe.Pointer(&payload[0])),
	}
	if got := ximTextString(uintptr(unsafe.Pointer(&text))); got != "界🙂" {
		t.Fatalf("multi-byte XIMText = %q, want %q", got, "界🙂")
	}
}

func TestXIMLookupStatusConstantsMatchXlib(t *testing.T) {
	if xBufferOverflow != -1 || xLookupNone != 1 || xLookupChars != 2 ||
		xLookupKeySym != 3 || xLookupBoth != 4 {
		t.Fatalf("XIM lookup constants do not match Xlib: overflow=%d none=%d chars=%d keysym=%d both=%d",
			xBufferOverflow, xLookupNone, xLookupChars, xLookupKeySym, xLookupBoth)
	}
}

func TestNativeKeyEventReleaseType(t *testing.T) {
	raw := nativeKeyEventType(&KeyEvent{}, 0, false)
	if got := binary.NativeEndian.Uint32(raw[0:4]); got != xKeyReleaseEvent {
		t.Fatalf("native release event type = %d, want %d", got, xKeyReleaseEvent)
	}
}

func TestNativeKeyEventCarriesXKeyEventFields(t *testing.T) {
	event := &KeyEvent{
		Sequence:   7,
		Time:       11,
		Event:      13,
		Root:       17,
		Child:      19,
		EventX:     -23,
		EventY:     29,
		RootX:      -31,
		RootY:      37,
		State:      41,
		Detail:     43,
		SameScreen: true,
	}
	raw := nativeKeyEvent(event, 47)
	if got := binary.NativeEndian.Uint32(raw[0:4]); got != xKeyPressEvent {
		t.Fatalf("native event type = %d, want %d", got, xKeyPressEvent)
	}
	if strconv.IntSize == 32 {
		if got := binary.NativeEndian.Uint32(raw[12:16]); got != 47 {
			t.Fatalf("native event display = %d, want 47", got)
		}
		if got := binary.NativeEndian.Uint32(raw[16:20]); got != 13 {
			t.Fatalf("native event window = %d, want 13", got)
		}
		if got := int32(binary.NativeEndian.Uint32(raw[32:36])); got != -23 {
			t.Fatalf("native event x = %d, want -23", got)
		}
		if got := binary.NativeEndian.Uint32(raw[52:56]); got != 43 {
			t.Fatalf("native event keycode = %d, want 43", got)
		}
		if got := binary.NativeEndian.Uint32(raw[56:60]); got != 1 {
			t.Fatalf("native event same_screen = %d, want 1", got)
		}
		return
	}
	if got := binary.NativeEndian.Uint64(raw[24:32]); got != 47 {
		t.Fatalf("native event display = %d, want 47", got)
	}
	if got := binary.NativeEndian.Uint64(raw[32:40]); got != 13 {
		t.Fatalf("native event window = %d, want 13", got)
	}
	if got := int32(binary.NativeEndian.Uint32(raw[64:68])); got != -23 {
		t.Fatalf("native event x = %d, want -23", got)
	}
	if got := binary.NativeEndian.Uint32(raw[84:88]); got != 43 {
		t.Fatalf("native event keycode = %d, want 43", got)
	}
	if got := binary.NativeEndian.Uint32(raw[88:92]); got != 1 {
		t.Fatalf("native event same_screen = %d, want 1", got)
	}
}
