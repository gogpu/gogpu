//go:build linux

package wayland

import (
	"testing"
	"unsafe"
)

func TestTextInputProtocolDescriptors(t *testing.T) {
	initTextInputInterfaces()
	if got := textInputInterfaces.manager.MethodCount; got != 2 {
		t.Fatalf("manager method count = %d, want 2", got)
	}
	if got := textInputInterfaces.text.MethodCount; got != 8 {
		t.Fatalf("text-input method count = %d, want 8", got)
	}
	if got := textInputInterfaces.text.EventCount; got != 6 {
		t.Fatalf("text-input event count = %d, want 6", got)
	}
}

func TestTextInputDoneAggregatesAndDeduplicates(t *testing.T) {
	const proxy = uintptr(0x71)
	var got TextInputUpdate
	h := &LibwaylandHandle{inputCallbacks: &InputCallbacks{
		OnTextInputDone: func(update TextInputUpdate) { got = update },
	}}
	textInputHandlesMu.Lock()
	textInputHandles[proxy] = h
	textInputHandlesMu.Unlock()
	t.Cleanup(func() {
		textInputHandlesMu.Lock()
		delete(textInputHandles, proxy)
		textInputHandlesMu.Unlock()
	})

	preedit := make([]byte, 1<<20)
	copy(preedit, "日本")
	commit := make([]byte, 1<<20)
	copy(commit, "に")
	textInputPreeditCb(0, proxy, uintptr(unsafe.Pointer(&preedit[0])), 0, 6)
	textInputCommitCb(0, proxy, uintptr(unsafe.Pointer(&commit[0])))
	// A duplicate commit in one transaction is ignored.
	textInputCommitCb(0, proxy, uintptr(unsafe.Pointer(&preedit[0])))
	textInputDeleteCb(0, proxy, 3, 2)
	textInputDoneCb(0, proxy, 4)

	if !got.HasPreedit || got.PreeditText != "日本" || got.CursorBegin != 0 || got.CursorEnd != 6 {
		t.Fatalf("unexpected preedit update: %+v", got)
	}
	if !got.HasCommit || got.CommitText != "に" {
		t.Fatalf("unexpected commit update: %+v", got)
	}
	if !got.HasDelete || got.DeleteBefore != 3 || got.DeleteAfter != 2 {
		t.Fatalf("unexpected delete update: %+v", got)
	}
}
