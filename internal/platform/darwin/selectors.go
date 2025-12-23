//go:build darwin

package darwin

import "sync"

// selectors holds cached selector values.
// Selectors are registered once and reused throughout the application lifetime.
var selectors struct {
	once sync.Once

	// NSObject - Memory management
	alloc   SEL
	init    SEL
	new     SEL
	release SEL
	retain  SEL

	// NSApplication - Application lifecycle
	sharedApplication                           SEL
	setActivationPolicy                         SEL
	activateIgnoringOtherApps                   SEL
	run                                         SEL
	stop                                        SEL
	terminate                                   SEL
	nextEventMatchingMaskUntilDateInModeDequeue SEL
	sendEvent                                   SEL
	finishLaunching                             SEL

	// NSApplication delegate
	setDelegate SEL

	// NSWindow - Window management
	initWithContentRectStyleMaskBackingDefer SEL
	setTitle                                 SEL
	title                                    SEL
	setContentView                           SEL
	contentView                              SEL
	makeKeyAndOrderFront                     SEL
	orderOut                                 SEL
	close                                    SEL
	miniaturize                              SEL
	deminiaturize                            SEL
	zoom                                     SEL
	setFrame                                 SEL
	frame                                    SEL
	contentRectForFrameRect                  SEL
	frameRectForContentRect                  SEL
	styleMask                                SEL
	setStyleMask                             SEL
	setAcceptsMouseMovedEvents               SEL
	makeFirstResponder                       SEL
	isKeyWindow                              SEL
	isVisible                                SEL
	isMiniaturized                           SEL
	isZoomed                                 SEL
	setReleasedWhenClosed                    SEL
	center                                   SEL

	// NSView - View management
	setWantsLayer   SEL
	wantsLayer      SEL
	setLayer        SEL
	layer           SEL
	bounds          SEL
	setBounds       SEL
	setNeedsDisplay SEL

	// NSScreen
	mainScreen   SEL
	screens      SEL
	visibleFrame SEL

	// NSDate
	distantPast   SEL
	distantFuture SEL

	// NSString
	initWithUTF8String SEL
	UTF8String         SEL
	length             SEL

	// NSAutoreleasePool
	drain SEL

	// CALayer / CAMetalLayer
	setContentsScale        SEL
	contentsScale           SEL
	setDrawableSize         SEL
	drawableSize            SEL
	setDevice               SEL
	device                  SEL
	setPixelFormat          SEL
	pixelFormat             SEL
	nextDrawable            SEL
	setFramebufferOnly      SEL
	setMaximumDrawableCount SEL
	setDisplaySyncEnabled   SEL

	// NSEvent
	eventType                   SEL
	locationInWindow            SEL
	modifierFlags               SEL
	keyCode                     SEL
	characters                  SEL
	charactersIgnoringModifiers SEL
	isARepeat                   SEL
	buttonNumber                SEL
	scrollingDeltaX             SEL
	scrollingDeltaY             SEL
	hasPreciseScrollingDeltas   SEL

	// NSNotificationCenter
	defaultCenter                 SEL
	addObserverSelectorNameObject SEL
	removeObserver                SEL

	// NSRunLoop
	currentRunLoop SEL
	runMode        SEL
}

// classes holds cached class references.
var classes struct {
	once sync.Once

	NSObject             Class
	NSApplication        Class
	NSWindow             Class
	NSView               Class
	NSScreen             Class
	NSDate               Class
	NSString             Class
	NSAutoreleasePool    Class
	NSEvent              Class
	NSNotificationCenter Class
	NSRunLoop            Class
	CALayer              Class
	CAMetalLayer         Class
}

