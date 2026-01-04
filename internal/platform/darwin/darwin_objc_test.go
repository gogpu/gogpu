//go:build darwin

package darwin_test

import (
	"math"
	"os"
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	platformdarwin "github.com/gogpu/gogpu/internal/platform/darwin"
)

type nsPoint struct {
	X float64
	Y float64
}

type nsSize struct {
	Width  float64
	Height float64
}

type nsRect struct {
	Origin nsPoint
	Size   nsSize
}

var (
	nsPointType = &types.TypeDescriptor{
		Kind: types.StructType,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}
	nsSizeType = &types.TypeDescriptor{
		Kind: types.StructType,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}
	nsRectType = &types.TypeDescriptor{
		Kind: types.StructType,
		Members: []*types.TypeDescriptor{
			nsPointType,
			nsSizeType,
		},
	}
)

type objcRuntime struct {
	libobjc        unsafe.Pointer
	foundation     unsafe.Pointer
	appKit         unsafe.Pointer
	quartzCore     unsafe.Pointer
	coreFoundation unsafe.Pointer

	objcGetClass    unsafe.Pointer
	selRegisterName unsafe.Pointer
	objcMsgSend     unsafe.Pointer

	cifCStringToPtr types.CallInterface
}

var (
	objcOnce     sync.Once
	objcInitErr  error
	objcRuntimeV *objcRuntime
)

var mainThreadTasks chan func()

func TestMain(m *testing.M) {
	mainThreadTasks = make(chan func())
	done := make(chan int, 1)

	runtime.LockOSThread()
	go func() {
		done <- m.Run()
		close(mainThreadTasks)
	}()

	for task := range mainThreadTasks {
		task()
	}

	os.Exit(<-done)
}

func loadObjcRuntime(t *testing.T) *objcRuntime {
	t.Helper()

	objcOnce.Do(func() {
		rt := &objcRuntime{}
		var err error

		rt.libobjc, err = ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
		if err != nil {
			objcInitErr = err
			return
		}
		rt.foundation, err = ffi.LoadLibrary("/System/Library/Frameworks/Foundation.framework/Foundation")
		if err != nil {
			objcInitErr = err
			return
		}
		rt.appKit, err = ffi.LoadLibrary("/System/Library/Frameworks/AppKit.framework/AppKit")
		if err != nil {
			objcInitErr = err
			return
		}
		rt.quartzCore, err = ffi.LoadLibrary("/System/Library/Frameworks/QuartzCore.framework/QuartzCore")
		if err != nil {
			objcInitErr = err
			return
		}
		rt.coreFoundation, err = ffi.LoadLibrary("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		if err != nil {
			objcInitErr = err
			return
		}

		rt.objcGetClass, err = ffi.GetSymbol(rt.libobjc, "objc_getClass")
		if err != nil {
			objcInitErr = err
			return
		}
		rt.selRegisterName, err = ffi.GetSymbol(rt.libobjc, "sel_registerName")
		if err != nil {
			objcInitErr = err
			return
		}
		rt.objcMsgSend, err = ffi.GetSymbol(rt.libobjc, "objc_msgSend")
		if err != nil {
			objcInitErr = err
			return
		}

		err = ffi.PrepareCallInterface(
			&rt.cifCStringToPtr,
			types.DefaultCall,
			types.PointerTypeDescriptor,
			[]*types.TypeDescriptor{types.PointerTypeDescriptor},
		)
		if err != nil {
			objcInitErr = err
			return
		}

		objcRuntimeV = rt
	})

	if objcInitErr != nil {
		t.Fatalf("objc runtime init failed: %v", objcInitErr)
	}

	return objcRuntimeV
}

func (rt *objcRuntime) getClass(t *testing.T, name string) uintptr {
	t.Helper()

	cname := append([]byte(name), 0)
	namePtr := unsafe.Pointer(&cname[0])

	var result uintptr
	err := ffi.CallFunction(
		&rt.cifCStringToPtr,
		rt.objcGetClass,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&namePtr)},
	)
	runtime.KeepAlive(cname)
	if err != nil {
		t.Fatalf("objc_getClass(%q) failed: %v", name, err)
	}
	if result == 0 {
		t.Fatalf("objc_getClass(%q) returned nil", name)
	}
	return result
}

