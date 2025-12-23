//go:build darwin

package darwin

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// Errors returned by Objective-C runtime operations.
var (
	ErrLibraryNotLoaded = errors.New("darwin: failed to load library")
	ErrSymbolNotFound   = errors.New("darwin: symbol not found")
	ErrClassNotFound    = errors.New("darwin: class not found")
	ErrSendFailed       = errors.New("darwin: objc_msgSend failed")
)

// ID represents an Objective-C object pointer.
// It wraps uintptr for type safety when working with objc objects.
type ID uintptr

// Class represents an Objective-C class pointer.
type Class uintptr

// SEL represents an Objective-C selector (method name).
type SEL uintptr

// objcRuntime holds the loaded Objective-C runtime library and function pointers.
type objcRuntime struct {
	once sync.Once
	err  error

	// Library handles
	libobjc        unsafe.Pointer
	foundation     unsafe.Pointer
	appKit         unsafe.Pointer
	quartzCore     unsafe.Pointer
	coreFoundation unsafe.Pointer

	// Function pointers
	objcGetClass     unsafe.Pointer
	objcMsgSend      unsafe.Pointer
	objcMsgSendFpret unsafe.Pointer
	objcMsgSendStret unsafe.Pointer
	selRegisterName  unsafe.Pointer

	// Call interfaces (reusable)
	cifVoidPtr  *types.CallInterface // Returns void*, takes variadic args
	cifFpret    *types.CallInterface // Returns floating point
	cifSelector *types.CallInterface // For sel_registerName
}

var runtime objcRuntime

// initRuntime initializes the Objective-C runtime by loading required libraries
// and resolving function symbols. This is called once on first use.
func initRuntime() error {
	runtime.once.Do(func() {
		runtime.err = loadRuntime()
	})
	return runtime.err
}

// loadRuntime loads all required macOS libraries and resolves symbols.
func loadRuntime() error {
	var err error

	// Load libobjc.A.dylib (Objective-C runtime)
	runtime.libobjc, err = ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load Foundation framework
	runtime.foundation, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/Foundation.framework/Foundation")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load AppKit framework
	runtime.appKit, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/AppKit.framework/AppKit")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load QuartzCore framework (for CAMetalLayer)
	runtime.quartzCore, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/QuartzCore.framework/QuartzCore")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load CoreFoundation framework
	runtime.coreFoundation, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Resolve objc_getClass
	runtime.objcGetClass, err = ffi.GetSymbol(runtime.libobjc, "objc_getClass")
	if err != nil {
		return errors.Join(ErrSymbolNotFound, err)
	}

	// Resolve objc_msgSend
	runtime.objcMsgSend, err = ffi.GetSymbol(runtime.libobjc, "objc_msgSend")
	if err != nil {
		return errors.Join(ErrSymbolNotFound, err)
	}

	// Resolve objc_msgSend_fpret (for floating point returns)
	runtime.objcMsgSendFpret, err = ffi.GetSymbol(runtime.libobjc, "objc_msgSend_fpret")
	if err != nil {
		// Some platforms may not have this, fall back to objc_msgSend
		runtime.objcMsgSendFpret = runtime.objcMsgSend
	}

	// Resolve objc_msgSend_stret (for struct returns)
	runtime.objcMsgSendStret, err = ffi.GetSymbol(runtime.libobjc, "objc_msgSend_stret")
	if err != nil {
		// ARM64 doesn't use stret, fall back to objc_msgSend
		runtime.objcMsgSendStret = runtime.objcMsgSend
	}

	// Resolve sel_registerName
	runtime.selRegisterName, err = ffi.GetSymbol(runtime.libobjc, "sel_registerName")
	if err != nil {
		return errors.Join(ErrSymbolNotFound, err)
	}

	// Prepare reusable call interfaces
	runtime.cifVoidPtr = &types.CallInterface{}
	runtime.cifFpret = &types.CallInterface{}
	runtime.cifSelector = &types.CallInterface{}

	// CIF for generic pointer-returning calls (2 args: self, _cmd)
	err = ffi.PrepareCallInterface(
		runtime.cifVoidPtr,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // self (ID)
			types.PointerTypeDescriptor, // _cmd (SEL)
		},
	)
	if err != nil {
		return err
	}

	// CIF for sel_registerName (1 arg: const char*)
	err = ffi.PrepareCallInterface(
		runtime.cifSelector,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // name
		},
	)
	if err != nil {
		return err
	}

	return nil
}

// GetClass returns the Objective-C class with the given name.
// Returns 0 if the class is not found.
func GetClass(name string) Class {
	if err := initRuntime(); err != nil {
		return 0
	}

	// Convert string to C string (null-terminated)
	cname := append([]byte(name), 0)

	var result uintptr
	namePtr := unsafe.Pointer(&cname[0])

	err := ffi.CallFunction(
		runtime.cifSelector,
		runtime.objcGetClass,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&namePtr)},
	)
	if err != nil {
		return 0
	}

	return Class(result)
}

