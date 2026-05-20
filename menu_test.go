package gogpu

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

// mockMenuManager implements platform.PlatMenuManager
type mockMenuManager struct {
	addToSystemMenuCalled bool
	addToSystemMenuMenu   platform.SystemMenu
	addToSystemMenuItems  []platform.MenuItem
	addToSystemMenuResult bool
}

func (m *mockMenuManager) SetApplicationMenu(items []platform.MenuItem) {
	// Not needed for these tests
}

func (m *mockMenuManager) AddToSystemMenu(menu platform.SystemMenu, items []platform.MenuItem) bool {
	m.addToSystemMenuCalled = true
	m.addToSystemMenuMenu = menu
	m.addToSystemMenuItems = items
	return m.addToSystemMenuResult
}

// TestNewMenu checks that NewMenu is not nil and that the menu is empty.
func TestNewMenu(t *testing.T) {
	menu := NewMenu()
	if menu == nil {
		t.Fatal("NewMenu returned nil")
	}
	if len(menu.Items) != 0 {
		t.Fatalf("expected empty menu, got %d items", len(menu.Items))
	}
}

// TestAddItem checks for the addition of elements and chained returns.
func TestAddItem(t *testing.T) {
	menu := NewMenu()
	result := menu.AddItem(MenuItem{Title: "Item 1"})
	if result != menu {
		t.Error("AddItem did not return the same menu for chaining")
	}
	if len(menu.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(menu.Items))
	}
	if menu.Items[0].Title != "Item 1" {
		t.Errorf("unexpected title: %q", menu.Items[0].Title)
	}

	menu.AddItem(MenuItem{Separator: true}).AddItem(MenuItem{Title: "Item 2", Action: func() {}})
	if len(menu.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(menu.Items))
	}
	if !menu.Items[1].Separator {
		t.Error("expected separator item")
	}
	if menu.Items[2].Action == nil {
		t.Error("expected action to be set")
	}
}

// TestSystemMenuHandleWithManager checks for addition via PlatformMenuManager.
func TestSystemMenuHandleWithManager(t *testing.T) {
	mgr := &mockMenuManager{}
	handle := &SystemMenuHandle{
		manager: mgr,
		menu:    platform.SystemMenuApplication,
	}

	mgr.addToSystemMenuResult = true
	ok := handle.AddItem(MenuItem{
		Title:     "Test",
		Action:    func() {},
		Disabled:  true,
		Separator: false,
	})
	if !ok {
		t.Error("expected true when manager returns true")
	}
	if !mgr.addToSystemMenuCalled {
		t.Fatal("AddToSystemMenu was not called")
	}
	if mgr.addToSystemMenuMenu != platform.SystemMenuApplication {
		t.Errorf("unexpected menu: %v", mgr.addToSystemMenuMenu)
	}
	if len(mgr.addToSystemMenuItems) != 1 {
		t.Fatalf("expected 1 item, got %d", len(mgr.addToSystemMenuItems))
	}
	item := mgr.addToSystemMenuItems[0]
	if item.Title != "Test" || item.Disabled != true || item.Separator != false {
		t.Errorf("item fields mismatch: %+v", item)
	}
	if item.Action == nil {
		t.Error("action should not be nil")
	}

	mgr.addToSystemMenuCalled = false
	mgr.addToSystemMenuResult = false
	ok = handle.AddItem(MenuItem{Title: "Fail"})
	if ok {
		t.Error("expected false when manager returns false")
	}
	if !mgr.addToSystemMenuCalled {
		t.Error("expected AddToSystemMenu to be called even if returning false")
	}
}

// TestSystemMenuHandlePending checks for pending additions via App.
func TestSystemMenuHandlePending(t *testing.T) {
	app := &App{
		pendingSystemMenuItems: make(map[SystemMenu][]MenuItem),
	}
	handle := &SystemMenuHandle{
		app:  app,
		menu: platform.SystemMenuWindow,
	}

	ok := handle.AddItem(MenuItem{
		Title:     "Pending",
		Separator: true,
	})
	if !ok {
		t.Error("expected true for pending addition")
	}
	if len(app.pendingSystemMenuItems) != 1 {
		t.Fatalf("expected 1 pending menu, got %d", len(app.pendingSystemMenuItems))
	}
	items, exists := app.pendingSystemMenuItems[SystemMenuWindow]
	if !exists {
		t.Fatal("expected pending items for SystemMenuWindow")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pending item, got %d", len(items))
	}
	if items[0].Title != "Pending" || !items[0].Separator {
		t.Errorf("pending item mismatch: %+v", items[0])
	}
}

// TestSystemMenuHandleNoApp checks whether false is returned when neither a manager nor an application is present.
func TestSystemMenuHandleNoApp(t *testing.T) {
	handle := &SystemMenuHandle{
		app: &App{},
	}
	ok := handle.AddItem(MenuItem{Title: "Noop"})
	if ok {
		t.Error("expected false when pendingSystemMenuItems is nil")
	}

	handle = &SystemMenuHandle{
		// app = nil, manager = nil
	}
	ok = handle.AddItem(MenuItem{Title: "Noop"})
	if ok {
		t.Error("expected false when both manager and app are nil")
	}
}