func (rt *objcRuntime) sel(t *testing.T, name string) uintptr {
	t.Helper()

	cname := append([]byte(name), 0)
	namePtr := unsafe.Pointer(&cname[0])

	var result uintptr
	err := ffi.CallFunction(
		&rt.cifCStringToPtr,
		rt.selRegisterName,
		unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&namePtr)},
	)
	runtime.KeepAlive(cname)
	if err != nil {
		t.Fatalf("sel_registerName(%q) failed: %v", name, err)
	}
	if result == 0 {
		t.Fatalf("sel_registerName(%q) returned nil", name)
	}
	return result
}

type objcArg struct {
	typ       *types.TypeDescriptor
	ptr       unsafe.Pointer
	keepAlive any
}

func objcArgPtr(val uintptr) objcArg {
	v := val
	return objcArg{typ: types.PointerTypeDescriptor, ptr: unsafe.Pointer(&v), keepAlive: &v}
}

func objcArgUInt64(val uint64) objcArg {
	v := val
	return objcArg{typ: types.UInt64TypeDescriptor, ptr: unsafe.Pointer(&v), keepAlive: &v}
}

func objcArgInt64(val int64) objcArg {
	v := val
	return objcArg{typ: types.SInt64TypeDescriptor, ptr: unsafe.Pointer(&v), keepAlive: &v}
}

func objcArgBool(val bool) objcArg {
	var v uint8
	if val {
		v = 1
	}
	return objcArg{typ: types.UInt8TypeDescriptor, ptr: unsafe.Pointer(&v), keepAlive: &v}
}

func objcArgDouble(val float64) objcArg {
	v := val
	return objcArg{typ: types.DoubleTypeDescriptor, ptr: unsafe.Pointer(&v), keepAlive: &v}
}

func objcArgRect(rect nsRect) objcArg {
	v := rect
	return objcArg{typ: nsRectType, ptr: unsafe.Pointer(&v), keepAlive: &v}
}

func objcArgSize(size nsSize) objcArg {
	v := size
	return objcArg{typ: nsSizeType, ptr: unsafe.Pointer(&v), keepAlive: &v}
}

func objcCall(t *testing.T, rt *objcRuntime, retType *types.TypeDescriptor, rvalue unsafe.Pointer, self, sel uintptr, args ...objcArg) {
	t.Helper()

	argTypes := make([]*types.TypeDescriptor, 0, 2+len(args))
	argTypes = append(argTypes, types.PointerTypeDescriptor, types.PointerTypeDescriptor)
	for _, arg := range args {
		argTypes = append(argTypes, arg.typ)
	}

	cif := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(cif, types.DefaultCall, retType, argTypes); err != nil {
		t.Fatalf("ffi.PrepareCallInterface failed: %v", err)
	}

	selfPtr := self
	selPtr := sel
	argPtrs := make([]unsafe.Pointer, 0, 2+len(args))
	argPtrs = append(argPtrs, unsafe.Pointer(&selfPtr), unsafe.Pointer(&selPtr))
	for _, arg := range args {
		argPtrs = append(argPtrs, arg.ptr)
	}

	if err := ffi.CallFunction(cif, rt.objcMsgSend, rvalue, argPtrs); err != nil {
		t.Fatalf("objc_msgSend failed: %v", err)
	}
	runtime.KeepAlive(args)
}

func objcCallVoid(t *testing.T, rt *objcRuntime, self, sel uintptr, args ...objcArg) {
	objcCall(t, rt, types.VoidTypeDescriptor, nil, self, sel, args...)
}

func objcRetType[T any]() *types.TypeDescriptor {
	var zero T
	switch any(zero).(type) {
	case uintptr:
		return types.PointerTypeDescriptor
	case uint64:
		return types.UInt64TypeDescriptor
	case int64:
		return types.SInt64TypeDescriptor
	case float64:
		return types.DoubleTypeDescriptor
	case uint8:
		return types.UInt8TypeDescriptor
	case nsRect:
		return nsRectType
	case nsSize:
		return nsSizeType
	default:
		panic("unsupported objc return type")
	}
}

func objcCallRet[T any](t *testing.T, rt *objcRuntime, self, sel uintptr, args ...objcArg) T {
	var result T
	objcCall(t, rt, objcRetType[T](), unsafe.Pointer(&result), self, sel, args...)
	return result
}

func must[T any](t *testing.T, name string, v T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatalf("%s failed: %v", name, err)
	}
	return v
}

func runOnMainThread(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	var panicVal any
	mainThreadTasks <- func() {
		defer func() {
			if r := recover(); r != nil {
				panicVal = r
			}
			close(done)
		}()
		fn()
	}
	<-done
	if panicVal != nil {
		panic(panicVal)
	}
}

