//go:build darwin

package darwin

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// Errors returned by BlitPixels operations.
var (
	ErrCoreGraphicsNotLoaded = errors.New("darwin: CoreGraphics framework not loaded")
	ErrCGImageCreationFailed = errors.New("darwin: failed to create CGImage from pixel data")
)

// CoreGraphics alpha info constants.
const (
	// kCGImageAlphaLast — the alpha component is stored in the least significant
	// bits of each pixel (RGBA byte order).
	cgImageAlphaLast = 3

	// kCGBitmapByteOrderDefault — use the default byte order.
	cgBitmapByteOrderDefault = 0
)

// cgState holds loaded CoreGraphics symbols for pixel blitting.
var cgState struct {
	once sync.Once
	err  error

	lib unsafe.Pointer

	// Function pointers
	cgColorSpaceCreateDeviceRGB unsafe.Pointer
	cgColorSpaceRelease         unsafe.Pointer
	cgBitmapContextCreate       unsafe.Pointer
	cgBitmapContextCreateImage  unsafe.Pointer
	cgContextRelease            unsafe.Pointer
	cgImageRelease              unsafe.Pointer

	// Call interfaces
	cifNoArgs      *types.CallInterface // () -> pointer
	cifReleasePtr  *types.CallInterface // (pointer) -> void
	cifBitmapCtx   *types.CallInterface // (ptr, uint, uint, uint, uint, ptr, uint32) -> ptr
	cifCreateImage *types.CallInterface // (ptr) -> ptr

	// Cached color space (created once, reused)
	colorSpace uintptr
}

// initCoreGraphics loads CoreGraphics symbols needed for pixel blitting.
func initCoreGraphics() error {
	cgState.once.Do(func() {
		cgState.err = loadCoreGraphics()
	})
	return cgState.err
}

func loadCoreGraphics() error {
	var err error

	cgState.lib, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	if err != nil {
		return errors.Join(ErrCoreGraphicsNotLoaded, err)
	}

	// Resolve symbols
	cgState.cgColorSpaceCreateDeviceRGB, err = ffi.GetSymbol(
		cgState.lib, "CGColorSpaceCreateDeviceRGB")
	if err != nil {
		return err
	}

	cgState.cgColorSpaceRelease, err = ffi.GetSymbol(
		cgState.lib, "CGColorSpaceRelease")
	if err != nil {
		return err
	}

	cgState.cgBitmapContextCreate, err = ffi.GetSymbol(
		cgState.lib, "CGBitmapContextCreate")
	if err != nil {
		return err
	}

	cgState.cgBitmapContextCreateImage, err = ffi.GetSymbol(
		cgState.lib, "CGBitmapContextCreateImage")
	if err != nil {
		return err
	}

	cgState.cgContextRelease, err = ffi.GetSymbol(
		cgState.lib, "CGContextRelease")
	if err != nil {
		return err
	}

	cgState.cgImageRelease, err = ffi.GetSymbol(
		cgState.lib, "CGImageRelease")
	if err != nil {
		return err
	}

	// Prepare CIFs

	// CGColorSpaceCreateDeviceRGB() -> CGColorSpaceRef
	cgState.cifNoArgs = &types.CallInterface{}
	err = ffi.PrepareCallInterface(cgState.cifNoArgs, types.DefaultCall,
		types.PointerTypeDescriptor, []*types.TypeDescriptor{})
	if err != nil {
		return err
	}

	// CGColorSpaceRelease(CGColorSpaceRef) -> void
	// CGContextRelease(CGContextRef) -> void
	// CGImageRelease(CGImageRef) -> void
	cgState.cifReleasePtr = &types.CallInterface{}
	err = ffi.PrepareCallInterface(cgState.cifReleasePtr, types.DefaultCall,
		types.VoidTypeDescriptor, []*types.TypeDescriptor{
			types.PointerTypeDescriptor,
		})
	if err != nil {
		return err
	}

	// CGBitmapContextCreate(void *data, size_t width, size_t height,
	//   size_t bitsPerComponent, size_t bytesPerRow,
	//   CGColorSpaceRef space, uint32_t bitmapInfo) -> CGContextRef
	cgState.cifBitmapCtx = &types.CallInterface{}
	err = ffi.PrepareCallInterface(cgState.cifBitmapCtx, types.DefaultCall,
		types.PointerTypeDescriptor, []*types.TypeDescriptor{
			types.PointerTypeDescriptor, // data
			types.UInt64TypeDescriptor,  // width (size_t = uint64 on 64-bit)
			types.UInt64TypeDescriptor,  // height
			types.UInt64TypeDescriptor,  // bitsPerComponent
			types.UInt64TypeDescriptor,  // bytesPerRow
			types.PointerTypeDescriptor, // colorSpace
			types.UInt32TypeDescriptor,  // bitmapInfo
		})
	if err != nil {
		return err
	}

	// CGBitmapContextCreateImage(CGContextRef) -> CGImageRef
	cgState.cifCreateImage = &types.CallInterface{}
	err = ffi.PrepareCallInterface(cgState.cifCreateImage, types.DefaultCall,
		types.PointerTypeDescriptor, []*types.TypeDescriptor{
			types.PointerTypeDescriptor,
		})
	if err != nil {
		return err
	}

	// Create and cache the device RGB color space
	var csResult uintptr
	err = ffi.CallFunction(cgState.cifNoArgs, cgState.cgColorSpaceCreateDeviceRGB,
		unsafe.Pointer(&csResult), nil)
	if err != nil || csResult == 0 {
		return errors.New("darwin: CGColorSpaceCreateDeviceRGB failed")
	}
	cgState.colorSpace = csResult

	return nil
}

