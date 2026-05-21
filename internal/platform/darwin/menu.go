//go:build darwin

package darwin

import (
	"sync"

	"github.com/go-webgpu/goffi/ffi"
)

// Menu-related selectors (initialized lazily).
var menuSels struct {
	initWithTitle                SEL
	addItem                      SEL
	setSubmenu                   SEL
	setMainMenu                  SEL
	separatorItem                SEL
	setKeyEquivalentModMask      SEL
	setServicesMenu              SEL
	initWithTitleActionKeyEquiv  SEL
	addItemWithTitleActionKey    SEL
	setWindowsMenu               SEL
	orderFrontStandardAboutPanel SEL
	orderFrontPreferencesPanel   SEL
	performClose                 SEL
	toggleFullScreen             SEL
	arrangeInFront               SEL
	hide                         SEL
	hideOtherApplications        SEL
	unhideAllApplications        SEL
	performMiniaturize           SEL
	performZoom                  SEL
	terminate                    SEL
}

func initMenuSelectors() {
	menuSels.initWithTitle = RegisterSelector("initWithTitle:")
	menuSels.addItem = RegisterSelector("addItem:")
	menuSels.setSubmenu = RegisterSelector("setSubmenu:")
	menuSels.setMainMenu = RegisterSelector("setMainMenu:")
	menuSels.separatorItem = RegisterSelector("separatorItem")
	menuSels.setKeyEquivalentModMask = RegisterSelector("setKeyEquivalentModifierMask:")
	menuSels.setServicesMenu = RegisterSelector("setServicesMenu:")
	menuSels.initWithTitleActionKeyEquiv = RegisterSelector("initWithTitle:action:keyEquivalent:")
	menuSels.addItemWithTitleActionKey = RegisterSelector("addItemWithTitle:action:keyEquivalent:")
	menuSels.setWindowsMenu = RegisterSelector("setWindowsMenu:")
	menuSels.orderFrontStandardAboutPanel = RegisterSelector("orderFrontStandardAboutPanel:")
	menuSels.orderFrontPreferencesPanel = RegisterSelector("orderFrontPreferencesPanel:")
	menuSels.performClose = RegisterSelector("performClose:")
	menuSels.toggleFullScreen = RegisterSelector("toggleFullScreen:")
	menuSels.arrangeInFront = RegisterSelector("arrangeInFront:")
	menuSels.hide = RegisterSelector("hide:")
	menuSels.hideOtherApplications = RegisterSelector("hideOtherApplications:")
	menuSels.unhideAllApplications = RegisterSelector("unhideAllApplications:")
	menuSels.performMiniaturize = RegisterSelector("performMiniaturize:")
	menuSels.performZoom = RegisterSelector("performZoom:")
	menuSels.terminate = selectors.terminate
}

// createMenuBar creates the standard macOS application menu bar.
// This enables Cmd+Q (quit), Cmd+H (hide), Cmd+M (minimize).
// Matches GLFW createMenuBar() / SDL3 Cocoa_RegisterApp() / winit menu::initialize().
// See ADR-016.
func (a *Application) createMenuBar(appName string) {
	initMenuSelectors()

	nsMenuClass := GetClass("NSMenu")
	nsMenuItemClass := GetClass("NSMenuItem")
	if nsMenuClass == 0 || nsMenuItemClass == 0 {
		return
	}

	// Create menu bar
	menuBar := ID(nsMenuClass).Send(selectors.alloc)
	menuBar = menuBar.Send(selectors.init)
	if menuBar.IsNil() {
		return
	}

	// Build and add App Menu
	appMenu := a.createAppMenu(appName, nsMenuClass, nsMenuItemClass)
	if !appMenu.IsNil() {
		appMenuItem := ID(nsMenuItemClass).Send(selectors.alloc)
		appMenuItem = appMenuItem.Send(selectors.init)
		if !appMenuItem.IsNil() {
			menuBar.SendPtr(menuSels.addItem, appMenuItem.Ptr())
			appMenuItem.SendPtr(menuSels.setSubmenu, appMenu.Ptr())
		}
	}

	// Build and add Window Menu
	windowMenu := a.createWindowMenu(nsMenuClass, nsMenuItemClass)
	if !windowMenu.IsNil() {
		windowMenuItem := ID(nsMenuItemClass).Send(selectors.alloc)
		windowMenuItem = windowMenuItem.Send(selectors.init)
		if !windowMenuItem.IsNil() {
			menuBar.SendPtr(menuSels.addItem, windowMenuItem.Ptr())
			windowMenuItem.SendPtr(menuSels.setSubmenu, windowMenu.Ptr())
		}
	}

	// Set as main menu + register window menu for automatic management
	a.nsApp.SendPtr(menuSels.setMainMenu, menuBar.Ptr())
	a.nsApp.SendPtr(menuSels.setWindowsMenu, windowMenu.Ptr())
}