func withAutoreleasePool(t *testing.T, rt *objcRuntime, fn func()) {
	t.Helper()

	poolClass := rt.getClass(t, "NSAutoreleasePool")
	selNew := rt.sel(t, "new")
	selDrain := rt.sel(t, "drain")

	pool := objcCallRet[uintptr](t, rt, poolClass, selNew)
	if pool == 0 {
		t.Fatal("NSAutoreleasePool new returned nil")
	}
	defer objcCallVoid(t, rt, pool, selDrain)

	fn()
}

func loadMetalDevice(t *testing.T) uintptr {
	t.Helper()

	metal, err := ffi.LoadLibrary("/System/Library/Frameworks/Metal.framework/Metal")
	if err != nil {
		t.Fatalf("ffi.LoadLibrary(Metal) failed: %v", err)
	}
	t.Cleanup(func() {
		_ = ffi.FreeLibrary(metal)
	})

	createDevice, err := ffi.GetSymbol(metal, "MTLCreateSystemDefaultDevice")
	if err != nil {
		t.Fatalf("ffi.GetSymbol(MTLCreateSystemDefaultDevice) failed: %v", err)
	}

	cifDevice := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(cifDevice, types.DefaultCall, types.PointerTypeDescriptor, nil); err != nil {
		t.Fatalf("ffi.PrepareCallInterface(MTLCreateSystemDefaultDevice) failed: %v", err)
	}
	var device uintptr
	if err := ffi.CallFunction(cifDevice, createDevice, unsafe.Pointer(&device), nil); err != nil {
		t.Fatalf("MTLCreateSystemDefaultDevice call failed: %v", err)
	}
	if device == 0 {
		t.Skip("MTLCreateSystemDefaultDevice returned nil")
	}
	return device
}

func cString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var length int
	for {
		if *(*byte)(unsafe.Add(unsafe.Pointer(ptr), length)) == 0 {
			break
		}
		length++
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length))
}

func TestDarwinSelectorsRegistered(t *testing.T) {
	rt := loadObjcRuntime(t)

	selectors := []string{
		"alloc",
		"init",
		"new",
		"release",
		"retain",
		"sharedApplication",
		"setActivationPolicy:",
		"activateIgnoringOtherApps:",
		"run",
		"stop:",
		"terminate:",
		"nextEventMatchingMask:untilDate:inMode:dequeue:",
		"sendEvent:",
		"finishLaunching",
		"setDelegate:",
		"initWithContentRect:styleMask:backing:defer:",
		"setTitle:",
		"title",
		"setContentView:",
		"contentView",
		"makeKeyAndOrderFront:",
		"orderOut:",
		"close",
		"miniaturize:",
		"deminiaturize:",
		"zoom",
		"setFrame:display:",
		"frame",
		"contentRectForFrameRect:",
		"frameRectForContentRect:",
		"styleMask",
		"setStyleMask:",
		"setAcceptsMouseMovedEvents:",
		"makeFirstResponder:",
		"isKeyWindow",
		"isVisible",
		"isMiniaturized",
		"isZoomed",
		"setReleasedWhenClosed:",
		"center",
		"setWantsLayer:",
		"wantsLayer",
		"setLayer:",
		"layer",
		"bounds",
		"setBounds:",
		"setNeedsDisplay:",
		"mainScreen",
		"screens",
		"visibleFrame",
		"distantPast",
		"distantFuture",
		"initWithUTF8String:",
		"UTF8String",
		"length",
		"drain",
		"setContentsScale:",
		"contentsScale",
		"setDrawableSize:",
		"drawableSize",
		"setDevice:",
		"device",
		"setPixelFormat:",
		"pixelFormat",
		"nextDrawable",
		"setFramebufferOnly:",
		"setMaximumDrawableCount:",
		"setDisplaySyncEnabled:",
		"type",
		"locationInWindow",
		"modifierFlags",
		"keyCode",
		"characters",
		"charactersIgnoringModifiers",
		"isARepeat",
		"buttonNumber",
		"scrollingDeltaX",
		"scrollingDeltaY",
		"hasPreciseScrollingDeltas",
		"defaultCenter",
		"addObserver:selector:name:object:",
		"removeObserver:",
		"currentRunLoop",
		"runMode:beforeDate:",
	}

	for _, name := range selectors {
		_ = rt.sel(t, name)
	}
}

