//go:build darwin

package darwin

import (
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// objcSuper is the ABI shape expected by objc_msgSendSuper. The receiver is
// the GoGPUView instance and superClass is NSTextView, not GoGPUView.
type objcSuper struct {
	receiver   uintptr
	superClass uintptr
}

// callGoGPUViewSuperVoid forwards a mutating NSTextInputClient callback to
// NSTextView after the Go handler has observed it. text is nil for selectors
// without a text argument (currently unmarkText); each range is one NSRange.
// The helper deliberately keeps all ABI details in the Darwin package so the
// platform layer only deals in UTF-16 location/length pairs.
func callGoGPUViewSuperVoid(self ID, selectorName string, text *ID, ranges ...[2]uintptr) {
	if self == 0 || !goGPUViewTextInput || goGPUViewSuper == 0 || objcRT.objcMsgSendSuper == nil {
		return
	}
	if err := initRuntime(); err != nil {
		return
	}
	selector := RegisterSelector(selectorName)
	if selector == 0 {
		return
	}

	super := objcSuper{receiver: uintptr(self), superClass: uintptr(goGPUViewSuper)}
	superPtr := uintptr(unsafe.Pointer(&super))
	selectorPtr := uintptr(selector)

	argTypes := make([]*types.TypeDescriptor, 0, 2+1+len(ranges))
	argTypes = append(argTypes,
		types.PointerTypeDescriptor, // struct objc_super*
		types.PointerTypeDescriptor, // SEL
	)
	argPtrs := make([]unsafe.Pointer, 0, 2+1+len(ranges))
	argPtrs = append(argPtrs, unsafe.Pointer(&superPtr), unsafe.Pointer(&selectorPtr))
	if text != nil {
		argTypes = append(argTypes, types.PointerTypeDescriptor)
		argPtrs = append(argPtrs, unsafe.Pointer(text))
	}

	// Make a heap-backed copy so every pointer remains stable while libffi
	// marshals the call. A Go [2]uintptr has the same layout as NSRange on the
	// supported 64-bit Darwin targets.
	rangeArgs := make([][2]uintptr, len(ranges))
	copy(rangeArgs, ranges)
	for i := range rangeArgs {
		argTypes = append(argTypes, nsRangeType)
		argPtrs = append(argPtrs, unsafe.Pointer(&rangeArgs[i]))
	}

	cif := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(cif, types.DefaultCall, types.VoidTypeDescriptor, argTypes); err != nil {
		return
	}
	viewTextInputMu.Lock()
	viewTextInputForward[uintptr(self)]++
	viewTextInputMu.Unlock()
	defer func() {
		viewTextInputMu.Lock()
		key := uintptr(self)
		depth, ok := viewTextInputForward[key]
		if !ok || depth <= 1 {
			delete(viewTextInputForward, uintptr(self))
		} else {
			viewTextInputForward[key] = depth - 1
		}
		viewTextInputMu.Unlock()
	}()
	_, _ = ffi.CallFunction(cif, objcRT.objcMsgSendSuper, nil, argPtrs)
}

// InterpretKeyEvent asks AppKit's input manager to translate one key event.
// It is called only while a GoGPUView has IME enabled; the ordinary disabled
// path dispatches NSEvent.characters directly to preserve existing behavior.
func InterpretKeyEvent(view, event ID) {
	if view == 0 || event == 0 || !goGPUViewTextInput {
		return
	}
	initSelectors()
	initClasses()
	if classes.NSArray == 0 {
		return
	}
	events := ID(classes.NSArray).SendPtr(selectors.arrayWithObject, event.Ptr())
	if events == 0 {
		return
	}
	view.SendPtr(selectors.interpretKeyEvents, events.Ptr())
}

// InvalidateInputContext requests AppKit to re-query the native text-input
// geometry. NSTextView owns firstRectForCharacterRange: because goffi cannot
// portably expose an NSRect-returning callback; the controller records its
// logical cursor area in the platform layer and invalidates native geometry at
// this main-thread boundary.
func InvalidateInputContext(view ID) {
	if view == 0 || !goGPUViewTextInput {
		return
	}
	initSelectors()
	context := view.Send(selectors.inputContext)
	if context != 0 {
		context.Send(selectors.invalidateCharacterCoordinates)
	}
}

// SetTextInputCursorArea updates NSTextView's native text-container origin.
// This lets the inherited firstRectForCharacterRange: report the logical
// caret point without requiring an NSRect-returning Go callback. It is called
// only from AppKit's main-thread event boundary.
func SetTextInputCursorArea(view ID, x, y float64) {
	if view == 0 || !goGPUViewTextInput {
		return
	}
	initSelectors()
	view.SendSize(selectors.setTextContainerInset, NSSize{Width: x, Height: y})
	// NSTextView's default line fragment padding would otherwise shift the
	// insertion point horizontally away from the requested caret location.
	if container := view.Send(selectors.textContainer); container != 0 {
		container.SendDouble(selectors.setLineFragmentPadding, 0)
	}
}

// UnmarkText clears NSTextView's native preedit state. The caller must be on
// AppKit's main thread; platform code uses this only from event/callback
// boundaries after Go-side cancellation has already been recorded.
func UnmarkText(view ID) {
	if view == 0 || !goGPUViewTextInput {
		return
	}
	initSelectors()
	view.Send(selectors.unmarkText)
}

// NewAutoreleasedAttributedString creates an NSAttributedString suitable for
// returning from an NSTextInputClient callback. The temporary NSString and
// attributed wrapper are released/autoreleased on the AppKit pool boundary.
func NewAutoreleasedAttributedString(value string) ID {
	initSelectors()
	initClasses()
	if classes.NSAttributedString == 0 {
		return NewAutoreleasedNSString(value)
	}
	text := NewNSString(value)
	if text == nil || text.ID() == 0 {
		return 0
	}
	defer text.Release()
	allocated := ID(classes.NSAttributedString).Send(selectors.alloc)
	if allocated == 0 {
		return 0
	}
	object := allocated.SendPtr(selectors.initWithString, text.ID().Ptr())
	if object == 0 {
		// `alloc` returned a retained object but its initializer failed; release
		// that ownership before returning through the callback boundary.
		allocated.Send(selectors.release)
		return 0
	}
	object.Send(selectors.autorelease)
	return object
}

// NewAutoreleasedNSString creates an NSString suitable for returning from a
// pointer-returning Objective-C callback. NewNSString returns a retained
// object; sending autorelease transfers ownership back to AppKit's pool.
func NewAutoreleasedNSString(value string) ID {
	ns := NewNSString(value)
	if ns == nil || ns.ID() == 0 {
		return 0
	}
	id := ns.ID()
	id.Send(selectors.autorelease)
	return id
}

// ObjectString converts NSString/NSAttributedString objects passed by AppKit
// into a bounded Go UTF-8 string. The fallback through -string handles an
// NSAttributedString returned by a custom input manager.
func ObjectString(object ID) string {
	if object == 0 {
		return ""
	}
	initSelectors()
	value := object
	if object.SendPtr(selectors.respondsToSelector, selectors.UTF8String.SELPtr()) == 0 {
		if object.SendPtr(selectors.respondsToSelector, selectors.string.SELPtr()) == 0 {
			return ""
		}
		value = object.Send(selectors.string)
	}
	if value == 0 {
		return ""
	}
	ptr := NSStringUTF8Ptr(value)
	if ptr == 0 {
		return ""
	}
	units := NSStringLength(value)
	// NSStringLength is UTF-16 units; four bytes per unit is a conservative
	// UTF-8 bound. Keep malformed or hostile objects from causing huge slices.
	if units == 0 || units > 1<<20 {
		return ""
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(units)*4+1)
	for i, b := range data {
		if b == 0 {
			data = data[:i]
			break
		}
	}
	return string(data)
}
