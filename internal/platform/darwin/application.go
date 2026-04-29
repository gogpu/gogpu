//go:build darwin

package darwin

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// Errors returned by Application operations.
var (
	ErrApplicationNotInitialized = errors.New("darwin: application not initialized")
	ErrApplicationAlreadyRunning = errors.New("darwin: application already running")
)

// Application manages the NSApplication lifecycle.
// There is only one NSApplication per process.
type Application struct {
	mu              sync.Mutex
	nsApp           ID
	pool            ID
	initialized     bool
	running         bool
	shouldTerminate bool
}

// global application instance
var app *Application

// GetApplication returns the shared Application instance.
// Call Init() before using other methods.
func GetApplication() *Application {
	if app == nil {
		app = &Application{}
	}
	return app
}

// Init initializes the NSApplication.
// This must be called before creating windows or processing events.
func (a *Application) Init() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.initialized {
		return nil
	}

	// Initialize runtime, selectors, and classes
	if err := initRuntime(); err != nil {
		return err
	}
	initSelectors()
	initClasses()

	// Create autorelease pool for initialization
	a.pool = classes.NSAutoreleasePool.Send(selectors.new)
	if a.pool.IsNil() {
		return errors.New("darwin: failed to create NSAutoreleasePool")
	}

	// Get shared NSApplication instance
	a.nsApp = classes.NSApplication.Send(selectors.sharedApplication)
	if a.nsApp.IsNil() {
		return errors.New("darwin: failed to get NSApplication")
	}

	// Set activation policy to regular app (with dock icon)
	a.nsApp.SendInt(selectors.setActivationPolicy, int64(NSApplicationActivationPolicyRegular))

	// Create default application menu (ADR-016: Cmd+Q, Cmd+H, Cmd+M).
	a.createMenuBar("GoGPU")

	// Finish launching (required for event processing)
	a.nsApp.Send(selectors.finishLaunching)

	// Activate the application
	a.nsApp.SendBool(selectors.activateIgnoringOtherApps, true)

	a.initialized = true
	return nil
}

// Terminate requests application termination.
// This sets a flag that can be checked with ShouldTerminate().
func (a *Application) Terminate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.shouldTerminate = true
}

// ShouldTerminate returns true if termination was requested.
func (a *Application) ShouldTerminate() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.shouldTerminate
}

// Destroy releases application resources.
func (a *Application) Destroy() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pool != 0 {
		a.pool.Send(selectors.release)
		a.pool = 0
	}

	a.initialized = false
	a.running = false
}

// PollEvents processes all pending events without blocking.
// Returns true if any events were processed.
func (a *Application) PollEvents() bool {
	if !a.initialized {
		return false
	}

	processed := false

	// Create local autorelease pool for event processing
	pool := classes.NSAutoreleasePool.Send(selectors.new)
	defer pool.Send(selectors.release)

	// Get distant past date for non-blocking poll
	distantPast := classes.NSDate.Send(selectors.distantPast)

	// Get default run loop mode string
	modeStr := NewNSString("kCFRunLoopDefaultMode")
	defer modeStr.Release()

	// Poll for events
	for {
		event := a.nextEvent(distantPast, modeStr.ID())
		if event.IsNil() {
			break
		}
		a.nsApp.SendPtr(selectors.sendEvent, event.Ptr())
		processed = true
	}

	return processed
}

// WaitEvents waits for events and processes them.
// This blocks until at least one event is available.
func (a *Application) WaitEvents() {
	if !a.initialized {
		return
	}

	// Create local autorelease pool
	pool := classes.NSAutoreleasePool.Send(selectors.new)
	defer pool.Send(selectors.release)

	// Get distant future date for blocking wait
	distantFuture := classes.NSDate.Send(selectors.distantFuture)

	// Get default run loop mode string
	modeStr := NewNSString("kCFRunLoopDefaultMode")
	defer modeStr.Release()

	// Wait for first event
	event := a.nextEvent(distantFuture, modeStr.ID())
	if !event.IsNil() {
		a.nsApp.SendPtr(selectors.sendEvent, event.Ptr())
	}

	// Process any remaining events
	a.PollEvents()
}

