package gogpu

import (
	"fmt"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
	"github.com/gogpu/gpucontext"
)

// mockMenuManager implements platform.PlatMenuManager
type mockMenuManager struct {
	setApplicationMenuCalled bool
	setApplicationMenuItems  []platform.MenuItem

	addToSystemMenuCalled bool
	addToSystemMenuMenu   platform.SystemMenu
	addToSystemMenuItems  []platform.MenuItem
	addToSystemMenuResult bool
}

func (m *mockMenuManager) SetApplicationMenu(items []platform.MenuItem) {
	m.setApplicationMenuCalled = true
	m.setApplicationMenuItems = items
}

func (m *mockMenuManager) AddToSystemMenu(menu platform.SystemMenu, items []platform.MenuItem) bool {
	m.addToSystemMenuCalled = true
	m.addToSystemMenuMenu = menu
	m.addToSystemMenuItems = items
	return m.addToSystemMenuResult
}

// Ensure mockMenuManager implements platform.PlatformManager (partially)
// and PlatMenuManager.
func (m *mockMenuManager) Init() error { return nil }
func (m *mockMenuManager) Destroy()    {}
func (m *mockMenuManager) CreateWindow(platform.Config) (platform.PlatformWindow, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockMenuManager) PollEvents() platform.Event                { return platform.Event{} }
func (m *mockMenuManager) WaitEvents()                               {}
func (m *mockMenuManager) WakeUp()                                   {}
func (m *mockMenuManager) ClipboardRead() (string, error)            { return "", nil }
func (m *mockMenuManager) ClipboardWrite(string) error               { return nil }
func (m *mockMenuManager) DarkMode() bool                            { return false }
func (m *mockMenuManager) ReduceMotion() bool                        { return false }
func (m *mockMenuManager) HighContrast() bool                        { return false }
func (m *mockMenuManager) FontScale() float32                        { return 1.0 }
func (m *mockMenuManager) SubpixelLayout() gpucontext.SubpixelLayout { return 0 }
func (m *mockMenuManager) SetAppName(string)                         {}

// TestNewMenu checks that NewMenu is not nil and that the menu is empty.
func TestNewMenu(t *testing.T) {
	menu := NewMenu("Test")
	if menu == nil {
		t.Fatal("NewMenu returned nil")
	}
	if menu.Title != "Test" {
		t.Errorf("expected title Test, got %q", menu.Title)
	}
	if len(menu.Items) != 0 {
		t.Fatalf("expected empty menu, got %d items", len(menu.Items))
	}
}

// TestAddItem checks for the addition of elements and chained returns.
func TestAddItem(t *testing.T) {
	menu := NewMenu("")
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

func TestCustomMenus(t *testing.T) {
	mgr := &mockMenuManager{}
	app := &App{
		manager: mgr,
	}

	// Initial main menu
	mainMenu := NewMenu("").AddItem(MenuItem{Title: "File"})
	app.SetMenu(mainMenu)

	if !mgr.setApplicationMenuCalled {
		t.Fatal("SetApplicationMenu was not called")
	}
	if len(mgr.setApplicationMenuItems) != 1 {
		t.Fatalf("expected 1 item, got %d", len(mgr.setApplicationMenuItems))
	}
	if mgr.setApplicationMenuItems[0].Title != "File" {
		t.Errorf("expected File, got %q", mgr.setApplicationMenuItems[0].Title)
	}

	// Add custom menu
	mgr.setApplicationMenuCalled = false
	customMenu := NewMenu("Tools").AddItem(MenuItem{Title: "Setting 1"})
	app.AddCustomMenu("tools", customMenu)

	if !mgr.setApplicationMenuCalled {
		t.Fatal("SetApplicationMenu was not called after AddCustomMenu")
	}
	// Items should be combined: "File" from main, "Tools" as a submenu
	if len(mgr.setApplicationMenuItems) != 2 {
		t.Fatalf("expected 2 items, got %d", len(mgr.setApplicationMenuItems))
	}
	if mgr.setApplicationMenuItems[1].Title != "Tools" {
		t.Errorf("expected Tools, got %q", mgr.setApplicationMenuItems[1].Title)
	}
	if len(mgr.setApplicationMenuItems[1].Submenu) != 1 {
		t.Fatalf("expected 1 submenu item, got %d", len(mgr.setApplicationMenuItems[1].Submenu))
	}

	// Update custom menu
	mgr.setApplicationMenuCalled = false
	customMenu.AddItem(MenuItem{Title: "Setting 2"})
	app.UpdateCustomMenu("tools", customMenu)

	if !mgr.setApplicationMenuCalled {
		t.Fatal("SetApplicationMenu was not called after UpdateCustomMenu")
	}
	// Now should have 2 top-level items, but the second one has 2 items in submenu
	if len(mgr.setApplicationMenuItems) != 2 {
		t.Fatalf("expected 2 items, got %d", len(mgr.setApplicationMenuItems))
	}
	if len(mgr.setApplicationMenuItems[1].Submenu) != 2 {
		t.Fatalf("expected 2 submenu items, got %d", len(mgr.setApplicationMenuItems[1].Submenu))
	}
}