// initSelectors registers all selectors used by the darwin package.
func initSelectors() {
	selectors.once.Do(func() {
		// NSObject
		selectors.alloc = RegisterSelector("alloc")
		selectors.init = RegisterSelector("init")
		selectors.new = RegisterSelector("new")
		selectors.release = RegisterSelector("release")
		selectors.retain = RegisterSelector("retain")

		// NSApplication
		selectors.sharedApplication = RegisterSelector("sharedApplication")
		selectors.setActivationPolicy = RegisterSelector("setActivationPolicy:")
		selectors.activateIgnoringOtherApps = RegisterSelector("activateIgnoringOtherApps:")
		selectors.run = RegisterSelector("run")
		selectors.stop = RegisterSelector("stop:")
		selectors.terminate = RegisterSelector("terminate:")
		selectors.nextEventMatchingMaskUntilDateInModeDequeue = RegisterSelector(
			"nextEventMatchingMask:untilDate:inMode:dequeue:")
		selectors.sendEvent = RegisterSelector("sendEvent:")
		selectors.finishLaunching = RegisterSelector("finishLaunching")

		// NSApplication delegate
		selectors.setDelegate = RegisterSelector("setDelegate:")

		// NSWindow
		selectors.initWithContentRectStyleMaskBackingDefer = RegisterSelector(
			"initWithContentRect:styleMask:backing:defer:")
		selectors.setTitle = RegisterSelector("setTitle:")
		selectors.title = RegisterSelector("title")
		selectors.setContentView = RegisterSelector("setContentView:")
		selectors.contentView = RegisterSelector("contentView")
		selectors.makeKeyAndOrderFront = RegisterSelector("makeKeyAndOrderFront:")
		selectors.orderOut = RegisterSelector("orderOut:")
		selectors.close = RegisterSelector("close")
		selectors.miniaturize = RegisterSelector("miniaturize:")
		selectors.deminiaturize = RegisterSelector("deminiaturize:")
		selectors.zoom = RegisterSelector("zoom:")
		selectors.setFrame = RegisterSelector("setFrame:display:")
		selectors.frame = RegisterSelector("frame")
		selectors.contentRectForFrameRect = RegisterSelector("contentRectForFrameRect:")
		selectors.frameRectForContentRect = RegisterSelector("frameRectForContentRect:")
		selectors.styleMask = RegisterSelector("styleMask")
		selectors.setStyleMask = RegisterSelector("setStyleMask:")
		selectors.setAcceptsMouseMovedEvents = RegisterSelector("setAcceptsMouseMovedEvents:")
		selectors.makeFirstResponder = RegisterSelector("makeFirstResponder:")
		selectors.isKeyWindow = RegisterSelector("isKeyWindow")
		selectors.isVisible = RegisterSelector("isVisible")
		selectors.isMiniaturized = RegisterSelector("isMiniaturized")
		selectors.isZoomed = RegisterSelector("isZoomed")
		selectors.setReleasedWhenClosed = RegisterSelector("setReleasedWhenClosed:")
		selectors.center = RegisterSelector("center")

		// NSView
		selectors.setWantsLayer = RegisterSelector("setWantsLayer:")
		selectors.wantsLayer = RegisterSelector("wantsLayer")
		selectors.setLayer = RegisterSelector("setLayer:")
		selectors.layer = RegisterSelector("layer")
		selectors.bounds = RegisterSelector("bounds")
		selectors.setBounds = RegisterSelector("setBounds:")
		selectors.setNeedsDisplay = RegisterSelector("setNeedsDisplay:")

		// NSScreen
		selectors.mainScreen = RegisterSelector("mainScreen")
		selectors.screens = RegisterSelector("screens")
		selectors.visibleFrame = RegisterSelector("visibleFrame")

		// NSDate
		selectors.distantPast = RegisterSelector("distantPast")
		selectors.distantFuture = RegisterSelector("distantFuture")

		// NSString
		selectors.initWithUTF8String = RegisterSelector("initWithUTF8String:")
		selectors.UTF8String = RegisterSelector("UTF8String")
		selectors.length = RegisterSelector("length")

		// NSAutoreleasePool
		selectors.drain = RegisterSelector("drain")

		// CALayer / CAMetalLayer
		selectors.setContentsScale = RegisterSelector("setContentsScale:")
		selectors.contentsScale = RegisterSelector("contentsScale")
		selectors.setDrawableSize = RegisterSelector("setDrawableSize:")
		selectors.drawableSize = RegisterSelector("drawableSize")
		selectors.setDevice = RegisterSelector("setDevice:")
		selectors.device = RegisterSelector("device")
		selectors.setPixelFormat = RegisterSelector("setPixelFormat:")
		selectors.pixelFormat = RegisterSelector("pixelFormat")
		selectors.nextDrawable = RegisterSelector("nextDrawable")
		selectors.setFramebufferOnly = RegisterSelector("setFramebufferOnly:")
		selectors.setMaximumDrawableCount = RegisterSelector("setMaximumDrawableCount:")
		selectors.setDisplaySyncEnabled = RegisterSelector("setDisplaySyncEnabled:")

		// NSEvent
		selectors.eventType = RegisterSelector("type")
		selectors.locationInWindow = RegisterSelector("locationInWindow")
		selectors.modifierFlags = RegisterSelector("modifierFlags")
		selectors.keyCode = RegisterSelector("keyCode")
		selectors.characters = RegisterSelector("characters")
		selectors.charactersIgnoringModifiers = RegisterSelector("charactersIgnoringModifiers")
		selectors.isARepeat = RegisterSelector("isARepeat")
		selectors.buttonNumber = RegisterSelector("buttonNumber")
		selectors.scrollingDeltaX = RegisterSelector("scrollingDeltaX")
		selectors.scrollingDeltaY = RegisterSelector("scrollingDeltaY")
		selectors.hasPreciseScrollingDeltas = RegisterSelector("hasPreciseScrollingDeltas")

		// NSNotificationCenter
		selectors.defaultCenter = RegisterSelector("defaultCenter")
		selectors.addObserverSelectorNameObject = RegisterSelector(
			"addObserver:selector:name:object:")
		selectors.removeObserver = RegisterSelector("removeObserver:")

		// NSRunLoop
		selectors.currentRunLoop = RegisterSelector("currentRunLoop")
		selectors.runMode = RegisterSelector("runMode:beforeDate:")
	})
}

