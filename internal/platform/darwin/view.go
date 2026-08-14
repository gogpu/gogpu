//go:build darwin

package darwin

import (
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

// GoGPUView is a custom NSView subclass that overrides keyDown:, keyUp:,
// doCommandBySelector:, and acceptsFirstResponder. This prevents the macOS
// system beep (NSBeep) that occurs when the default NSView receives unhandled
// keyboard events.
//
// It also implements the NSDraggingDestination protocol for file drag-and-drop.
// Enterprise references: winit (WinitView), SDL3 (SDL3Window), GLFW (GLFWContentView).
//
// See ADR-015 for full analysis.

var (
	goGPUViewClass     Class
	goGPUViewSuper     Class
	goGPUViewTextInput bool
	goGPUViewClassOnce sync.Once
	errGoGPUViewClass  error
)

// NSDragOperation constants from AppKit/NSDragging.h.
const (
	// NSDragOperationNone indicates no drag operation is supported.
	NSDragOperationNone uintptr = 0
	// NSDragOperationCopy indicates the data will be copied.
	NSDragOperationCopy uintptr = 1
	// NSDragOperationGeneric indicates a generic drag operation.
	NSDragOperationGeneric uintptr = 4
)

// DragEvent describes a drag-and-drop event from macOS NSDragging protocol.
// The platform layer converts these into platform.Event entries.
type DragEvent struct {
	// Type is the drag event kind.
	Type DragEventType
	// X is the drop/hover x position in logical view coordinates (macOS Y-up origin).
	// The platform layer is responsible for Y-flipping to top-left origin.
	X float64
	// Y is the drop/hover y position in logical view coordinates (macOS Y-up origin).
	Y float64
	// Paths contains file paths for DragEventEnter and DragEventDrop events.
	Paths []string
}

// DragEventType identifies the kind of drag event.
type DragEventType int

const (
	// DragEventEnter fires when files enter the view area.
	DragEventEnter DragEventType = iota
	// DragEventMove fires as files move over the view.
	DragEventMove
	// DragEventDrop fires when files are dropped on the view.
	DragEventDrop
	// DragEventLeave fires when files leave the view area.
	DragEventLeave
)

// DragHandler is called by GoGPUView NSDragging callbacks to report drag events.
// Installed via SetViewDragHandler. Called on the main thread.
type DragHandler func(event DragEvent)

// viewDragHandlers maps view pointers to their DragHandler functions.
// We cannot store Go function pointers in ObjC associated objects directly,
// so we maintain a Go-side map keyed by the view's uintptr identity.
// Protected by viewDragMu. Entries are removed when the view is destroyed.
var (
	viewDragHandlers = make(map[uintptr]DragHandler)
	viewDragMu       sync.RWMutex
)

// TextInputHandler is the Go side of the NSTextInputClient bridge installed
// on each GoGPUView. AppKit invokes these callbacks on the main thread while
// interpreting a key event. NSRange values are passed as UTF-16 location and
// length pairs because that is the Cocoa ABI; the platform layer converts
// them to gpucontext's UTF-8 byte offsets before queueing events.
type TextInputHandler struct {
	KeyDown func(event ID)

	SetMarkedText func(text ID, selectedLocation, selectedLength,
		replacementLocation, replacementLength uintptr)
	InsertText func(text ID, replacementLocation, replacementLength uintptr)
	UnmarkText func()

	// AttributedSubstring returns an autoreleased NSAttributedString.
	// actualRange is a pointer to Cocoa's NSRange storage and may be zero.
	AttributedSubstring func(location, length, actualRange uintptr) ID
}

var (
	viewTextInputHandlers = make(map[uintptr]TextInputHandler)
	viewTextInputForward  = make(map[uintptr]int)
	viewTextInputMu       sync.RWMutex
)

// SetViewTextInputHandler installs the NSTextInputClient bridge callbacks for
// a view. Passing a zero handler removes the entry and releases the Go closure
// from the view-side registry; the ObjC class itself remains process-lived.
func SetViewTextInputHandler(view ID, handler TextInputHandler) {
	viewTextInputMu.Lock()
	defer viewTextInputMu.Unlock()
	if handler.KeyDown == nil && handler.SetMarkedText == nil &&
		handler.InsertText == nil && handler.UnmarkText == nil &&
		handler.AttributedSubstring == nil {
		delete(viewTextInputHandlers, view.Ptr())
		delete(viewTextInputForward, view.Ptr())
		return
	}
	viewTextInputHandlers[view.Ptr()] = handler
}

// ClearViewTextInputHandler removes the bridge callbacks for a destroyed
// window content view. Call this before releasing the underlying ObjC object.
func ClearViewTextInputHandler(view ID) {
	SetViewTextInputHandler(view, TextInputHandler{})
}

func getViewTextInputHandler(viewPtr uintptr) (TextInputHandler, bool) {
	viewTextInputMu.RLock()
	defer viewTextInputMu.RUnlock()
	h, ok := viewTextInputHandlers[viewPtr]
	return h, ok
}

func isViewTextInputForwarding(viewPtr uintptr) bool {
	viewTextInputMu.RLock()
	defer viewTextInputMu.RUnlock()
	return viewTextInputForward[viewPtr] != 0
}

// SetViewDragHandler installs a DragHandler on a GoGPUView instance.
// The handler receives drag events from macOS NSDragging protocol callbacks.
// Pass nil to remove the handler.
func SetViewDragHandler(view ID, handler DragHandler) {
	viewDragMu.Lock()
	defer viewDragMu.Unlock()

	if handler == nil {
		delete(viewDragHandlers, view.Ptr())
	} else {
		viewDragHandlers[view.Ptr()] = handler
	}
}

// getViewDragHandler returns the DragHandler for a GoGPUView instance, or nil.
func getViewDragHandler(viewPtr uintptr) DragHandler {
	viewDragMu.RLock()
	defer viewDragMu.RUnlock()

	return viewDragHandlers[viewPtr]
}

// ClearViewDragHandler removes the drag handler for a view.
// Call this when the window is destroyed to prevent leaks.
func ClearViewDragHandler(view ID) {
	SetViewDragHandler(view, nil)
}

// GoGPUViewClass returns the registered GoGPUView ObjC class.
// The class is created once and reused for all windows.
func GoGPUViewClass() (Class, error) {
	goGPUViewClassOnce.Do(func() {
		goGPUViewClass, errGoGPUViewClass = registerGoGPUViewClass()
	})
	return goGPUViewClass, errGoGPUViewClass
}

func registerGoGPUViewClass() (Class, error) {
	if err := initRuntime(); err != nil {
		return 0, err
	}

	// NSTextView already provides the struct-returning portions of
	// NSTextInputClient (markedRange, selectedRange, firstRect, and the
	// character-coordinate methods). Go callbacks cannot portably return
	// NSRange/NSRect on every supported macOS ABI, so the custom class extends
	// NSTextView and observes only the void/pointer-returning methods below.
	// The NSView fallback preserves window creation if AppKit class lookup is
	// unavailable during an early runtime probe.
	superClass := GetClass("NSTextView")
	textInput := superClass != 0
	if superClass == 0 {
		superClass = GetClass("NSView")
	}
	if superClass == 0 {
		return 0, ErrClassNotFound
	}

	cls := AllocateClassPair(superClass, "GoGPUView")
	if cls == 0 {
		return 0, ErrClassNotFound
	}
	goGPUViewSuper = superClass
	goGPUViewTextInput = textInput

	// keyDown: — handle keyboard event, prevent default NSBeep chain.
	// ObjC signature: -(void)keyDown:(NSEvent*)event → "v@:@"
	keyDownIMP := ffi.NewCallback(func(self, sel, event uintptr) uintptr {
		if handler, ok := getViewTextInputHandler(self); ok && handler.KeyDown != nil {
			handler.KeyDown(ID(event))
		}
		// Event dispatch is intentionally not forwarded to NSTextView's
		// keyDown:. The bridge calls interpretKeyEvents: explicitly when IME
		// input is enabled; disabled input follows the existing platform event
		// path and cannot produce duplicate committed text.
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("keyDown:"), keyDownIMP, "v@:@")

	// keyUp: — same pattern, prevent super from running.
	keyUpIMP := ffi.NewCallback(func(self, sel, event uintptr) uintptr {
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("keyUp:"), keyUpIMP, "v@:@")

	// flagsChanged: — modifier key events (Shift, Cmd, etc.)
	flagsChangedIMP := ffi.NewCallback(func(self, sel, event uintptr) uintptr {
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("flagsChanged:"), flagsChangedIMP, "v@:@")

	// doCommandBySelector: — no-op. This is the method that calls NSBeep()
	// when no responder handles the command. GLFW uses empty {}, SDL3 same.
	doCommandIMP := ffi.NewCallback(func(self, sel, aSelector uintptr) uintptr {
		return 0
	})
	ClassAddMethod(cls, RegisterSelector("doCommandBySelector:"), doCommandIMP, "v@::")

	// acceptsFirstResponder — return YES so the view receives key events.
	acceptsIMP := ffi.NewCallback(func(self, sel uintptr) uintptr {
		return 1 // YES
	})
	ClassAddMethod(cls, RegisterSelector("acceptsFirstResponder"), acceptsIMP, "B@:")

	if textInput {
		// NSTextInputClient mutators. NSRange is two NSUInteger values in the
		// Cocoa ABI, so the callback receives location/length pairs rather than
		// a Go struct (goffi callbacks intentionally restrict struct arguments
		// on arm64).
		setMarkedTextIMP := ffi.NewCallback(func(self, sel, text, selectedLocation,
			selectedLength, replacementLocation, replacementLength uintptr) uintptr {
			textID := ID(text)
			if !isViewTextInputForwarding(self) {
				if handler, ok := getViewTextInputHandler(self); ok && handler.SetMarkedText != nil {
					handler.SetMarkedText(textID, selectedLocation, selectedLength,
						replacementLocation, replacementLength)
				}
			}
			callGoGPUViewSuperVoid(
				ID(self),
				"setMarkedText:selectedRange:replacementRange:",
				&textID,
				[2]uintptr{selectedLocation, selectedLength},
				[2]uintptr{replacementLocation, replacementLength},
			)
			return 0
		})
		ClassAddMethod(cls, RegisterSelector("setMarkedText:selectedRange:replacementRange:"),
			setMarkedTextIMP, "v@:@{_NSRange=QQ}{_NSRange=QQ}")

		// Older input methods still send the two-argument spelling. Keep it as
		// a compatibility shim and forward it to NSTextView's implementation.
		setMarkedTextLegacyIMP := ffi.NewCallback(func(self, sel, text,
			selectedLocation, selectedLength uintptr) uintptr {
			textID := ID(text)
			if !isViewTextInputForwarding(self) {
				if handler, ok := getViewTextInputHandler(self); ok && handler.SetMarkedText != nil {
					handler.SetMarkedText(textID, selectedLocation, selectedLength,
						^uintptr(0), ^uintptr(0))
				}
			}
			callGoGPUViewSuperVoid(
				ID(self),
				"setMarkedText:selectedRange:",
				&textID,
				[2]uintptr{selectedLocation, selectedLength},
			)
			return 0
		})
		ClassAddMethod(cls, RegisterSelector("setMarkedText:selectedRange:"),
			setMarkedTextLegacyIMP, "v@:@{_NSRange=QQ}")

		insertTextIMP := ffi.NewCallback(func(self, sel, text,
			replacementLocation, replacementLength uintptr) uintptr {
			textID := ID(text)
			if !isViewTextInputForwarding(self) {
				if handler, ok := getViewTextInputHandler(self); ok && handler.InsertText != nil {
					handler.InsertText(textID, replacementLocation, replacementLength)
				}
			}
			callGoGPUViewSuperVoid(
				ID(self),
				"insertText:replacementRange:",
				&textID,
				[2]uintptr{replacementLocation, replacementLength},
			)
			return 0
		})
		ClassAddMethod(cls, RegisterSelector("insertText:replacementRange:"),
			insertTextIMP, "v@:@{_NSRange=QQ}")

		// Compatibility with the pre-10.6 insertText: spelling.
		insertTextLegacyIMP := ffi.NewCallback(func(self, sel, text uintptr) uintptr {
			textID := ID(text)
			if !isViewTextInputForwarding(self) {
				if handler, ok := getViewTextInputHandler(self); ok && handler.InsertText != nil {
					handler.InsertText(textID, ^uintptr(0), ^uintptr(0))
				}
			}
			callGoGPUViewSuperVoid(ID(self), "insertText:", &textID)
			return 0
		})
		ClassAddMethod(cls, RegisterSelector("insertText:"), insertTextLegacyIMP, "v@:@")

		unmarkTextIMP := ffi.NewCallback(func(self, sel uintptr) uintptr {
			if !isViewTextInputForwarding(self) {
				if handler, ok := getViewTextInputHandler(self); ok && handler.UnmarkText != nil {
					handler.UnmarkText()
				}
			}
			callGoGPUViewSuperVoid(ID(self), "unmarkText", nil)
			return 0
		})
		ClassAddMethod(cls, RegisterSelector("unmarkText"), unmarkTextIMP, "v@:")

		// Surrounding text is supplied by the focused widget rather than by
		// NSTextView's internal text storage. This callback returns an
		// autoreleased NSString, which is a valid NSAttributedString result.
		attributedSubstringIMP := ffi.NewCallback(func(self, sel, location,
			length, actualRange uintptr) uintptr {
			if handler, ok := getViewTextInputHandler(self); ok && handler.AttributedSubstring != nil {
				return uintptr(handler.AttributedSubstring(location, length, actualRange))
			}
			return 0
		})
		ClassAddMethod(cls, RegisterSelector("attributedSubstringForProposedRange:actualRange:"),
			attributedSubstringIMP, "@@:{_NSRange=QQ}^{_NSRange=QQ}")
	}

	// --- NSDraggingDestination protocol methods ---
	// These handle OS file drag-and-drop onto the view.
	// Reference: winit WinitView, SDL3 SDL3Window, GLFW GLFWContentView.

	// draggingEntered: — fires when a drag enters the view bounds.
	// ObjC: -(NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender
	// Type encoding: "Q@:@" (returns NSUInteger/NSDragOperation, self, _cmd, id)
	// Note: NSDragOperation is NSUInteger (uint64 on 64-bit), ObjC encoding "Q".
	draggingEnteredIMP := ffi.NewCallback(func(self, sel, sender uintptr) uintptr {
		handler := getViewDragHandler(self)
		if handler == nil {
			return NSDragOperationNone
		}

		loc := getDraggingLocation(ID(sender))
		paths := getFilenamesFromDraggingInfo(ID(sender))

		handler(DragEvent{
			Type:  DragEventEnter,
			X:     loc.X,
			Y:     loc.Y,
			Paths: paths,
		})

		return NSDragOperationCopy
	})
	ClassAddMethod(cls, RegisterSelector("draggingEntered:"), draggingEnteredIMP, "Q@:@")

	// draggingUpdated: — fires as the drag moves over the view.
	// ObjC: -(NSDragOperation)draggingUpdated:(id<NSDraggingInfo>)sender
	draggingUpdatedIMP := ffi.NewCallback(func(self, sel, sender uintptr) uintptr {
		handler := getViewDragHandler(self)
		if handler == nil {
			return NSDragOperationNone
		}

		loc := getDraggingLocation(ID(sender))

		handler(DragEvent{
			Type: DragEventMove,
			X:    loc.X,
			Y:    loc.Y,
		})

		return NSDragOperationCopy
	})
	ClassAddMethod(cls, RegisterSelector("draggingUpdated:"), draggingUpdatedIMP, "Q@:@")

	// draggingExited: — fires when the drag leaves the view bounds.
	// ObjC: -(void)draggingExited:(id<NSDraggingInfo>)sender
	draggingExitedIMP := ffi.NewCallback(func(self, sel, sender uintptr) uintptr {
		handler := getViewDragHandler(self)
		if handler == nil {
			return 0
		}

		handler(DragEvent{Type: DragEventLeave})

		return 0
	})
	ClassAddMethod(cls, RegisterSelector("draggingExited:"), draggingExitedIMP, "v@:@")

	// prepareForDragOperation: — return YES to accept the drop.
	// ObjC: -(BOOL)prepareForDragOperation:(id<NSDraggingInfo>)sender
	prepareForDragIMP := ffi.NewCallback(func(self, sel, sender uintptr) uintptr {
		return 1 // YES — always accept drops
	})
	ClassAddMethod(cls, RegisterSelector("prepareForDragOperation:"), prepareForDragIMP, "B@:@")

	// performDragOperation: — the actual drop. Extract file paths and emit event.
	// ObjC: -(BOOL)performDragOperation:(id<NSDraggingInfo>)sender
	performDragIMP := ffi.NewCallback(func(self, sel, sender uintptr) uintptr {
		handler := getViewDragHandler(self)
		if handler == nil {
			return 0 // NO
		}

		loc := getDraggingLocation(ID(sender))
		paths := getFilenamesFromDraggingInfo(ID(sender))
		if len(paths) == 0 {
			return 0 // NO — nothing to drop
		}

		handler(DragEvent{
			Type:  DragEventDrop,
			X:     loc.X,
			Y:     loc.Y,
			Paths: paths,
		})

		return 1 // YES
	})
	ClassAddMethod(cls, RegisterSelector("performDragOperation:"), performDragIMP, "B@:@")

	// --- NSDraggingSource protocol methods (outgoing drag) ---

	// draggingSession:sourceOperationMaskForDraggingContext:
	// Tells AppKit which drag operations we support as a source.
	sourceOpMaskIMP := ffi.NewCallback(func(self, sel, session, context uintptr) uintptr {
		return 1 | 16 // NSDragOperationCopy | NSDragOperationMove
	})
	ClassAddMethod(cls,
		RegisterSelector("draggingSession:sourceOperationMaskForDraggingContext:"),
		sourceOpMaskIMP, "Q@:@q")

	// draggingSession:endedAtPoint:operation:
	// Called when an outgoing drag session ends. Provides the actual operation
	// (Copy/Move/None) so we can relay it to the Go callback.
	// NSPoint (two doubles) passed by value — we ignore point coords, only need operation.
	dragEndedIMP := ffi.NewCallback(func(self, sel, session uintptr, ptX, ptY float64, operation uintptr) uintptr {
		cb := getDragSourceCallback(self)
		if cb != nil {
			cb(DragOperation(operation))
		}
		return 0
	})
	ClassAddMethod(cls,
		RegisterSelector("draggingSession:endedAtPoint:operation:"),
		dragEndedIMP, "v@:@{CGPoint=dd}Q")

	RegisterClassPair(cls)
	return cls, nil
}

// RegisterForDraggedTypes registers file drop types on a view.
// This must be called after the view is created to enable NSDragging protocol.
// Uses NSFilenamesPboardType (legacy but universally supported on all macOS versions)
// and NSPasteboardTypeFileURL (modern, macOS 10.13+).
func RegisterForDraggedTypes(view ID) {
	if view.IsNil() {
		return
	}
	initSelectors()
	initClasses()

	// Build an NSArray containing the pasteboard type strings.
	// NSFilenamesPboardType = "NSFilenamesPboardType" (legacy, works on all macOS)
	// NSPasteboardTypeFileURL = "public.file-url" (modern, macOS 10.13+)
	legacyType := NewNSString("NSFilenamesPboardType")
	modernType := NewNSString("public.file-url")

	if legacyType == nil || modernType == nil {
		if legacyType != nil {
			legacyType.Release()
		}
		if modernType != nil {
			modernType.Release()
		}
		return
	}
	defer legacyType.Release()
	defer modernType.Release()

	// Create NSArray with the two types using [NSArray arrayWithObjects:count:]
	nsArrayClass := GetClass("NSArray")
	if nsArrayClass == 0 {
		return
	}

	// Build a C array of two object pointers for arrayWithObjects:count:
	objects := [2]uintptr{uintptr(legacyType.ID()), uintptr(modernType.ID())}
	arrayWithObjectsCount := RegisterSelector("arrayWithObjects:count:")

	// Call [NSArray arrayWithObjects:&objects count:2]
	typesArray := msgSend(ID(nsArrayClass), arrayWithObjectsCount,
		uintptr(unsafe.Pointer(&objects[0])), 2)
	if typesArray.IsNil() {
		return
	}

	// [view registerForDraggedTypes:typesArray]
	view.SendPtr(selectors.registerForDraggedTypes, typesArray.Ptr())
}

// getDraggingLocation extracts the drag position from an NSDraggingInfo sender.
// Returns coordinates in the view's coordinate system (macOS Y-up origin).
func getDraggingLocation(sender ID) NSPoint {
	if sender.IsNil() {
		return NSPoint{}
	}
	initSelectors()
	return sender.GetPoint(selectors.draggingLocation)
}

// getFilenamesFromDraggingInfo extracts file paths from NSDraggingInfo's pasteboard.
// Uses [pasteboard propertyListForType:NSFilenamesPboardType] which returns an
// NSArray<NSString*> of file paths. This is the SDL3 pattern, compatible with all
// macOS versions.
func getFilenamesFromDraggingInfo(sender ID) []string {
	if sender.IsNil() {
		return nil
	}
	initSelectors()

	// [sender draggingPasteboard]
	pasteboard := sender.Send(selectors.draggingPasteboard)
	if pasteboard.IsNil() {
		return nil
	}

	// [pasteboard propertyListForType:@"NSFilenamesPboardType"]
	typeStr := NewNSString("NSFilenamesPboardType")
	if typeStr == nil {
		return nil
	}
	defer typeStr.Release()

	fileList := pasteboard.SendPtr(selectors.propertyListForType, uintptr(typeStr.ID()))
	if fileList.IsNil() {
		return nil
	}

	// fileList is an NSArray<NSString*>. Extract count and iterate.
	count := fileList.GetUint64(selectors.count)
	if count == 0 {
		return nil
	}

	paths := make([]string, 0, count)
	for i := uint64(0); i < count; i++ {
		// [fileList objectAtIndex:i]
		nsStr := fileList.SendUint(selectors.objectAtIndex, i)
		if nsStr.IsNil() {
			continue
		}

		path := nsStringToGoString(nsStr)
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// nsStringToGoString converts an NSString to a Go string.
func nsStringToGoString(nsStr ID) string {
	if nsStr.IsNil() {
		return ""
	}
	initSelectors()

	utf8Ptr := NSStringUTF8Ptr(nsStr)
	if utf8Ptr == 0 {
		return ""
	}

	length := NSStringLength(nsStr)
	if length == 0 {
		return ""
	}

	// Read UTF-8 bytes. length is character count; UTF-8 may use up to 4 bytes per char.
	data := unsafe.Slice((*byte)(unsafe.Pointer(utf8Ptr)), length*4) //nolint:govet // ObjC UTF8String pointer, bounded by NSString length

	// Find actual end of the C string (null terminator).
	end := 0
	for end < len(data) && data[end] != 0 {
		end++
	}

	return string(data[:end])
}

// CreateGoGPUView creates an instance of GoGPUView with the given frame rect.
// The returned ID is an allocated, initialized AppKit view subclass instance.
// The view is automatically registered to accept file drag-and-drop.
func CreateGoGPUView(frame NSRect) (ID, error) {
	cls, err := GoGPUViewClass()
	if err != nil {
		return 0, err
	}

	// [[GoGPUView alloc] initWithFrame:frame]
	alloc := ID(cls).Send(RegisterSelector("alloc"))
	if alloc.IsNil() {
		return 0, ErrViewCreationFailed
	}

	view := alloc.SendRect(RegisterSelector("initWithFrame:"), frame)
	if view.IsNil() {
		return 0, ErrViewCreationFailed
	}

	if goGPUViewTextInput {
		// NSTextView defaults to an opaque background and renders its internal
		// text storage. GoGPU owns the pixels and draws preedit text itself, so
		// keep the native text view as an invisible input/geometry surface while
		// retaining its NSTextInputClient state for AppKit's input manager.
		initSelectors()
		initClasses()
		view.SendBool(selectors.setDrawsBackground, false)
		if clearColor := classes.NSColor.Send(selectors.clearColor); clearColor != 0 {
			view.SendPtr(selectors.setTextColor, clearColor.Ptr())
		}
	}

	// Register the view to accept file drag-and-drop.
	RegisterForDraggedTypes(view)

	return view, nil
}