// RegisterSelector registers a selector name and returns its SEL.
// Selectors are cached by the runtime, so calling this multiple times
// with the same name returns the same SEL.
func RegisterSelector(name string) SEL {
	if err := initRuntime(); err != nil {
		return 0
	}

	// Convert string to C string (null-terminated)
	cname := append([]byte(name), 0)

	var result uintptr
	namePtr := unsafe.Pointer(&cname[0])

	err := ffi.CallFunction(
		runtime.cifSelector,
		runtime.selRegisterName,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&namePtr)},
	)
	if err != nil {
		return 0
	}

	return SEL(result)
}

// Send sends a message to an Objective-C object and returns the result.
// This is equivalent to calling objc_msgSend(self, sel).
// For methods with arguments, use SendArgs.
func (id ID) Send(sel SEL) ID {
	if id == 0 || sel == 0 {
		return 0
	}

	if err := initRuntime(); err != nil {
		return 0
	}

	var result uintptr
	self := uintptr(id)
	cmd := uintptr(sel)

	err := ffi.CallFunction(
		runtime.cifVoidPtr,
		runtime.objcMsgSend,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{
			unsafe.Pointer(&self),
			unsafe.Pointer(&cmd),
		},
	)
	if err != nil {
		return 0
	}

	return ID(result)
}

// SendClass sends a message to a Class and returns the result.
// This is used for class methods like [NSApplication sharedApplication].
func (c Class) Send(sel SEL) ID {
	return ID(c).Send(sel)
}

// SendSuper sends a message to the superclass implementation.
// This is useful for delegate callbacks that need to call super.
func (id ID) SendSuper(sel SEL) ID {
	// For simplicity, we just call the regular Send here.
	// A full implementation would use objc_msgSendSuper.
	return id.Send(sel)
}

// IsNil returns true if the ID is nil (0).
func (id ID) IsNil() bool {
	return id == 0
}

// Ptr returns the ID as a uintptr for use with FFI.
func (id ID) Ptr() uintptr {
	return uintptr(id)
}

// ClassPtr returns the Class as a uintptr for use with FFI.
func (c Class) ClassPtr() uintptr {
	return uintptr(c)
}

// SELPtr returns the SEL as a uintptr for use with FFI.
func (s SEL) SELPtr() uintptr {
	return uintptr(s)
}

// msgSend is a low-level helper that calls objc_msgSend with arbitrary arguments.
// The arguments slice contains the values to pass after self and _cmd.
// This function creates a new CIF for each call, which is not optimal for
// performance-critical code paths. For hot paths, create a dedicated CIF.
func msgSend(self ID, sel SEL, args ...uintptr) ID {
	if self == 0 || sel == 0 {
		return 0
	}

	if err := initRuntime(); err != nil {
		return 0
	}

	// Build argument type list: self, _cmd, then user args
	argTypes := make([]*types.TypeDescriptor, 2+len(args))
	argTypes[0] = types.PointerTypeDescriptor // self
	argTypes[1] = types.PointerTypeDescriptor // _cmd
	for i := range args {
		argTypes[2+i] = types.PointerTypeDescriptor // Each arg as pointer
	}

	// Prepare CIF
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

	// Build argument pointers: self, sel, then user args
	selfPtr := uintptr(self)
	selPtr := uintptr(sel)
	argPtrs := make([]unsafe.Pointer, 2+len(args))
	argPtrs[0] = unsafe.Pointer(&selfPtr)
	argPtrs[1] = unsafe.Pointer(&selPtr)
	for i := range args {
		argPtrs[2+i] = unsafe.Pointer(&args[i])
	}

	var result uintptr
	err = ffi.CallFunction(
		cif,
		runtime.objcMsgSend,
		unsafe.Pointer(&result),
		argPtrs,
	)
	if err != nil {
		return 0
	}

	return ID(result)
}

// SendPtr sends a message with one pointer argument.
func (id ID) SendPtr(sel SEL, arg uintptr) ID {
	return msgSend(id, sel, arg)
}

// SendBool sends a message with one boolean argument.
func (id ID) SendBool(sel SEL, arg bool) ID {
	var val uintptr
	if arg {
		val = 1
	}
	return msgSend(id, sel, val)
}

// SendInt sends a message with one integer argument.
func (id ID) SendInt(sel SEL, arg int64) ID {
	return msgSend(id, sel, uintptr(arg))
}

// SendUint sends a message with one unsigned integer argument.
func (id ID) SendUint(sel SEL, arg uint64) ID {
	return msgSend(id, sel, uintptr(arg))
}