func TestDarwinClassesAvailable(t *testing.T) {
	rt := loadObjcRuntime(t)

	classes := []string{
		"NSObject",
		"NSApplication",
		"NSWindow",
		"NSView",
		"NSScreen",
		"NSDate",
		"NSString",
		"NSAutoreleasePool",
		"NSEvent",
		"NSNotificationCenter",
		"NSRunLoop",
		"CALayer",
		"CAMetalLayer",
	}

	for _, name := range classes {
		_ = rt.getClass(t, name)
	}
}

func TestDarwinNSStringRoundTrip(t *testing.T) {
	rt := loadObjcRuntime(t)

	withAutoreleasePool(t, rt, func() {
		strClass := rt.getClass(t, "NSString")
		selAlloc := rt.sel(t, "alloc")
		selInit := rt.sel(t, "initWithUTF8String:")
		selUTF8 := rt.sel(t, "UTF8String")
		selLength := rt.sel(t, "length")
		selRelease := rt.sel(t, "release")

		hello := []byte("goffi\x00")
		helloPtr := unsafe.Pointer(&hello[0])

		obj := objcCallRet[uintptr](t, rt, strClass, selAlloc)
		if obj == 0 {
			t.Fatal("NSString alloc returned nil")
		}

		obj = objcCallRet[uintptr](t, rt, obj, selInit, objcArgPtr(uintptr(helloPtr)))
		if obj == 0 {
			t.Fatal("NSString initWithUTF8String returned nil")
		}
		defer objcCallVoid(t, rt, obj, selRelease)

		length := objcCallRet[uint64](t, rt, obj, selLength)
		if length != 5 {
			t.Fatalf("NSString length = %d, want 5", length)
		}

		utf8Ptr := objcCallRet[uintptr](t, rt, obj, selUTF8)
		if utf8Ptr == 0 {
			t.Fatal("NSString UTF8String returned nil")
		}
		if got := cString(utf8Ptr); got != "goffi" {
			t.Fatalf("NSString UTF8String = %q, want %q", got, "goffi")
		}
	})
}

func TestDarwinNSStringCompareOptions(t *testing.T) {
	rt := loadObjcRuntime(t)

	withAutoreleasePool(t, rt, func() {
		strClass := rt.getClass(t, "NSString")
		selAlloc := rt.sel(t, "alloc")
		selInit := rt.sel(t, "initWithUTF8String:")
		selCompare := rt.sel(t, "compare:options:")
		selRelease := rt.sel(t, "release")

		leftBytes := []byte("alpha\x00")
		rightBytes := []byte("alpha\x00")
		leftPtr := unsafe.Pointer(&leftBytes[0])
		rightPtr := unsafe.Pointer(&rightBytes[0])

		left := objcCallRet[uintptr](t, rt, strClass, selAlloc)
		left = objcCallRet[uintptr](t, rt, left, selInit, objcArgPtr(uintptr(leftPtr)))
		defer objcCallVoid(t, rt, left, selRelease)

		right := objcCallRet[uintptr](t, rt, strClass, selAlloc)
		right = objcCallRet[uintptr](t, rt, right, selInit, objcArgPtr(uintptr(rightPtr)))
		defer objcCallVoid(t, rt, right, selRelease)

		result := objcCallRet[int64](t, rt, left, selCompare, objcArgPtr(right), objcArgUInt64(0))
		if result != 0 {
			t.Fatalf("NSString compare:options: = %d, want 0", result)
		}
	})
}

func TestDarwinNSNumberDoubleValue(t *testing.T) {
	rt := loadObjcRuntime(t)

	withAutoreleasePool(t, rt, func() {
		numClass := rt.getClass(t, "NSNumber")
		selNumberWithDouble := rt.sel(t, "numberWithDouble:")
		selDoubleValue := rt.sel(t, "doubleValue")

		num := objcCallRet[uintptr](t, rt, numClass, selNumberWithDouble, objcArgDouble(3.25))
		if num == 0 {
			t.Fatal("NSNumber numberWithDouble returned nil")
		}

		got := objcCallRet[float64](t, rt, num, selDoubleValue)
		if math.Abs(got-3.25) > 1e-9 {
			t.Fatalf("NSNumber doubleValue = %.6f, want 3.25", got)
		}
	})
}