// nextEvent retrieves the next event from the event queue.
// date controls blocking behavior: distantPast for non-blocking, distantFuture for blocking.
func (a *Application) nextEvent(date ID, mode ID) ID {
	// Call [NSApp nextEventMatchingMask:untilDate:inMode:dequeue:]
	// This requires a special calling convention for the multi-argument method
	return a.nsApp.nextEventMatchingMask(NSEventMaskAny, date, mode, true)
}

// nextEventMatchingMask calls the Objective-C method with proper arguments.
func (id ID) nextEventMatchingMask(mask NSEventMask, date ID, mode ID, dequeue bool) ID {
	if id == 0 {
		return 0
	}

	initSelectors()

	var dequeueVal uintptr
	if dequeue {
		dequeueVal = 1
	}

	return msgSend(id, selectors.nextEventMatchingMaskUntilDateInModeDequeue,
		uintptr(mask),
		date.Ptr(),
		mode.Ptr(),
		dequeueVal,
	)
}

// PostEmptyEvent posts a synthetic NSEventTypeApplicationDefined event
// to unblock WaitEvents. This is thread-safe and can be called from any goroutine.
// It is the standard Cocoa pattern used by GLFW, winit, SDL, and Qt to wake
// the main event loop.
func (a *Application) PostEmptyEvent() {
	if !a.initialized || a.nsApp.IsNil() {
		return
	}

	// Create synthetic event:
	// [NSEvent otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:]
	event := createApplicationDefinedEvent()
	if event.IsNil() {
		return
	}

	// [NSApp postEvent:event atStart:YES]
	// postEvent:atStart: is thread-safe per Apple documentation.
	msgSend(a.nsApp, selectors.postEventAtStart, event.Ptr(), uintptr(1))
}

// createApplicationDefinedEvent creates an NSEventTypeApplicationDefined event
// using the NSEvent class method otherEventWithType:location:modifierFlags:
// timestamp:windowNumber:context:subtype:data1:data2:.
func createApplicationDefinedEvent() ID {
	if err := initRuntime(); err != nil {
		return 0
	}

	initSelectors()
	initClasses()

	// otherEventWithType: has 9 parameters (+ self + _cmd = 11 total):
	//   self:          NSEvent class pointer
	//   _cmd:          selector
	//   type:          NSUInteger (uint64) = 15 (ApplicationDefined)
	//   location:      NSPoint (struct of 2 doubles)
	//   modifierFlags: NSUInteger (uint64) = 0
	//   timestamp:     NSTimeInterval (double) = 0.0
	//   windowNumber:  NSInteger (int64) = 0
	//   context:       pointer = nil
	//   subtype:       short (int16) = 0
	//   data1:         NSInteger (int64) = 0
	//   data2:         NSInteger (int64) = 0
	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self (NSEvent class)
		types.PointerTypeDescriptor, // _cmd (SEL)
		types.UInt64TypeDescriptor,  // type (NSUInteger)
		nsPointType,                 // location (NSPoint = 2 doubles)
		types.UInt64TypeDescriptor,  // modifierFlags (NSUInteger)
		types.DoubleTypeDescriptor,  // timestamp (NSTimeInterval)
		types.SInt64TypeDescriptor,  // windowNumber (NSInteger)
		types.PointerTypeDescriptor, // context (nil)
		types.SInt16TypeDescriptor,  // subtype (short)
		types.SInt64TypeDescriptor,  // data1 (NSInteger)
		types.SInt64TypeDescriptor,  // data2 (NSInteger)
	}

	cif := &types.CallInterface{}
	err := ffi.PrepareCallInterface(
		cif,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		argTypes,
	)
	if err != nil {
		return 0
	}

	argBox := &struct {
		self          uintptr
		cmd           uintptr
		eventType     uint64
		location      NSPoint
		modifierFlags uint64
		timestamp     float64
		windowNumber  int64
		context       uintptr
		subtype       int16
		data1         int64
		data2         int64
	}{
		self:      uintptr(classes.NSEvent),
		cmd:       uintptr(selectors.otherEventWithType),
		eventType: uint64(NSEventTypeApplicationDefined),
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&argBox.self),
		unsafe.Pointer(&argBox.cmd),
		unsafe.Pointer(&argBox.eventType),
		unsafe.Pointer(&argBox.location),
		unsafe.Pointer(&argBox.modifierFlags),
		unsafe.Pointer(&argBox.timestamp),
		unsafe.Pointer(&argBox.windowNumber),
		unsafe.Pointer(&argBox.context),
		unsafe.Pointer(&argBox.subtype),
		unsafe.Pointer(&argBox.data1),
		unsafe.Pointer(&argBox.data2),
	}

	var result uintptr
	err = ffi.CallFunction(
		cif,
		objcRT.objcMsgSend,
		unsafe.Pointer(&result),
		argPtrs,
	)
	if err != nil {
		return 0
	}

	return ID(result)
}