// updateMenuBar updates the titles of standard menu items when the app name changes.
func (a *Application) updateMenuBar(appName string) {
	mainMenu := a.nsApp.Send(RegisterSelector("mainMenu"))
	if mainMenu.IsNil() {
		return
	}

	// The App Menu is usually at index 0
	appMenuItem := mainMenu.SendInt(RegisterSelector("itemAtIndex:"), 0)
	if appMenuItem.IsNil() {
		return
	}

	appMenu := appMenuItem.Send(RegisterSelector("submenu"))
	if appMenu.IsNil() {
		return
	}

	// Update App Menu title
	nsTitle := NewNSString(appName)
	if nsTitle != nil {
		appMenu.SendPtr(selectors.setTitle, nsTitle.ID().Ptr())
	}

	// Update items in App Menu
	itemCount := appMenu.GetInt64(RegisterSelector("numberOfItems"))
	for i := int64(0); i < itemCount; i++ {
		item := appMenu.SendInt(RegisterSelector("itemAtIndex:"), i)
		if item.IsNil() {
			continue
		}

		action := SEL(item.Send(RegisterSelector("action")).Ptr())
		title := item.Send(RegisterSelector("title"))
		if title.IsNil() {
			continue
		}

		switch action {
		case menuSels.orderFrontStandardAboutPanel:
			t := NewNSString("About " + appName)
			if t != nil {
				item.SendPtr(selectors.setTitle, t.ID().Ptr())
			}
		case RegisterSelector("hide:"):
			t := NewNSString("Hide " + appName)
			if t != nil {
				item.SendPtr(selectors.setTitle, t.ID().Ptr())
			}
		case selectors.terminate:
			t := NewNSString("Quit " + appName)
			if t != nil {
				item.SendPtr(selectors.setTitle, t.ID().Ptr())
			}
		}
	}
}

// createAppMenu builds the application (App) menu with standard items.
func (a *Application) createAppMenu(appName string, nsMenuClass, nsMenuItemClass Class) ID {
	// === App Menu ===
	appMenu := ID(nsMenuClass).Send(selectors.alloc)
	appMenuTitle := NewNSString(appName)
	if appMenuTitle != nil {
		appMenu = appMenu.SendPtr(menuSels.initWithTitle, appMenuTitle.ID().Ptr())
	} else {
		appMenu = appMenu.Send(selectors.init)
	}

	// About {appName}
	aboutTitle := NewNSString("About " + appName)
	if aboutTitle != nil {
		item := createMenuItem(nsMenuItemClass, aboutTitle.ID(), menuSels.orderFrontStandardAboutPanel, "")
		if !item.IsNil() {
			appMenu.SendPtr(menuSels.addItem, item.Ptr())
		}
	}

	// Separator after About
	sepAbout := ID(nsMenuItemClass).Send(menuSels.separatorItem)
	if !sepAbout.IsNil() {
		appMenu.SendPtr(menuSels.addItem, sepAbout.Ptr())
	}

	// Preferences… (Cmd+,)
	prefsTitle := NewNSString("Preferences…")
	if prefsTitle != nil {
		addMenuItem(nsMenuItemClass, appMenu, prefsTitle.ID(), menuSels.orderFrontPreferencesPanel, ",")
	}

	// Separator before Services
	sepPrefs := ID(nsMenuItemClass).Send(menuSels.separatorItem)
	if !sepPrefs.IsNil() {
		appMenu.SendPtr(menuSels.addItem, sepPrefs.Ptr())
	}

	// Services submenu – filled by the system automatically.
	servicesMenu := ID(nsMenuClass).Send(selectors.alloc).Send(selectors.init)
	if !servicesMenu.IsNil() {
		a.nsApp.SendPtr(menuSels.setServicesMenu, servicesMenu.Ptr())
	}

	// Separator after Services
	sepServices := ID(nsMenuItemClass).Send(menuSels.separatorItem)
	if !sepServices.IsNil() {
		appMenu.SendPtr(menuSels.addItem, sepServices.Ptr())
	}

	// "Hide {appName}" — Cmd+H
	hideTitle := NewNSString("Hide " + appName)
	if hideTitle != nil {
		addMenuItem(nsMenuItemClass, appMenu, hideTitle.ID(), RegisterSelector("hide:"), "h")
	}

	// "Hide Others" — Cmd+Opt+H
	hideOthersTitle := NewNSString("Hide Others")
	if hideOthersTitle != nil {
		item := createMenuItem(nsMenuItemClass, hideOthersTitle.ID(), RegisterSelector("hideOtherApplications:"), "h")
		if !item.IsNil() {
			// NSEventModifierFlagCommand | NSEventModifierFlagOption
			item.SendInt(menuSels.setKeyEquivalentModMask, int64(NSEventModifierFlagCommand|NSEventModifierFlagOption))
			appMenu.SendPtr(menuSels.addItem, item.Ptr())
		}
	}

	// "Show All"
	showAllTitle := NewNSString("Show All")
	if showAllTitle != nil {
		addMenuItem(nsMenuItemClass, appMenu, showAllTitle.ID(), RegisterSelector("unhideAllApplications:"), "")
	}

	// Separator
	sep := ID(nsMenuItemClass).Send(menuSels.separatorItem)
	if !sep.IsNil() {
		appMenu.SendPtr(menuSels.addItem, sep.Ptr())
	}

	// "Quit {appName}" — Cmd+Q
	quitTitle := NewNSString("Quit " + appName)
	if quitTitle != nil {
		addMenuItem(nsMenuItemClass, appMenu, quitTitle.ID(), selectors.terminate, "q")
	}

	return appMenu
}