// SendRect sends a message with an NSRect argument.
// On x86_64, NSRect is passed by value in registers.
// On ARM64, it may be passed differently.
func (id ID) SendRect(sel SEL, rect NSRect) ID {
	if id == 0 || sel == 0 {
		return 0
	}

	if err := initRuntime(); err != nil {
		return 0
	}

	// NSRect consists of 4 CGFloat values (32 bytes total on 64-bit)
	// We pass them as 4 separate double arguments
	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
		types.DoubleTypeDescriptor,  // x
		types.DoubleTypeDescriptor,  // y
		types.DoubleTypeDescriptor,  // width
		types.DoubleTypeDescriptor,  // height
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

	selfPtr := uintptr(id)
	selPtr := uintptr(sel)
	x := rect.Origin.X
	y := rect.Origin.Y
	w := rect.Size.Width
	h := rect.Size.Height

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&selfPtr),
		unsafe.Pointer(&selPtr),
		unsafe.Pointer(&x),
		unsafe.Pointer(&y),
		unsafe.Pointer(&w),
		unsafe.Pointer(&h),
	}

	var result uintptr
	err = ffi.CallFunction(
		cif,
		runtime.objcMsgSend,
		unsafe.Pointer(&result),
		argPtrs,
	)
	if err != nil {
		return 0
	}

	return ID(result)
}

// SendRectUintUintBool sends a message for initWithContentRect:styleMask:backing:defer:
// This is the standard NSWindow initialization method.
func (id ID) SendRectUintUintBool(sel SEL, rect NSRect, style NSUInteger, backing NSBackingStoreType, deferFlag bool) ID {
	if id == 0 || sel == 0 {
		return 0
	}

	if err := initRuntime(); err != nil {
		return 0
	}

	// Arguments: self, _cmd, rect (4 doubles), styleMask, backing, defer
	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
		types.DoubleTypeDescriptor,  // rect.origin.x
		types.DoubleTypeDescriptor,  // rect.origin.y
		types.DoubleTypeDescriptor,  // rect.size.width
		types.DoubleTypeDescriptor,  // rect.size.height
		types.UInt64TypeDescriptor,  // styleMask
		types.UInt64TypeDescriptor,  // backing
		types.UInt8TypeDescriptor,   // defer (BOOL)
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

	selfPtr := uintptr(id)
	selPtr := uintptr(sel)
	x := rect.Origin.X
	y := rect.Origin.Y
	w := rect.Size.Width
	h := rect.Size.Height
	styleVal := style     // NSUInteger is already uint64
	backingVal := backing // NSBackingStoreType is already uint64
	var deferVal uint8
	if deferFlag {
		deferVal = 1
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&selfPtr),
		unsafe.Pointer(&selPtr),
		unsafe.Pointer(&x),
		unsafe.Pointer(&y),
		unsafe.Pointer(&w),
		unsafe.Pointer(&h),
		unsafe.Pointer(&styleVal),
		unsafe.Pointer(&backingVal),
		unsafe.Pointer(&deferVal),
	}

	var result uintptr
	err = ffi.CallFunction(
		cif,
		runtime.objcMsgSend,
		unsafe.Pointer(&result),
		argPtrs,
	)
	if err != nil {
		return 0
	}

	return ID(result)
}

// GetRect receives an NSRect return value from a method like frame.
// On x86_64, small structs may be returned in registers.
// On ARM64, the behavior differs.
func (id ID) GetRect(sel SEL) NSRect {
	if id == 0 || sel == 0 {
		return NSRect{}
	}

	if err := initRuntime(); err != nil {
		return NSRect{}
	}

	// For struct returns, we use a struct type descriptor
	// NSRect is: { CGPoint origin { CGFloat x, y }, CGSize size { CGFloat width, height } }
	// Which flattens to: { double x, double y, double width, double height }
	// Total 32 bytes

	// Create a struct type for NSRect manually
	// TypeDescriptor with StructType kind and Members
	rectType := &types.TypeDescriptor{
		Size:      32, // 4 * 8 bytes (4 doubles)
		Alignment: 8,  // double alignment
		Kind:      types.StructType,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}

	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
	}

	cif := &types.CallInterface{}
	err := ffi.PrepareCallInterface(
		cif,
		types.DefaultCall,
		rectType,
		argTypes,
	)
	if err != nil {
		return NSRect{}
	}

	selfPtr := uintptr(id)
	selPtr := uintptr(sel)

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&selfPtr),
		unsafe.Pointer(&selPtr),
	}

	// Result buffer for the struct
	var result [4]float64
	err = ffi.CallFunction(
		cif,
		runtime.objcMsgSend,
		unsafe.Pointer(&result),
		argPtrs,
	)
	if err != nil {
		return NSRect{}
	}

	return NSRect{
		Origin: NSPoint{X: result[0], Y: result[1]},
		Size:   NSSize{Width: result[2], Height: result[3]},
	}
}

// SendSize sends a message with an NSSize argument.
func (id ID) SendSize(sel SEL, size NSSize) ID {
	if id == 0 || sel == 0 {
		return 0
	}

	if err := initRuntime(); err != nil {
		return 0
	}

	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
		types.DoubleTypeDescriptor,  // width
		types.DoubleTypeDescriptor,  // height
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

	selfPtr := uintptr(id)
	selPtr := uintptr(sel)
	w := size.Width
	h := size.Height

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&selfPtr),
		unsafe.Pointer(&selPtr),
		unsafe.Pointer(&w),
		unsafe.Pointer(&h),
	}

	var result uintptr
	err = ffi.CallFunction(
		cif,
		runtime.objcMsgSend,
		unsafe.Pointer(&result),
		argPtrs,
	)
	if err != nil {
		return 0
	}

	return ID(result)
}