// CreateCGImageFromRGBA creates a CGImage from RGBA pixel data.
// The returned CGImageRef must be released with ReleaseCGImage.
// The pixels slice must remain valid until the CGImage is released.
func CreateCGImageFromRGBA(pixels []byte, width, height int) (uintptr, error) {
	if err := initCoreGraphics(); err != nil {
		return 0, err
	}

	stride := width * 4
	bitsPerComponent := uint64(8)
	w := uint64(width)
	h := uint64(height)
	bytesPerRow := uint64(stride)
	bitmapInfo := uint32(cgImageAlphaLast | cgBitmapByteOrderDefault)

	// Create bitmap context from pixel data
	dataPtr := uintptr(unsafe.Pointer(&pixels[0]))

	var ctxResult uintptr
	args := [7]unsafe.Pointer{
		unsafe.Pointer(&dataPtr),
		unsafe.Pointer(&w),
		unsafe.Pointer(&h),
		unsafe.Pointer(&bitsPerComponent),
		unsafe.Pointer(&bytesPerRow),
		unsafe.Pointer(&cgState.colorSpace),
		unsafe.Pointer(&bitmapInfo),
	}

	err := ffi.CallFunction(cgState.cifBitmapCtx, cgState.cgBitmapContextCreate,
		unsafe.Pointer(&ctxResult), args[:])
	if err != nil || ctxResult == 0 {
		return 0, ErrCGImageCreationFailed
	}

	// Create CGImage from context
	var imgResult uintptr
	imgArgs := [1]unsafe.Pointer{unsafe.Pointer(&ctxResult)}
	err = ffi.CallFunction(cgState.cifCreateImage, cgState.cgBitmapContextCreateImage,
		unsafe.Pointer(&imgResult), imgArgs[:])

	// Release the bitmap context (image retains the data it needs)
	releaseArgs := [1]unsafe.Pointer{unsafe.Pointer(&ctxResult)}
	_ = ffi.CallFunction(cgState.cifReleasePtr, cgState.cgContextRelease,
		unsafe.Pointer(new(uintptr)), releaseArgs[:])

	if err != nil || imgResult == 0 {
		return 0, ErrCGImageCreationFailed
	}

	return imgResult, nil
}

// ReleaseCGImage releases a CGImage created by CreateCGImageFromRGBA.
func ReleaseCGImage(image uintptr) {
	if image == 0 {
		return
	}
	if err := initCoreGraphics(); err != nil {
		return
	}
	args := [1]unsafe.Pointer{unsafe.Pointer(&image)}
	_ = ffi.CallFunction(cgState.cifReleasePtr, cgState.cgImageRelease,
		unsafe.Pointer(new(uintptr)), args[:])
}