// createWindowMenu builds the Window menu with standard items.
func (a *Application) createWindowMenu(nsMenuClass, nsMenuItemClass Class) ID {
	// === Window Menu ===
	windowMenu := ID(nsMenuClass).Send(selectors.alloc)
	windowTitle := NewNSString("Window")
	if windowTitle != nil {
		windowMenu = windowMenu.SendPtr(menuSels.initWithTitle, windowTitle.ID().Ptr())
	} else {
		windowMenu = windowMenu.Send(selectors.init)
	}

	// "Minimize" — Cmd+M
	minimizeTitle := NewNSString("Minimize")
	if minimizeTitle != nil {
		addMenuItem(nsMenuItemClass, windowMenu, minimizeTitle.ID(), RegisterSelector("performMiniaturize:"), "m")
	}

	// "Zoom"
	zoomTitle := NewNSString("Zoom")
	if zoomTitle != nil {
		addMenuItem(nsMenuItemClass, windowMenu, zoomTitle.ID(), RegisterSelector("performZoom:"), "")
	}

	// Separator before Close/Full Screen/Bring All
	sepWindow := ID(nsMenuItemClass).Send(menuSels.separatorItem)
	if !sepWindow.IsNil() {
		windowMenu.SendPtr(menuSels.addItem, sepWindow.Ptr())
	}

	// Close (Cmd+W)
	closeTitle := NewNSString("Close")
	if closeTitle != nil {
		addMenuItem(nsMenuItemClass, windowMenu, closeTitle.ID(), menuSels.performClose, "w")
	}

	// Full Screen (Ctrl+Cmd+F)
	fullScrTitle := NewNSString("Full Screen")
	if fullScrTitle != nil {
		item := createMenuItem(nsMenuItemClass, fullScrTitle.ID(), menuSels.toggleFullScreen, "f")
		if !item.IsNil() {
			item.SendInt(menuSels.setKeyEquivalentModMask,
				int64(NSEventModifierFlagCommand|NSEventModifierFlagControl))
			windowMenu.SendPtr(menuSels.addItem, item.Ptr())
		}
	}

	// Bring All to Front
	bringTitle := NewNSString("Bring All to Front")
	if bringTitle != nil {
		addMenuItem(nsMenuItemClass, windowMenu, bringTitle.ID(), menuSels.arrangeInFront, "")
	}

	return windowMenu
}

// createMenuItem creates an NSMenuItem with title, action, and key equivalent.
// Uses Send5Ptr (pre-cached CIF) for maximum performance.
func createMenuItem(nsMenuItemClass Class, title ID, action SEL, keyEquiv string) ID {
	item := ID(nsMenuItemClass).Send(selectors.alloc)
	if item.IsNil() {
		return 0
	}

	keyStr := NewNSString(keyEquiv)
	var keyPtr uintptr
	if keyStr != nil {
		keyPtr = keyStr.ID().Ptr()
	} else {
		emptyStr := NewNSString("")
		if emptyStr != nil {
			keyPtr = emptyStr.ID().Ptr()
		}
	}

	// initWithTitle:action:keyEquivalent: takes three pointer arguments.
	return item.Send5Ptr(menuSels.initWithTitleActionKeyEquiv, title.Ptr(), uintptr(action), keyPtr)
}

