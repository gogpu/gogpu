//go:build windows

package platform

import (
	"testing"
)

// procGetMenuItemCount is declared here for test-only HMENU structure
// verification. user32 is already loaded by platform_windows.go.
var procGetMenuItemCount = user32.NewProc("GetMenuItemCount")

// menuItemCount returns the number of items in an HMENU, or -1 on failure.
func menuItemCount(hMenu uintptr) int {
	n, _, _ := procGetMenuItemCount.Call(hMenu)
	return int(int32(n)) // Win32 returns -1 (0xFFFFFFFF) on error
}

// clearMenuActions removes all entries from the package-level menuActions map.
func clearMenuActions(t *testing.T) {
	t.Helper()
	menuActions.Clear()
}

// --- Command ID allocation ---

func TestNextMenuCmdID_StartsAtOrAbove0x1000(t *testing.T) {
	id := nextMenuCmdID()
	if id < 0x1000 {
		t.Errorf("nextMenuCmdID() = %#x, want >= 0x1000 (systray collision range 0x0001–0x0FFF)", id)
	}
}

func TestNextMenuCmdID_Monotonic(t *testing.T) {
	a := nextMenuCmdID()
	b := nextMenuCmdID()
	if b <= a {
		t.Errorf("IDs not monotonically increasing: got %#x then %#x", a, b)
	}
}

// --- WM_COMMAND dispatch ---

func TestDispatchMenuCommand_InvokesAction(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	var called bool
	id := nextMenuCmdID()
	menuActions.Store(id, func() { called = true })

	dispatchMenuCommand(id)

	if !called {
		t.Error("dispatchMenuCommand: registered action was not invoked")
	}
}

func TestDispatchMenuCommand_CallsExactAction(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	var calls [2]int
	idA := nextMenuCmdID()
	idB := nextMenuCmdID()
	menuActions.Store(idA, func() { calls[0]++ })
	menuActions.Store(idB, func() { calls[1]++ })

	dispatchMenuCommand(idA)
	dispatchMenuCommand(idB)
	dispatchMenuCommand(idA)

	if calls[0] != 2 {
		t.Errorf("idA action: called %d times, want 2", calls[0])
	}
	if calls[1] != 1 {
		t.Errorf("idB action: called %d times, want 1", calls[1])
	}
}

func TestDispatchMenuCommand_UnknownID_NoPanic(t *testing.T) {
	// Must not panic for an unregistered ID.
	dispatchMenuCommand(0xFFFE)
}

// --- AddToSystemMenu ---

func TestAddToSystemMenu_AlwaysFalse(t *testing.T) {
	p := &windowsPlatform{}
	cases := []SystemMenu{SystemMenuApplication, SystemMenuWindow}
	for _, m := range cases {
		if p.AddToSystemMenu(m, nil) {
			t.Errorf("AddToSystemMenu(%v) = true, want false", m)
		}
	}
}

func TestAddToSystemMenu_WithItems_AlwaysFalse(t *testing.T) {
	p := &windowsPlatform{}
	items := []MenuItem{{Title: "Custom"}}
	if p.AddToSystemMenu(SystemMenuApplication, items) {
		t.Error("AddToSystemMenu(SystemMenuApplication, items) = true, want false")
	}
}

// --- HMENU structure (real Win32 calls, no HWND required) ---

func TestBuildMenuPopup_Empty(t *testing.T) {
	hPopup := buildMenuPopup(nil)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup(nil): returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)

	if got := menuItemCount(hPopup); got != 0 {
		t.Errorf("GetMenuItemCount = %d, want 0 for empty item list", got)
	}
}

func TestBuildMenuPopup_LeafItems(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{Title: "Cut"},
		{Title: "Copy"},
		{Title: "Paste"},
	}
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)

	if got := menuItemCount(hPopup); got != 3 {
		t.Errorf("GetMenuItemCount = %d, want 3", got)
	}
}

func TestBuildMenuPopup_Separator(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{Title: "Open"},
		{Separator: true},
		{Title: "Close"},
	}
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)

	// Separator counts as one entry in Win32 HMENU.
	if got := menuItemCount(hPopup); got != 3 {
		t.Errorf("GetMenuItemCount = %d, want 3 (2 items + 1 separator)", got)
	}
}

func TestBuildMenuPopup_Submenu(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{
			Title: "Format",
			Submenu: []MenuItem{
				{Title: "Bold"},
				{Title: "Italic"},
				{Title: "Underline"},
			},
		},
	}
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)

	// Top-level has one entry (the submenu itself).
	if got := menuItemCount(hPopup); got != 1 {
		t.Errorf("GetMenuItemCount (top) = %d, want 1", got)
	}
}