func TestDarwinNSScreenVisibleFrame(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("struct return tests require arm64")
	}

	rt := loadObjcRuntime(t)

	withAutoreleasePool(t, rt, func() {
		screenClass := rt.getClass(t, "NSScreen")
		selMainScreen := rt.sel(t, "mainScreen")
		selVisibleFrame := rt.sel(t, "visibleFrame")

		mainScreen := objcCallRet[uintptr](t, rt, screenClass, selMainScreen)
		if mainScreen == 0 {
			t.Skip("NSScreen mainScreen returned nil")
		}

		frame := objcCallRet[nsRect](t, rt, mainScreen, selVisibleFrame)
		if frame.Size.Width <= 0 || frame.Size.Height <= 0 {
			t.Fatalf("NSScreen visibleFrame = %+v, want positive size", frame)
		}
	})
}

func TestDarwinCoreGraphicsStructs(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("struct argument/return tests require arm64")
	}

	handle, err := ffi.LoadLibrary("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	if err != nil {
		t.Fatalf("ffi.LoadLibrary(CoreGraphics) failed: %v", err)
	}
	defer ffi.FreeLibrary(handle)

	mainDisplayID, err := ffi.GetSymbol(handle, "CGMainDisplayID")
	if err != nil {
		t.Fatalf("ffi.GetSymbol(CGMainDisplayID) failed: %v", err)
	}
	displayBounds, err := ffi.GetSymbol(handle, "CGDisplayBounds")
	if err != nil {
		t.Fatalf("ffi.GetSymbol(CGDisplayBounds) failed: %v", err)
	}
	pathCreateRect, err := ffi.GetSymbol(handle, "CGPathCreateWithRect")
	if err != nil {
		t.Fatalf("ffi.GetSymbol(CGPathCreateWithRect) failed: %v", err)
	}
	pathRelease, err := ffi.GetSymbol(handle, "CGPathRelease")
	if err != nil {
		t.Fatalf("ffi.GetSymbol(CGPathRelease) failed: %v", err)
	}

	displayIDCIF := &types.CallInterface{}
	err = ffi.PrepareCallInterface(displayIDCIF, types.DefaultCall, types.UInt32TypeDescriptor, nil)
	if err != nil {
		t.Fatalf("ffi.PrepareCallInterface(CGMainDisplayID) failed: %v", err)
	}
	var displayID uint32
	err = ffi.CallFunction(displayIDCIF, mainDisplayID, unsafe.Pointer(&displayID), nil)
	if err != nil {
		t.Fatalf("CGMainDisplayID call failed: %v", err)
	}
	if displayID == 0 {
		t.Skip("CGMainDisplayID returned 0")
	}

	boundsCIF := &types.CallInterface{}
	err = ffi.PrepareCallInterface(boundsCIF, types.DefaultCall, nsRectType, []*types.TypeDescriptor{
		types.UInt32TypeDescriptor,
	})
	if err != nil {
		t.Fatalf("ffi.PrepareCallInterface(CGDisplayBounds) failed: %v", err)
	}

	var bounds nsRect
	err = ffi.CallFunction(boundsCIF, displayBounds, unsafe.Pointer(&bounds), []unsafe.Pointer{
		unsafe.Pointer(&displayID),
	})
	if err != nil {
		t.Fatalf("CGDisplayBounds call failed: %v", err)
	}
	if bounds.Size.Width <= 0 || bounds.Size.Height <= 0 {
		t.Fatalf("CGDisplayBounds = %+v, want positive size", bounds)
	}

	pathCIF := &types.CallInterface{}
	err = ffi.PrepareCallInterface(pathCIF, types.DefaultCall, types.PointerTypeDescriptor, []*types.TypeDescriptor{
		nsRectType,
		types.PointerTypeDescriptor,
	})
	if err != nil {
		t.Fatalf("ffi.PrepareCallInterface(CGPathCreateWithRect) failed: %v", err)
	}

	rect := nsRect{
		Origin: nsPoint{X: 1.25, Y: 2.5},
		Size:   nsSize{Width: 100.5, Height: 200.25},
	}
	var transform uintptr
	var path uintptr
	err = ffi.CallFunction(pathCIF, pathCreateRect, unsafe.Pointer(&path), []unsafe.Pointer{
		unsafe.Pointer(&rect),
		unsafe.Pointer(&transform),
	})
	if err != nil {
		t.Fatalf("CGPathCreateWithRect call failed: %v", err)
	}
	if path == 0 {
		t.Fatalf("CGPathCreateWithRect returned nil")
	}

	releaseCIF := &types.CallInterface{}
	err = ffi.PrepareCallInterface(releaseCIF, types.DefaultCall, types.VoidTypeDescriptor, []*types.TypeDescriptor{
		types.PointerTypeDescriptor,
	})
	if err != nil {
		t.Fatalf("ffi.PrepareCallInterface(CGPathRelease) failed: %v", err)
	}
	err = ffi.CallFunction(releaseCIF, pathRelease, nil, []unsafe.Pointer{
		unsafe.Pointer(&path),
	})
	if err != nil {
		t.Fatalf("CGPathRelease call failed: %v", err)
	}
}