// addMenuItem is a convenience that creates and adds a menu item in one step.
func addMenuItem(nsMenuItemClass Class, menu ID, title ID, action SEL, keyEquiv string) {
	item := createMenuItem(nsMenuItemClass, title, action, keyEquiv)
	if !item.IsNil() {
		menu.SendPtr(menuSels.addItem, item.Ptr())
	}
}

var appDelegateClassOnce sync.Once
var menuActionMap sync.Map

func ensureAppDelegate() {
	appDelegateClassOnce.Do(func() {
		if err := initRuntime(); err != nil {
			return
		}
		nsObjectClass := GetClass("NSObject")
		cls := AllocateClassPair(nsObjectClass, "GoGPUAppDelegate")
		if cls == 0 {
			return
		}
		handleMenuItemIMP := ffi.NewCallback(func(self, sel, sender uintptr) uintptr {
			fn := getMenuItemAction(ID(sender))
			if fn != nil {
				fn()
			}
			return 0
		})
		ClassAddMethod(cls, RegisterSelector("handleMenuItem:"), handleMenuItemIMP, "v@:@")
		RegisterClassPair(cls)

		alloc := ID(cls).Send(RegisterSelector("alloc"))
		delegate := alloc.Send(RegisterSelector("init"))
		nsApp := GetClass("NSApplication").Send(RegisterSelector("sharedApplication"))
		nsApp.SendPtr(RegisterSelector("setDelegate:"), delegate.Ptr())
	})
}

func setMenuItemAction(item ID, action func()) {
	menuActionMap.Store(uintptr(item), action)
}

func getMenuItemAction(item ID) func() {
	val, ok := menuActionMap.Load(uintptr(item))
	if !ok {
		return nil
	}
	return val.(func())
}

// NewMainMenu creates an empty NSMenu, sets it as the main menu of NSApp,
// and returns its ID. Returns 0 on failure.
func NewMainMenu() ID {
	if err := initRuntime(); err != nil {
		return 0
	}
	ensureAppDelegate()

	nsMenuClass := GetClass("NSMenu")
	alloc := ID(nsMenuClass).Send(RegisterSelector("alloc"))
	if alloc.IsNil() {
		return 0
	}

	titleNS := createNSString("")
	if titleNS == 0 {
		return 0
	}
	mainMenu := alloc.SendPtr(RegisterSelector("initWithTitle:"), uintptr(titleNS))
	if mainMenu.IsNil() {
		return 0
	}

	nsApp := GetClass("NSApplication").Send(RegisterSelector("sharedApplication"))
	if nsApp.IsNil() {
		return 0
	}

	nsApp.SendPtr(menuSels.setMainMenu, mainMenu.Ptr())

	return mainMenu
}

// AddSeparatorItem adds a separator to the given menu.
func AddSeparatorItem(menu ID) {
	if menu.IsNil() {
		return
	}
	sep := GetClass("NSMenuItem").Send(RegisterSelector("separatorItem"))
	menu.SendPtr(menuSels.addItem, sep.Ptr())
}

// AddMenuItemWithCallback adds a menu item that executes the provided Go function
// when selected. keyEquivalent can be "" for no shortcut.
func AddMenuItemWithCallback(menu ID, title string, action func(), keyEquivalent string) ID {
	if menu.IsNil() {
		return 0
	}
	ensureAppDelegate()

	titleNS := createNSString(title)
	keyNS := createNSString(keyEquivalent)

	itemClass := GetClass("NSMenuItem")
	alloc := itemClass.Send(RegisterSelector("alloc"))
	selInit := RegisterSelector("initWithTitle:action:keyEquivalent:")
	item := alloc.Send5Ptr(selInit,
		uintptr(titleNS),
		uintptr(RegisterSelector("handleMenuItem:")),
		uintptr(keyNS),
	)
	if item.IsNil() {
		return 0
	}
	setMenuItemAction(item, action)
	menu.SendPtr(menuSels.addItem, item.Ptr())
	return item
}

