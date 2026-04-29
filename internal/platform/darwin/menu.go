//go:build darwin

package darwin

import (
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// Menu-related selectors (initialized lazily).
var menuSels struct {
	initWithTitle               SEL
	addItem                     SEL
	setSubmenu                  SEL
	setMainMenu                 SEL
	separatorItem               SEL
	setKeyEquivalentModMask     SEL
	setServicesMenu             SEL
	initWithTitleActionKeyEquiv SEL
	addItemWithTitleActionKey   SEL
	setWindowsMenu              SEL
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

	// === App Menu ===
	appMenu := ID(nsMenuClass).Send(selectors.alloc)
	appMenuTitle := NewNSString(appName)
	if appMenuTitle != nil {
		appMenu = appMenu.SendPtr(menuSels.initWithTitle, appMenuTitle.ID().Ptr())
	} else {
		appMenu = appMenu.Send(selectors.init)
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

	// Add app menu to menu bar
	appMenuItem := ID(nsMenuItemClass).Send(selectors.alloc)
	appMenuItem = appMenuItem.Send(selectors.init)
	if !appMenuItem.IsNil() {
		menuBar.SendPtr(menuSels.addItem, appMenuItem.Ptr())
		appMenuItem.SendPtr(menuSels.setSubmenu, appMenu.Ptr())
	}

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

	// Add window menu to menu bar
	windowMenuItem := ID(nsMenuItemClass).Send(selectors.alloc)
	windowMenuItem = windowMenuItem.Send(selectors.init)
	if !windowMenuItem.IsNil() {
		menuBar.SendPtr(menuSels.addItem, windowMenuItem.Ptr())
		windowMenuItem.SendPtr(menuSels.setSubmenu, windowMenu.Ptr())
	}

	// Set as main menu + register window menu for automatic management
	a.nsApp.SendPtr(menuSels.setMainMenu, menuBar.Ptr())
	a.nsApp.SendPtr(menuSels.setWindowsMenu, windowMenu.Ptr())
}

// createMenuItem creates an NSMenuItem with title, action, and key equivalent.
// Uses objc_msgSend with 3 pointer args for initWithTitle:action:keyEquivalent:.
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

	return MsgSend3Ptr(item, menuSels.initWithTitleActionKeyEquiv, title.Ptr(), uintptr(action), keyPtr)
}

// addMenuItem is a convenience that creates and adds a menu item in one step.
func addMenuItem(nsMenuItemClass Class, menu ID, title ID, action SEL, keyEquiv string) {
	item := createMenuItem(nsMenuItemClass, title, action, keyEquiv)
	if !item.IsNil() {
		menu.SendPtr(menuSels.addItem, item.Ptr())
	}
}

// MsgSend3Ptr calls objc_msgSend with self, sel, and 3 pointer arguments.
func MsgSend3Ptr(id ID, sel SEL, arg0, arg1, arg2 uintptr) ID {
	if err := initRuntime(); err != nil {
		return 0
	}

	cif := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(cif, types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // self
			types.PointerTypeDescriptor, // _cmd
			types.PointerTypeDescriptor, // arg0
			types.PointerTypeDescriptor, // arg1
			types.PointerTypeDescriptor, // arg2
		},
	); err != nil {
		return 0
	}

	self := uintptr(id)
	cmd := uintptr(sel)

	var result uintptr
	if err := ffi.CallFunction(cif,
		objcRT.objcMsgSend,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{
			unsafe.Pointer(&self),
			unsafe.Pointer(&cmd),
			unsafe.Pointer(&arg0),
			unsafe.Pointer(&arg1),
			unsafe.Pointer(&arg2),
		},
	); err != nil {
		return 0
	}
	return ID(result)
}