func TestDarwinCAMetalLayerProperties(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("struct argument/return tests require arm64")
	}

	rt := loadObjcRuntime(t)

	metal, err := ffi.LoadLibrary("/System/Library/Frameworks/Metal.framework/Metal")
	if err != nil {
		t.Fatalf("ffi.LoadLibrary(Metal) failed: %v", err)
	}
	defer ffi.FreeLibrary(metal)
	createDevice, err := ffi.GetSymbol(metal, "MTLCreateSystemDefaultDevice")
	if err != nil {
		t.Fatalf("ffi.GetSymbol(MTLCreateSystemDefaultDevice) failed: %v", err)
	}

	cifDevice := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(cifDevice, types.DefaultCall, types.PointerTypeDescriptor, nil); err != nil {
		t.Fatalf("ffi.PrepareCallInterface(MTLCreateSystemDefaultDevice) failed: %v", err)
	}
	var device uintptr
	if err := ffi.CallFunction(cifDevice, createDevice, unsafe.Pointer(&device), nil); err != nil {
		t.Fatalf("MTLCreateSystemDefaultDevice call failed: %v", err)
	}
	if device == 0 {
		t.Skip("MTLCreateSystemDefaultDevice returned nil")
	}

	withAutoreleasePool(t, rt, func() {

		layerClass := rt.getClass(t, "CAMetalLayer")
		selNew := rt.sel(t, "new")
		selRelease := rt.sel(t, "release")
		selSetDevice := rt.sel(t, "setDevice:")
		selSetContentsScale := rt.sel(t, "setContentsScale:")
		selContentsScale := rt.sel(t, "contentsScale")
		selSetDrawableSize := rt.sel(t, "setDrawableSize:")
		selDrawableSize := rt.sel(t, "drawableSize")
		selSetPixelFormat := rt.sel(t, "setPixelFormat:")
		selPixelFormat := rt.sel(t, "pixelFormat")
		selSetFramebufferOnly := rt.sel(t, "setFramebufferOnly:")
		selSetDisplaySyncEnabled := rt.sel(t, "setDisplaySyncEnabled:")

		layer := objcCallRet[uintptr](t, rt, layerClass, selNew)
		if layer == 0 {
			t.Fatal("CAMetalLayer new returned nil")
		}
		defer objcCallVoid(t, rt, layer, selRelease)

		objcCallVoid(t, rt, layer, selSetDevice, objcArgPtr(device))

		objcCallVoid(t, rt, layer, selSetContentsScale, objcArgDouble(2.0))
		scale := objcCallRet[float64](t, rt, layer, selContentsScale)
		if math.Abs(scale-2.0) > 1e-9 {
			t.Fatalf("CAMetalLayer contentsScale = %.6f, want 2.0", scale)
		}

		size := nsSize{Width: 640, Height: 480}
		objcCallVoid(t, rt, layer, selSetDrawableSize, objcArgDouble(size.Width), objcArgDouble(size.Height))
		gotSize := objcCallRet[nsSize](t, rt, layer, selDrawableSize)
		if math.Abs(gotSize.Width-size.Width) > 1e-6 || math.Abs(gotSize.Height-size.Height) > 1e-6 {
			t.Fatalf("CAMetalLayer drawableSize = %+v, want %+v", gotSize, size)
		}

		objcCallVoid(t, rt, layer, selSetPixelFormat, objcArgUInt64(80))
		pixelFormat := objcCallRet[uint64](t, rt, layer, selPixelFormat)
		if pixelFormat != 80 {
			t.Fatalf("CAMetalLayer pixelFormat = %d, want 80", pixelFormat)
		}

		objcCallVoid(t, rt, layer, selSetFramebufferOnly, objcArgBool(true))
		objcCallVoid(t, rt, layer, selSetDisplaySyncEnabled, objcArgBool(true))
	})
}