// WaitEventsWithHandler blocks until at least one event is available,
// then processes all pending events using the provided handler.
// This combines the blocking behavior of WaitEvents with the handler
// dispatch pattern of PollEventsWithHandler.
func (a *Application) WaitEventsWithHandler(handler EventHandler) {
	if !a.initialized {
		return
	}

	// Create local autorelease pool for event processing
	pool := classes.NSAutoreleasePool.Send(selectors.new)
	defer pool.Send(selectors.release)

	// Get distant future date for blocking wait
	distantFuture := classes.NSDate.Send(selectors.distantFuture)

	// Get default run loop mode string
	modeStr := NewNSString("kCFRunLoopDefaultMode")
	defer modeStr.Release()

	// Block until first event arrives
	event := a.nextEvent(distantFuture, modeStr.ID())
	if !event.IsNil() {
		// Get event type
		eventType := NSEventType(event.GetUint64(selectors.eventType))

		// Let handler inspect the event
		shouldDispatch := true
		if handler != nil {
			shouldDispatch = handler(event, eventType)
		}

		// Dispatch to application if not consumed
		if shouldDispatch {
			a.nsApp.SendPtr(selectors.sendEvent, event.Ptr())
		}
	}

	// Process any remaining pending events without blocking
	a.PollEventsWithHandler(handler)
}

// NSApp returns the raw NSApplication ID for advanced usage.
func (a *Application) NSApp() ID {
	return a.nsApp
}

// IsInitialized returns true if the application has been initialized.
func (a *Application) IsInitialized() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.initialized
}

// NSString wraps an Objective-C NSString.
type NSString struct {
	id ID
}

// NewNSString creates an NSString from a Go string.
func NewNSString(s string) *NSString {
	initSelectors()
	initClasses()

	// Allocate NSString
	nsstr := classes.NSString.Send(selectors.alloc)
	if nsstr.IsNil() {
		return nil
	}

	// Convert Go string to C string (null-terminated)
	cstr := append([]byte(s), 0)

	// Initialize with UTF8 string
	// Pass the address of the first byte as uintptr
	nsstr = nsstr.SendPtr(selectors.initWithUTF8String, bytesPtr(cstr))

	return &NSString{id: nsstr}
}

// ID returns the underlying Objective-C object ID.
func (s *NSString) ID() ID {
	if s == nil {
		return 0
	}
	return s.id
}

// Release releases the NSString.
func (s *NSString) Release() {
	if s != nil && s.id != 0 {
		s.id.Send(selectors.release)
		s.id = 0
	}
}

// String returns the Go string representation.
// Note: This requires reading from the NSString's UTF8String pointer,
// which is more complex than shown here.
func (s *NSString) String() string {
	// Simplified: return empty string
	// A full implementation would call UTF8String and read the C string
	return ""
}

// bytesPtr returns a uintptr to the first element of the byte slice.
// The caller must ensure the slice remains valid during use.
func bytesPtr(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&b[0]))
}
