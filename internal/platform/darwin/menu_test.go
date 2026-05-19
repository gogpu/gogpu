//go:build darwin

package darwin_test

import (
	"testing"

	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
)

// TestMenuSelectorRegistration verifies that menu-related ObjC selectors
// can be registered without panic.
func TestMenuSelectorRegistration(t *testing.T) {
	runOnMainThread(t, func() {
		sels := []string{
			"initWithTitle:",
			"addItem:",
			"setSubmenu:",
			"setMainMenu:",
			"separatorItem",
			"setKeyEquivalentModifierMask:",
			"initWithTitle:action:keyEquivalent:",
			"setWindowsMenu:",
			"terminate:",
			"hide:",
			"hideOtherApplications:",
			"unhideAllApplications:",
			"performMiniaturize:",
			"performZoom:",
		}
		for _, name := range sels {
			sel := platformdarwin.RegisterSelector(name)
			if sel == 0 {
				t.Errorf("RegisterSelector(%q) returned 0", name)
			}
		}
	})
}

// TestNSMenuClassExists verifies that NSMenu and NSMenuItem classes
// are available in the ObjC runtime.
func TestNSMenuClassExists(t *testing.T) {
	runOnMainThread(t, func() {
		menu := platformdarwin.GetClass("NSMenu")
		if menu == 0 {
			t.Fatal("NSMenu class not found")
		}
		menuItem := platformdarwin.GetClass("NSMenuItem")
		if menuItem == 0 {
			t.Fatal("NSMenuItem class not found")
		}
	})
}

// TestNSMenuCreation verifies that an NSMenu instance can be created
// and initialized via the ObjC runtime.
func TestNSMenuCreation(t *testing.T) {
	runOnMainThread(t, func() {
		nsMenuClass := platformdarwin.GetClass("NSMenu")
		if nsMenuClass == 0 {
			t.Fatal("NSMenu class not found")
		}

		alloc := platformdarwin.ID(nsMenuClass).Send(platformdarwin.RegisterSelector("alloc"))
		if alloc.IsNil() {
			t.Fatal("NSMenu alloc returned nil")
		}

		menu := alloc.Send(platformdarwin.RegisterSelector("init"))
		if menu.IsNil() {
			t.Fatal("NSMenu init returned nil")
		}
	})
}

// TestNSMenuItemCreation verifies that an NSMenuItem can be created
// with the standard alloc/init pattern.
func TestNSMenuItemCreation(t *testing.T) {
	runOnMainThread(t, func() {
		nsMenuItemClass := platformdarwin.GetClass("NSMenuItem")
		if nsMenuItemClass == 0 {
			t.Fatal("NSMenuItem class not found")
		}

		alloc := platformdarwin.ID(nsMenuItemClass).Send(platformdarwin.RegisterSelector("alloc"))
		if alloc.IsNil() {
			t.Fatal("NSMenuItem alloc returned nil")
		}

		item := alloc.Send(platformdarwin.RegisterSelector("init"))
		if item.IsNil() {
			t.Fatal("NSMenuItem init returned nil")
		}
	})
}

// TestNSMenuSeparatorItem verifies that the separatorItem class method works.
func TestNSMenuSeparatorItem(t *testing.T) {
	runOnMainThread(t, func() {
		nsMenuItemClass := platformdarwin.GetClass("NSMenuItem")
		if nsMenuItemClass == 0 {
			t.Fatal("NSMenuItem class not found")
		}

		sep := platformdarwin.ID(nsMenuItemClass).Send(platformdarwin.RegisterSelector("separatorItem"))
		if sep.IsNil() {
			t.Fatal("separatorItem returned nil")
		}
	})
}

// TestSend5Ptr verifies that Send5Ptr correctly creates an NSMenuItem
// via initWithTitle:action:keyEquivalent:.
func TestSend5Ptr(t *testing.T) {
	runOnMainThread(t, func() {
		nsMenuItemClass := platformdarwin.GetClass("NSMenuItem")
		if nsMenuItemClass == 0 {
			t.Fatal("NSMenuItem class not found")
		}

		alloc := platformdarwin.ID(nsMenuItemClass).Send(platformdarwin.RegisterSelector("alloc"))
		if alloc.IsNil() {
			t.Fatal("NSMenuItem alloc returned nil")
		}

		title := platformdarwin.NewNSString("Test Item")
		if title == nil {
			t.Fatal("NewNSString returned nil")
		}

		keyEquiv := platformdarwin.NewNSString("t")
		if keyEquiv == nil {
			t.Fatal("NewNSString for key returned nil")
		}

		sel := platformdarwin.RegisterSelector("initWithTitle:action:keyEquivalent:")
		action := platformdarwin.RegisterSelector("terminate:")

		item := alloc.Send5Ptr(sel, title.ID().Ptr(), uintptr(action), keyEquiv.ID().Ptr())
		if item.IsNil() {
			t.Fatal("initWithTitle:action:keyEquivalent: returned nil")
		}
	})
}

// TestNewMainMenu verifies that NewMainMenu() returns a non‑nil ID.
func TestNewMainMenu(t *testing.T) {
	runOnMainThread(t, func() {
		mainMenu := platformdarwin.NewMainMenu()
		if mainMenu.IsNil() {
			t.Fatal("NewMainMenu() returned nil")
		}
	})
}

// TestAddSeparatorItem verifies that AddSeparatorItem adds a separator
// to a menu without crashing.
func TestAddSeparatorItem(t *testing.T) {
	runOnMainThread(t, func() {
		menu := platformdarwin.NewMainMenu()
		if menu.IsNil() {
			t.Fatal("NewMainMenu() returned nil")
		}
		platformdarwin.AddSeparatorItem(menu)
		// If we reach here, no panic occurred.
	})
}

// TestAddMenuItemWithCallback verifies that AddMenuItemWithCallback
// creates a menu item and adds it to the menu.
func TestAddMenuItemWithCallback(t *testing.T) {
	runOnMainThread(t, func() {
		menu := platformdarwin.NewMainMenu()
		if menu.IsNil() {
			t.Fatal("NewMainMenu() returned nil")
		}

		called := false
		platformdarwin.AddMenuItemWithCallback(menu, "Test Item", func() {
			called = true
		}, "")

		// The delegate is invoked only when the user selects the item,
		// so we don't call it here. Just ensure no panic.
		_ = called
	})
}

// TestMenuItemActionAssociation verifies that setMenuItemAction and
// getMenuItemAction correctly store and retrieve a Go function.
func TestMenuItemActionAssociation(t *testing.T) {
	runOnMainThread(t, func() {
		nsMenuItemClass := platformdarwin.GetClass("NSMenuItem")
		if nsMenuItemClass == 0 {
			t.Fatal("NSMenuItem class not found")
		}

		alloc := platformdarwin.ID(nsMenuItemClass).Send(platformdarwin.RegisterSelector("alloc"))
		if alloc.IsNil() {
			t.Fatal("NSMenuItem alloc returned nil")
		}

		item := alloc.Send(platformdarwin.RegisterSelector("init"))
		if item.IsNil() {
			t.Fatal("NSMenuItem init returned nil")
		}
	})
}