func TestDarwinObjcStressLoop(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("stress tests require arm64")
	}

	rt := loadObjcRuntime(t)
	device := loadMetalDevice(t)

	poolClass := rt.getClass(t, "NSAutoreleasePool")
	strClass := rt.getClass(t, "NSString")
	numClass := rt.getClass(t, "NSNumber")
	screenClass := rt.getClass(t, "NSScreen")
	layerClass := rt.getClass(t, "CAMetalLayer")

	selNew := rt.sel(t, "new")
	selRelease := rt.sel(t, "release")
	selAlloc := rt.sel(t, "alloc")
	selInit := rt.sel(t, "initWithUTF8String:")
	selUTF8 := rt.sel(t, "UTF8String")
	selLength := rt.sel(t, "length")
	selCompare := rt.sel(t, "compare:options:")
	selNumberWithDouble := rt.sel(t, "numberWithDouble:")
	selDoubleValue := rt.sel(t, "doubleValue")
	selMainScreen := rt.sel(t, "mainScreen")
	selVisibleFrame := rt.sel(t, "visibleFrame")
	selSetDevice := rt.sel(t, "setDevice:")
	selSetContentsScale := rt.sel(t, "setContentsScale:")
	selContentsScale := rt.sel(t, "contentsScale")
	selSetDrawableSize := rt.sel(t, "setDrawableSize:")
	selDrawableSize := rt.sel(t, "drawableSize")
	selSetPixelFormat := rt.sel(t, "setPixelFormat:")
	selPixelFormat := rt.sel(t, "pixelFormat")
	selSetFramebufferOnly := rt.sel(t, "setFramebufferOnly:")
	selSetDisplaySyncEnabled := rt.sel(t, "setDisplaySyncEnabled:")
	selNextDrawable := rt.sel(t, "nextDrawable")

	hello := []byte("goffi\x00")
	helloPtr := unsafe.Pointer(&hello[0])
	alpha := []byte("alpha\x00")
	alphaPtr := unsafe.Pointer(&alpha[0])

	const iterations = 100
	for i := 0; i < iterations; i++ {
		pool := objcCallRet[uintptr](t, rt, poolClass, selNew)
		if pool == 0 {
			t.Fatal("NSAutoreleasePool new returned nil")
		}

		// NSString round trip + compare
		left := objcCallRet[uintptr](t, rt, strClass, selAlloc)
		if left == 0 {
			t.Fatal("NSString alloc returned nil")
		}
		left = objcCallRet[uintptr](t, rt, left, selInit, objcArgPtr(uintptr(alphaPtr)))
		if left == 0 {
			t.Fatal("NSString initWithUTF8String returned nil")
		}
		right := objcCallRet[uintptr](t, rt, strClass, selAlloc)
		if right == 0 {
			t.Fatal("NSString alloc returned nil")
		}
		right = objcCallRet[uintptr](t, rt, right, selInit, objcArgPtr(uintptr(alphaPtr)))
		if right == 0 {
			t.Fatal("NSString initWithUTF8String returned nil")
		}

		length := objcCallRet[uint64](t, rt, left, selLength)
		if length != 5 {
			t.Fatalf("NSString length = %d, want 5", length)
		}
		utf8Ptr := objcCallRet[uintptr](t, rt, left, selUTF8)
		if utf8Ptr == 0 {
			t.Fatal("NSString UTF8String returned nil")
		}
		if got := cString(utf8Ptr); got != "alpha" {
			t.Fatalf("NSString UTF8String = %q, want %q", got, "alpha")
		}
		if cmp := objcCallRet[int64](t, rt, left, selCompare, objcArgPtr(right), objcArgUInt64(0)); cmp != 0 {
			t.Fatalf("NSString compare:options: = %d, want 0", cmp)
		}
		objcCallVoid(t, rt, right, selRelease)
		objcCallVoid(t, rt, left, selRelease)

		// NSNumber double round trip
		num := objcCallRet[uintptr](t, rt, numClass, selNumberWithDouble, objcArgDouble(3.25))
		if num == 0 {
			t.Fatal("NSNumber numberWithDouble returned nil")
		}
		if got := objcCallRet[float64](t, rt, num, selDoubleValue); math.Abs(got-3.25) > 1e-9 {
			t.Fatalf("NSNumber doubleValue = %.6f, want 3.25", got)
		}

		// NSScreen visibleFrame (struct return)
		mainScreen := objcCallRet[uintptr](t, rt, screenClass, selMainScreen)
		if mainScreen != 0 {
			frame := objcCallRet[nsRect](t, rt, mainScreen, selVisibleFrame)
			if frame.Size.Width <= 0 || frame.Size.Height <= 0 {
				t.Fatalf("NSScreen visibleFrame = %+v, want positive size", frame)
			}
		}

		// CAMetalLayer properties (mix bool/uint/double/struct)
		layer := objcCallRet[uintptr](t, rt, layerClass, selNew)
		if layer == 0 {
			t.Fatal("CAMetalLayer new returned nil")
		}
		objcCallVoid(t, rt, layer, selSetDevice, objcArgPtr(device))
		objcCallVoid(t, rt, layer, selSetContentsScale, objcArgDouble(2.0))
		scale := objcCallRet[float64](t, rt, layer, selContentsScale)
		if math.Abs(scale-2.0) > 1e-9 {
			t.Fatalf("CAMetalLayer contentsScale = %.6f, want 2.0", scale)
		}
		size := nsSize{Width: 640, Height: 480}
		objcCallVoid(t, rt, layer, selSetDrawableSize, objcArgDouble(size.Width), objcArgDouble(size.Height))
		gotSize := objcCallRet[nsSize](t, rt, layer, selDrawableSize)
		if math.Abs(gotSize.Width-size.Width) > 1e-6 || math.Abs(gotSize.Height-size.Height) > 1e-6 {
			t.Fatalf("CAMetalLayer drawableSize = %+v, want %+v", gotSize, size)
		}
		objcCallVoid(t, rt, layer, selSetPixelFormat, objcArgUInt64(80))
		if pixelFormat := objcCallRet[uint64](t, rt, layer, selPixelFormat); pixelFormat != 80 {
			t.Fatalf("CAMetalLayer pixelFormat = %d, want 80", pixelFormat)
		}
		objcCallVoid(t, rt, layer, selSetFramebufferOnly, objcArgBool(true))
		objcCallVoid(t, rt, layer, selSetDisplaySyncEnabled, objcArgBool(true))
		_ = objcCallRet[uintptr](t, rt, layer, selNextDrawable)
		objcCallVoid(t, rt, layer, selRelease)

		// extra NSString traffic to stress autorelease + retain/release paths
		for j := 0; j < 10; j++ {
			obj := objcCallRet[uintptr](t, rt, strClass, selAlloc)
			if obj == 0 {
				t.Fatal("NSString alloc returned nil")
			}
			obj = objcCallRet[uintptr](t, rt, obj, selInit, objcArgPtr(uintptr(helloPtr)))
			if obj == 0 {
				t.Fatal("NSString initWithUTF8String returned nil")
			}
			objcCallVoid(t, rt, obj, selRelease)
		}

		objcCallVoid(t, rt, pool, selRelease)
		runtime.GC()
	}
}

