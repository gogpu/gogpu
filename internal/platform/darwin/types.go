//go:build darwin

package darwin

// CGFloat is the floating-point type used by Core Graphics.
// On 64-bit systems (all supported macOS versions), CGFloat is float64.
type CGFloat = float64

// CGPoint represents a point in a two-dimensional coordinate system.
type CGPoint struct {
	X CGFloat
	Y CGFloat
}

// CGSize represents the dimensions of a rectangle.
type CGSize struct {
	Width  CGFloat
	Height CGFloat
}

// CGRect represents a rectangle defined by origin and size.
// macOS uses a bottom-left origin coordinate system.
type CGRect struct {
	Origin CGPoint
	Size   CGSize
}

// NSPoint is an alias for CGPoint in AppKit.
type NSPoint = CGPoint

// NSSize is an alias for CGSize in AppKit.
type NSSize = CGSize

// NSRect is an alias for CGRect in AppKit.
type NSRect = CGRect

// NSUInteger is the unsigned integer type used by Cocoa.
// On 64-bit systems, NSUInteger is uint64.
type NSUInteger = uint64

// NSInteger is the signed integer type used by Cocoa.
// On 64-bit systems, NSInteger is int64.
type NSInteger = int64

// BOOL is the Objective-C boolean type.
// YES = 1, NO = 0.
type BOOL = int8

// Objective-C BOOL constants.
const (
	YES BOOL = 1
	NO  BOOL = 0
)

// NSWindowStyleMask specifies the style of a window and its capabilities.
type NSWindowStyleMask NSUInteger

// Window style mask values.
const (
	// NSWindowStyleMaskBorderless creates a window with no border or title bar.
	NSWindowStyleMaskBorderless NSWindowStyleMask = 0

	// NSWindowStyleMaskTitled creates a window with a title bar.
	NSWindowStyleMaskTitled NSWindowStyleMask = 1 << 0

	// NSWindowStyleMaskClosable allows the window to be closed.
	NSWindowStyleMaskClosable NSWindowStyleMask = 1 << 1

	// NSWindowStyleMaskMiniaturizable allows the window to be minimized.
	NSWindowStyleMaskMiniaturizable NSWindowStyleMask = 1 << 2

	// NSWindowStyleMaskResizable allows the window to be resized.
	NSWindowStyleMaskResizable NSWindowStyleMask = 1 << 3

	// NSWindowStyleMaskFullScreen enables fullscreen mode.
	NSWindowStyleMaskFullScreen NSWindowStyleMask = 1 << 14

	// NSWindowStyleMaskFullSizeContentView extends content under title bar.
	NSWindowStyleMaskFullSizeContentView NSWindowStyleMask = 1 << 15
)

// NSBackingStoreType specifies how the window buffer is stored.
type NSBackingStoreType NSUInteger

// Backing store types.
const (
	// NSBackingStoreBuffered uses a buffer for window contents.
	// This is the standard mode for modern macOS.
	NSBackingStoreBuffered NSBackingStoreType = 2
)

// NSEventMask specifies which events to receive.
type NSEventMask NSUInteger

// Event mask values.
const (
	// NSEventMaskAny matches any event type.
	NSEventMaskAny NSEventMask = ^NSEventMask(0)
)

// NSEventType specifies the type of an event.
type NSEventType NSUInteger

// Event types.
const (
	NSEventTypeLeftMouseDown  NSEventType = 1
	NSEventTypeLeftMouseUp    NSEventType = 2
	NSEventTypeRightMouseDown NSEventType = 3
	NSEventTypeRightMouseUp   NSEventType = 4
	NSEventTypeMouseMoved     NSEventType = 5
	NSEventTypeKeyDown        NSEventType = 10
	NSEventTypeKeyUp          NSEventType = 11
	NSEventTypeFlagsChanged   NSEventType = 12
	NSEventTypeScrollWheel    NSEventType = 22
)

// NSApplicationActivationPolicy specifies how an app is activated.
type NSApplicationActivationPolicy NSInteger

// Activation policies.
const (
	// NSApplicationActivationPolicyRegular is a regular app with dock icon.
	NSApplicationActivationPolicyRegular NSApplicationActivationPolicy = 0

	// NSApplicationActivationPolicyAccessory is an accessory app without dock icon.
	NSApplicationActivationPolicyAccessory NSApplicationActivationPolicy = 1

	// NSApplicationActivationPolicyProhibited is a background app.
	NSApplicationActivationPolicyProhibited NSApplicationActivationPolicy = 2
)

// MakeRect creates an NSRect from origin and size components.
func MakeRect(x, y, width, height CGFloat) NSRect {
	return NSRect{
		Origin: NSPoint{X: x, Y: y},
		Size:   NSSize{Width: width, Height: height},
	}
}

// MakePoint creates an NSPoint from coordinates.
func MakePoint(x, y CGFloat) NSPoint {
	return NSPoint{X: x, Y: y}
}

// MakeSize creates an NSSize from dimensions.
func MakeSize(width, height CGFloat) NSSize {
	return NSSize{Width: width, Height: height}
}