// createNSString is a helper to create an NSString from a Go string.
func createNSString(s string) ID {
	ns := NewNSString(s)
	if ns == nil {
		return 0
	}
	return ns.ID()
}

// GetMenuSelector returns the selector for a predefined menu item role.
func (a *Application) GetMenuSelector(role string) SEL {
	initMenuSelectors()
	switch role {
	case "about":
		return menuSels.orderFrontStandardAboutPanel
	case "preferences":
		return menuSels.orderFrontPreferencesPanel
	case "services":
		return 0 // Services menu is handled specially
	case "hide":
		return menuSels.hide
	case "hideOthers":
		return menuSels.hideOtherApplications
	case "showAll":
		return menuSels.unhideAllApplications
	case "quit":
		return menuSels.terminate
	case "close":
		return menuSels.performClose
	case "minimize":
		return menuSels.performMiniaturize
	case "zoom":
		return menuSels.performZoom
	case "fullScreen":
		return menuSels.toggleFullScreen
	case "bringAllToFront":
		return menuSels.arrangeInFront
	}
	return 0
}

// AddMenuItemWithRole creates a menu item with a predefined role and returns its ID.
func AddMenuItemWithRole(menu ID, title string, role string) ID {
	initMenuSelectors()
	nsMenuItemClass := GetClass("NSMenuItem")
	if nsMenuItemClass == 0 {
		return 0
	}

	var action SEL
	var keyEquiv string
	var modMask int64

	switch role {
	case "about":
		action = menuSels.orderFrontStandardAboutPanel
	case "preferences":
		action = menuSels.orderFrontPreferencesPanel
		keyEquiv = ","
	case "services":
		// Services is usually a submenu
		return 0
	case "hide":
		action = menuSels.hide
		keyEquiv = "h"
	case "hideOthers":
		action = menuSels.hideOtherApplications
		keyEquiv = "h"
		modMask = int64(NSEventModifierFlagCommand | NSEventModifierFlagOption)
	case "showAll":
		action = menuSels.unhideAllApplications
	case "quit":
		action = menuSels.terminate
		keyEquiv = "q"
	case "close":
		action = menuSels.performClose
		keyEquiv = "w"
	case "minimize":
		action = menuSels.performMiniaturize
		keyEquiv = "m"
	case "zoom":
		action = menuSels.performZoom
	case "fullScreen":
		action = menuSels.toggleFullScreen
		keyEquiv = "f"
		modMask = int64(NSEventModifierFlagCommand | NSEventModifierFlagControl)
	case "bringAllToFront":
		action = menuSels.arrangeInFront
	}

	nsTitle := NewNSString(title)
	if nsTitle == nil {
		return 0
	}

	item := createMenuItem(nsMenuItemClass, nsTitle.ID(), action, keyEquiv)
	if !item.IsNil() {
		if modMask != 0 {
			item.SendInt(menuSels.setKeyEquivalentModMask, modMask)
		}
		menu.SendPtr(menuSels.addItem, item.Ptr())
	}
	return item
}

// NewMenuWithTitle creates a new NSMenu with the specified title.
func NewMenuWithTitle(title string) ID {
	if err := initRuntime(); err != nil {
		return 0
	}
	nsMenuClass := GetClass("NSMenu")
	alloc := ID(nsMenuClass).Send(RegisterSelector("alloc"))
	if alloc.IsNil() {
		return 0
	}
	titleNS := createNSString(title)
	if titleNS == 0 {
		return 0
	}
	return alloc.SendPtr(RegisterSelector("initWithTitle:"), uintptr(titleNS))
}

// NewMenuItemWithSubmenu creates an NSMenuItem with a title and an attached submenu.
func NewMenuItemWithSubmenu(title string, submenu ID) ID {
	if err := initRuntime(); err != nil {
		return 0
	}
	itemClass := GetClass("NSMenuItem")
	alloc := ID(itemClass).Send(RegisterSelector("alloc"))
	if alloc.IsNil() {
		return 0
	}
	titleNS := createNSString(title)
	if titleNS == 0 {
		return 0
	}
	keyNS := createNSString("")
	if keyNS == 0 {
		return 0
	}
	item := alloc.Send5Ptr(RegisterSelector("initWithTitle:action:keyEquivalent:"),
		uintptr(titleNS),
		0,
		uintptr(keyNS),
	)
	if item.IsNil() {
		return 0
	}
	item.SendPtr(RegisterSelector("setSubmenu:"), submenu.Ptr())
	return item
}