func TestBuildMenuPopup_MixedContent(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{Title: "New"},
		{Title: "Open"},
		{Separator: true},
		{Title: "Recent", Submenu: []MenuItem{{Title: "file1.go"}, {Title: "file2.go"}}},
		{Separator: true},
		{Title: "Quit", Role: MenuRoleQuit},
	}
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)

	if got := menuItemCount(hPopup); got != 6 {
		t.Errorf("GetMenuItemCount = %d, want 6", got)
	}
}

// --- Action registration ---

func TestBuildMenuPopup_ActionRegistered(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	var called bool
	items := []MenuItem{
		{Title: "Undo", Action: func() { called = true }},
	}

	before := menuCmdIDCounter.Load()
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)
	after := menuCmdIDCounter.Load()

	// Dispatch every ID allocated during the build and verify the action fires.
	for id := uint16(before); id < uint16(after); id++ { //nolint:gocritic // safe: counter range 0x1000–0xFFFF fits uint16
		dispatchMenuCommand(id)
	}
	if !called {
		t.Error("action for 'Undo' item was not registered or not dispatched")
	}
}

func TestBuildMenuPopup_NoAction_NotRegistered(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{Title: "Header"}, // no Action, no Role
	}

	before := menuCmdIDCounter.Load()
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)
	after := menuCmdIDCounter.Load()

	for id := uint16(before); id < uint16(after); id++ { //nolint:gocritic // safe: counter range 0x1000–0xFFFF fits uint16
		if _, ok := menuActions.Load(id); ok {
			t.Errorf("action unexpectedly registered for item with no Action/Role (id=%#x)", id)
		}
	}
}

func TestBuildMenuPopup_DisabledItem_StillAppears(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{Title: "Paste", Disabled: true, Action: func() {}},
	}
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)

	// A disabled item is still present; only its enabled state differs.
	if got := menuItemCount(hPopup); got != 1 {
		t.Errorf("GetMenuItemCount = %d, want 1 (disabled item must appear)", got)
	}
}

// --- MenuRoleQuit ---

func TestBuildMenuPopup_RoleQuit_UserActionCalled(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	var userCalled bool
	items := []MenuItem{
		{Title: "Quit", Role: MenuRoleQuit, Action: func() { userCalled = true }},
	}

	before := menuCmdIDCounter.Load()
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)
	after := menuCmdIDCounter.Load()

	// globalPlatform is nil in unit tests so WM_CLOSE is not posted, but the
	// user-supplied action must still be called.
	for id := uint16(before); id < uint16(after); id++ { //nolint:gocritic // safe: counter range 0x1000–0xFFFF fits uint16
		if fn, ok := menuActions.Load(id); ok {
			fn.(func())()
		}
	}
	if !userCalled {
		t.Error("MenuRoleQuit: user-supplied Action was not called")
	}
}

func TestBuildMenuPopup_RoleQuit_NoUserAction_NoPanic(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{Title: "Quit", Role: MenuRoleQuit}, // no Action
	}

	before := menuCmdIDCounter.Load()
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)
	after := menuCmdIDCounter.Load()

	// Invoking the registered closure with nil globalPlatform must not panic.
	for id := uint16(before); id < uint16(after); id++ { //nolint:gocritic // safe: counter range 0x1000–0xFFFF fits uint16
		if fn, ok := menuActions.Load(id); ok {
			fn.(func())()
		}
	}
}

// TestBuildMenuPopup_RoleQuit_Registered verifies that a MenuRoleQuit item
// always registers a closure (even with no user Action), so WM_CLOSE will be
// posted when the user selects Quit.
func TestBuildMenuPopup_RoleQuit_Registered(t *testing.T) {
	t.Cleanup(func() { clearMenuActions(t) })

	items := []MenuItem{
		{Title: "Quit", Role: MenuRoleQuit},
	}

	before := menuCmdIDCounter.Load()
	hPopup := buildMenuPopup(items)
	if hPopup == 0 {
		t.Fatal("buildMenuPopup: returned NULL HMENU")
	}
	defer procDestroyMenu.Call(hPopup)
	after := menuCmdIDCounter.Load()

	found := false
	for id := uint16(before); id < uint16(after); id++ { //nolint:gocritic // safe: counter range 0x1000–0xFFFF fits uint16
		if _, ok := menuActions.Load(id); ok {
			found = true
		}
	}
	if !found {
		t.Error("MenuRoleQuit: no action registered; WM_CLOSE would never be posted")
	}
}

// TestSetApplicationMenu_NilPrimary_NoPanic verifies that SetApplicationMenu
// is safe to call before a window is created (p.primary == nil).
func TestSetApplicationMenu_NilPrimary_NoPanic(t *testing.T) {
	p := &windowsPlatform{}
	p.SetApplicationMenu([]MenuItem{{Title: "File"}})
	// Pending items must be stored even without a window.
	p.menuMu.Lock()
	n := len(p.pendingMenu)
	p.menuMu.Unlock()
	if n != 1 {
		t.Errorf("pendingMenu length = %d, want 1", n)
	}
}
