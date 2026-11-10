//go:build darwin

package darwin

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// cursorState holds the CoreGraphics symbols that move the cursor and detach
// it from the mouse.
var cursorState struct {
	once sync.Once
	err  error

	associateMouseAndMouseCursorPosition unsafe.Pointer
	warpMouseCursorPosition              unsafe.Pointer

	cifAssociate *types.CallInterface // (boolean_t) -> CGError
	cifWarp      *types.CallInterface // (CGPoint) -> CGError
}

// HideCursor hides the cursor. Hides are counted: each needs an UnhideCursor.
func HideCursor() {
	initClasses()
	initSelectors()
	ID(classes.NSCursor).Send(selectors.hideCursor)
}

// UnhideCursor undoes one HideCursor.
func UnhideCursor() {
	initClasses()
	initSelectors()
	ID(classes.NSCursor).Send(selectors.unhideCursor)
}

// ShapeCursor returns the NSCursor of a gpucontext.CursorShape value (0-11),
// the arrow for one it does not know. CursorNone's has no visible pixel, so a
// view showing it hides the cursor over that view alone, where HideCursor
// hides it everywhere.
func ShapeCursor(shape int) ID {
	initClasses()
	initSelectors()
	cursorClass := ID(classes.NSCursor)
	switch shape {
	case 1: // CursorPointer
		return cursorClass.Send(selectors.pointingHandCursor)
	case 2: // CursorText
		return cursorClass.Send(selectors.IBeamCursor)
	case 3: // CursorCrosshair
		return cursorClass.Send(selectors.crosshairCursor)
	case 4: // CursorMove
		return cursorClass.Send(selectors.openHandCursor)
	case 5: // CursorResizeNS
		return cursorClass.Send(selectors.resizeUpDownCursor)
	case 6: // CursorResizeEW
		return cursorClass.Send(selectors.resizeLeftRightCursor)
	case 9: // CursorNotAllowed
		return cursorClass.Send(selectors.operationNotAllowedCursor)
	case 11: // CursorNone
		return invisibleCursor()
	default: // CursorDefault, and the shapes AppKit has no cursor for
		return cursorClass.Send(selectors.arrowCursor)
	}
}

// invisibleCursor is an NSCursor whose 16x16 image holds no pixel, made once.
var invisibleCursor = sync.OnceValue(func() ID {
	image := GetClass("NSImage").Send(selectors.alloc).SendSize(RegisterSelector("initWithSize:"), NSSize{Width: 16, Height: 16})
	if image.IsNil() {
		return 0
	}
	cursor := ID(classes.NSCursor).Send(selectors.alloc).SendPtrPoint(RegisterSelector("initWithImage:hotSpot:"), image.Ptr(), NSPoint{})
	image.Send(selectors.release) // the cursor retains it
	return cursor
})

// CursorLocation returns the cursor's position in Cocoa screen coordinates:
// origin at the bottom-left of the primary display, y up.
func CursorLocation() NSPoint {
	initClasses()
	initSelectors()
	return ID(classes.NSEvent).GetPoint(selectors.mouseLocation)
}

// AssociateCursorWithMouse links the cursor to the mouse, or with false
// detaches it: the mouse still delivers motion in event deltas, but the cursor
// stays where it is (CGAssociateMouseAndMouseCursorPosition).
func AssociateCursorWithMouse(associated bool) error {
	if err := initCursor(); err != nil {
		return err
	}
	var value uint32
	if associated {
		value = 1
	}
	var result uint64
	args := [1]unsafe.Pointer{unsafe.Pointer(&value)}
	if _, err := ffi.CallFunction(cursorState.cifAssociate,
		cursorState.associateMouseAndMouseCursorPosition, unsafe.Pointer(&result), args[:]); err != nil {
		return err
	}
	if code := int32(result); code != 0 {
		return fmt.Errorf("darwin: CGAssociateMouseAndMouseCursorPosition: CGError %d", code)
	}
	return nil
}

// WarpCursor moves the cursor to a point in Cocoa screen coordinates without
// generating a mouse event (CGWarpMouseCursorPosition).
func WarpCursor(point NSPoint) error {
	if err := initCursor(); err != nil {
		return err
	}
	// CoreGraphics' global display space has its origin at the top-left of
	// the primary display, y down.
	initClasses()
	initSelectors()
	screens := ID(classes.NSScreen).Send(selectors.screens)
	if screens.IsNil() || screens.GetUint64(selectors.count) == 0 {
		return errors.New("darwin: no screen to warp the cursor on")
	}
	primary := screens.SendUint(selectors.objectAtIndex, 0).GetRect(selectors.frame)
	target := NSPoint{X: point.X, Y: primary.Size.Height - point.Y}

	var result uint64
	args := [1]unsafe.Pointer{unsafe.Pointer(&target)}
	if _, err := ffi.CallFunction(cursorState.cifWarp,
		cursorState.warpMouseCursorPosition, unsafe.Pointer(&result), args[:]); err != nil {
		return err
	}
	if code := int32(result); code != 0 {
		return fmt.Errorf("darwin: CGWarpMouseCursorPosition: CGError %d", code)
	}
	return nil
}

// initCursor loads the CoreGraphics cursor symbols once.
func initCursor() error {
	cursorState.once.Do(func() {
		cursorState.err = loadCursor()
	})
	return cursorState.err
}

func loadCursor() error {
	lib, err := ffi.LoadLibrary("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	if err != nil {
		return errors.Join(ErrCoreGraphicsNotLoaded, err)
	}

	cursorState.associateMouseAndMouseCursorPosition, err = ffi.GetSymbol(lib, "CGAssociateMouseAndMouseCursorPosition")
	if err != nil {
		return err
	}
	cursorState.warpMouseCursorPosition, err = ffi.GetSymbol(lib, "CGWarpMouseCursorPosition")
	if err != nil {
		return err
	}

	cursorState.cifAssociate = &types.CallInterface{}
	err = ffi.PrepareCallInterface(cursorState.cifAssociate, types.DefaultCall,
		types.SInt32TypeDescriptor, []*types.TypeDescriptor{types.UInt32TypeDescriptor})
	if err != nil {
		return err
	}

	cursorState.cifWarp = &types.CallInterface{}
	return ffi.PrepareCallInterface(cursorState.cifWarp, types.DefaultCall,
		types.SInt32TypeDescriptor, []*types.TypeDescriptor{nsPointType})
}