func TestDarwinGogpuWindowSurfaceStress(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("stress tests require arm64")
	}

	runOnMainThread(t, func() {
		app := platformdarwin.GetApplication()
		config := platformdarwin.WindowConfig{
			Title:     "gogpu",
			Width:     640,
			Height:    480,
			Resizable: true,
		}

		const iterations = 100
		for i := 0; i < iterations; i++ {
			if err := app.Init(); err != nil {
				t.Fatalf("Application.Init failed: %v", err)
			}

			for j := 0; j < 3; j++ {
				w, err := platformdarwin.NewWindow(config)
				w = must(t, "NewWindow", w, err)
				w.Show()
				w.SetTitle("gogpu")
				w.SetSize(640+(i+j)%7, 480+(i+j)%5)
				_ = w.Frame()
				_ = w.ContentRect()
				_ = w.IsVisible()
				w.UpdateSize()

				s, err := platformdarwin.NewSurface(w)
				s = must(t, "NewSurface", s, err)
				s.Configure(platformdarwin.DefaultSurfaceConfig())
				s.UpdateSize()

				app.PollEvents()

				s.Destroy()
				w.Destroy()
			}

			app.Destroy()
			runtime.GC()
		}
	})
}

// This isolates the Application Init/Destroy lifecycle without windows/surfaces.
// If this crashes, the NSAutoreleasePool or NSApplication path is unsafe by itself.
func TestDarwinApplicationInitDestroyLoop(t *testing.T) {
	t.Skip("Stress test disabled")
	if runtime.GOARCH != "arm64" {
		t.Skip("stress tests require arm64")
	}

	runOnMainThread(t, func() {
		app := platformdarwin.GetApplication()
		const iterations = 50
		for i := 0; i < iterations; i++ {
			if err := app.Init(); err != nil {
				t.Fatalf("Application.Init failed: %v", err)
			}
			app.Destroy()
			runtime.GC()
		}
	})
}
