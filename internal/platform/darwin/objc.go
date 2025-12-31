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

	// Protect shared CIF usage; CallInterface is not concurrency-safe.
	cifMu sync.Mutex
}

// objcRT is the global Objective-C runtime state.
// Named to avoid conflict with the standard library "runtime" package.
var objcRT objcRuntime

var (
	nsRectType = &types.TypeDescriptor{
		Size:      32,
		Alignment: 8,
		Kind:      types.StructType,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}
	nsSizeType = &types.TypeDescriptor{
		Size:      16,
		Alignment: 8,
		Kind:      types.StructType,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}
)

// initRuntime initializes the Objective-C runtime by loading required libraries
// and resolving function symbols. This is called once on first use.
func initRuntime() error {
	objcRT.once.Do(func() {
		objcRT.err = loadRuntime()
	})
	return objcRT.err
}

// loadRuntime loads all required macOS libraries and resolves symbols.
func loadRuntime() error {
	var err error

	// Load libobjc.A.dylib (Objective-C runtime)
	objcRT.libobjc, err = ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load Foundation framework
	objcRT.foundation, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/Foundation.framework/Foundation")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load AppKit framework
	objcRT.appKit, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/AppKit.framework/AppKit")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load QuartzCore framework (for CAMetalLayer)
	objcRT.quartzCore, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/QuartzCore.framework/QuartzCore")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Load CoreFoundation framework
	objcRT.coreFoundation, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	// Resolve objc_getClass
	objcRT.objcGetClass, err = ffi.GetSymbol(objcRT.libobjc, "objc_getClass")
	if err != nil {
		return errors.Join(ErrSymbolNotFound, err)
	}

	// Resolve objc_msgSend
	objcRT.objcMsgSend, err = ffi.GetSymbol(objcRT.libobjc, "objc_msgSend")
	if err != nil {
		return errors.Join(ErrSymbolNotFound, err)
	}

	// Resolve objc_msgSend_fpret (for floating point returns)
	objcRT.objcMsgSendFpret, err = ffi.GetSymbol(objcRT.libobjc, "objc_msgSend_fpret")
	if err != nil {
		// Some platforms may not have this, fall back to objc_msgSend
		objcRT.objcMsgSendFpret = objcRT.objcMsgSend
	}

	// Resolve objc_msgSend_stret (for struct returns)
	objcRT.objcMsgSendStret, err = ffi.GetSymbol(objcRT.libobjc, "objc_msgSend_stret")
	if err != nil {
		// ARM64 doesn't use stret, fall back to objc_msgSend
		objcRT.objcMsgSendStret = objcRT.objcMsgSend
	}

	// Resolve sel_registerName
	objcRT.selRegisterName, err = ffi.GetSymbol(objcRT.libobjc, "sel_registerName")
	if err != nil {
		return errors.Join(ErrSymbolNotFound, err)
	}

	// Prepare reusable call interfaces
	objcRT.cifVoidPtr = &types.CallInterface{}
	objcRT.cifFpret = &types.CallInterface{}
	objcRT.cifSelector = &types.CallInterface{}

	// CIF for generic pointer-returning calls (2 args: self, _cmd)
	err = ffi.PrepareCallInterface(
		objcRT.cifVoidPtr,
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
		objcRT.cifSelector,
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
	argBox := &struct {
		name unsafe.Pointer
	}{
		name: namePtr,
	}

	err := objcCallLocked(
		objcRT.cifSelector,
		objcRT.objcGetClass,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&argBox.name)},
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
	argBox := &struct {
		name unsafe.Pointer
	}{
		name: namePtr,
	}

	err := objcCallLocked(
		objcRT.cifSelector,
		objcRT.selRegisterName,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&argBox.name)},
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

	initSelectors()
	switch sel {
	case selectors.frame,
		selectors.bounds,
		selectors.visibleFrame,
		selectors.drawableSize,
		selectors.contentRectForFrameRect,
		selectors.frameRectForContentRect:
		panic("objc: Send used with struct-return selector")
	}

	var result uintptr
	argBox := &struct {
		self uintptr
		cmd  uintptr
	}{
		self: uintptr(id),
		cmd:  uintptr(sel),
	}

	err := objcCallLocked(
		objcRT.cifVoidPtr,
		objcRT.objcMsgSend,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{
			unsafe.Pointer(&argBox.self),
			unsafe.Pointer(&argBox.cmd),
		},
	)
	if err != nil {
		return 0
	}

	ret := ID(result)
	return ret
}