// initClasses loads all class references used by the darwin package.
func initClasses() {
	classes.once.Do(func() {
		classes.NSObject = GetClass("NSObject")
		classes.NSApplication = GetClass("NSApplication")
		classes.NSWindow = GetClass("NSWindow")
		classes.NSView = GetClass("NSView")
		classes.NSScreen = GetClass("NSScreen")
		classes.NSDate = GetClass("NSDate")
		classes.NSString = GetClass("NSString")
		classes.NSAutoreleasePool = GetClass("NSAutoreleasePool")
		classes.NSEvent = GetClass("NSEvent")
		classes.NSNotificationCenter = GetClass("NSNotificationCenter")
		classes.NSRunLoop = GetClass("NSRunLoop")
		classes.CALayer = GetClass("CALayer")
		classes.CAMetalLayer = GetClass("CAMetalLayer")
	})
}

// Sel returns the cached selector for common operations.
// This provides type-safe access to selectors.
type Sel struct{}

// Selectors returns the global selector cache.
func Selectors() *Sel {
	initSelectors()
	return &Sel{}
}

// Classes initializes and returns class cache.
func Classes() {
	initClasses()
}

// Convenience accessors for selectors

// Alloc returns the alloc selector.
func (s *Sel) Alloc() SEL { return selectors.alloc }

// Init returns the init selector.
func (s *Sel) Init() SEL { return selectors.init }

// New returns the new selector.
func (s *Sel) New() SEL { return selectors.new }

// Release returns the release selector.
func (s *Sel) Release() SEL { return selectors.release }

// Retain returns the retain selector.
func (s *Sel) Retain() SEL { return selectors.retain }