func objcCallLocked(cif *types.CallInterface, fn unsafe.Pointer, rvalue unsafe.Pointer, avalue []unsafe.Pointer) error {
	// Add mutex if the CIF is shared
	return ffi.CallFunction(cif, fn, rvalue, avalue)
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

	initSelectors()
	if len(args) > 6 {
		panic("objc: msgSend stack args unsupported")
	}
	switch sel {
	case selectors.setDrawableSize,
		selectors.setContentsScale,
		selectors.setFrame,
		selectors.setBounds,
		selectors.initWithContentRectStyleMaskBackingDefer,
		selectors.contentRectForFrameRect,
		selectors.frameRectForContentRect:
		panic("objc: msgSend requires typed args")
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

	// Build argument pointers: self, sel, then user args. Use a heap-backed slice.
	argVals := make([]uintptr, 2+len(args))
	argVals[0] = uintptr(self)
	argVals[1] = uintptr(sel)
	copy(argVals[2:], args)

	argPtrs := make([]unsafe.Pointer, 2+len(args))
	for i := range argVals {
		argPtrs[i] = unsafe.Pointer(&argVals[i])
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

	ret := ID(result)
	return ret
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

// SendUintUint sends a message with two uint64 arguments.
func (id ID) SendUintUint(sel SEL, arg0, arg1 uint64) ID {
	return msgSend(id, sel, uintptr(arg0), uintptr(arg1))
}

// SendDouble sends a message with one double argument.
func (id ID) SendDouble(sel SEL, arg float64) ID {
	if id == 0 || sel == 0 {
		return 0
	}

	if err := initRuntime(); err != nil {
		return 0
	}

	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
		types.DoubleTypeDescriptor,  // arg
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
		self uintptr
		sel  uintptr
		arg  float64
	}{
		self: uintptr(id),
		sel:  uintptr(sel),
		arg:  arg,
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&argBox.self),
		unsafe.Pointer(&argBox.sel),
		unsafe.Pointer(&argBox.arg),
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

	ret := ID(result)
	return ret
}

// SendDoubleDouble sends a message with two double arguments.
func (id ID) SendDoubleDouble(sel SEL, arg0, arg1 float64) ID {
	if id == 0 || sel == 0 {
		return 0
	}

	if err := initRuntime(); err != nil {
		return 0
	}

	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
		types.DoubleTypeDescriptor,  // arg0
		types.DoubleTypeDescriptor,  // arg1
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
		self uintptr
		sel  uintptr
		arg0 float64
		arg1 float64
	}{
		self: uintptr(id),
		sel:  uintptr(sel),
		arg0: arg0,
		arg1: arg1,
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&argBox.self),
		unsafe.Pointer(&argBox.sel),
		unsafe.Pointer(&argBox.arg0),
		unsafe.Pointer(&argBox.arg1),
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

	ret := ID(result)
	return ret
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

	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
		nsRectType,                  // rect
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
		self uintptr
		sel  uintptr
		rect NSRect
	}{
		self: uintptr(id),
		sel:  uintptr(sel),
		rect: rect,
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&argBox.self),
		unsafe.Pointer(&argBox.sel),
		unsafe.Pointer(&argBox.rect),
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

	ret := ID(result)
	return ret
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

	// Arguments: self, _cmd, rect, styleMask, backing, defer
	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
		nsRectType,                  // rect
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

	var deferVal uint8
	if deferFlag {
		deferVal = 1
	}
	argBox := &struct {
		self     uintptr
		sel      uintptr
		rect     NSRect
		style    NSUInteger
		backing  NSBackingStoreType
		deferVal uint8
	}{
		self:     uintptr(id),
		sel:      uintptr(sel),
		rect:     rect,
		style:    style,
		backing:  backing,
		deferVal: deferVal,
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&argBox.self),
		unsafe.Pointer(&argBox.sel),
		unsafe.Pointer(&argBox.rect),
		unsafe.Pointer(&argBox.style),
		unsafe.Pointer(&argBox.backing),
		unsafe.Pointer(&argBox.deferVal),
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

	ret := ID(result)
	return ret
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

	argTypes := []*types.TypeDescriptor{
		types.PointerTypeDescriptor, // self
		types.PointerTypeDescriptor, // _cmd
	}

	cif := &types.CallInterface{}
	err := ffi.PrepareCallInterface(
		cif,
		types.DefaultCall,
		nsRectType,
		argTypes,
	)
	if err != nil {
		return NSRect{}
	}

	argBox := &struct {
		self uintptr
		sel  uintptr
	}{
		self: uintptr(id),
		sel:  uintptr(sel),
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&argBox.self),
		unsafe.Pointer(&argBox.sel),
	}

	// Result buffer for the struct
	var result [4]float64
	err = ffi.CallFunction(
		cif,
		objcRT.objcMsgSend,
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
		nsSizeType,                  // size
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
		self uintptr
		sel  uintptr
		size NSSize
	}{
		self: uintptr(id),
		sel:  uintptr(sel),
		size: size,
	}

	argPtrs := []unsafe.Pointer{
		unsafe.Pointer(&argBox.self),
		unsafe.Pointer(&argBox.sel),
		unsafe.Pointer(&argBox.size),
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

	ret := ID(result)
	return ret
}
