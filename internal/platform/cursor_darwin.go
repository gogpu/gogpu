//go:build darwin

package platform

import (
	"math"

	"github.com/gogpu/gogpu/internal/platform/darwin"
	"github.com/gogpu/gpucontext"
)

// Cursor modes as the PlatformWindow interface passes them.
const (
	cursorModeNormal   = int(gpucontext.CursorModeNormal)
	cursorModeLocked   = int(gpucontext.CursorModeLocked)
	cursorModeConfined = int(gpucontext.CursorModeConfined)
)

// warpMatchTolerance is how far, in points, an event's location may be from a
// warp's target and still count as an event the warp came before. macOS may
// round the cursor's position; a wrong match within this distance is undone by
// the next event.
const warpMatchTolerance = 1.0

// pendingWarpLimit bounds the warps kept for events to match, should events
// stop matching them; the oldest go first.
const pendingWarpLimit = 32

// cursorSystem is what darwinCursor needs from macOS for one window. Points
// are Cocoa screen coordinates: origin at the bottom-left of the primary
// display, y up.
type cursorSystem interface {
	Hide()
	Unhide()
	// Associate links the cursor to the mouse, or with false detaches it:
	// the mouse still reports motion, but the cursor stays where it is.
	Associate(associated bool)
	Warp(point darwin.NSPoint)
	Location() darwin.NSPoint
	Focused() bool
	// ContentRect is the window's content area on screen.
	ContentRect() darwin.NSRect
}

// pointerMotion is a mouse move as darwinCursor reports it: where the pointer
// is, in window coordinates (origin at the content's top-left, y down), and,
// while Locked, how far the mouse moved.
type pointerMotion struct {
	X      float64
	Y      float64
	DeltaX float64
	DeltaY float64
}

// cursorWarp is a warp no event has been seen after yet.
type cursorWarp struct {
	// X and Y are the target in window coordinates.
	X float64
	Y float64
	// DeltaX and DeltaY are the distance moved, y down, which macOS adds to
	// the delta of the first event after the warp.
	DeltaX float64
	DeltaY float64
}

// darwinCursor applies one window's cursor mode on macOS.
//
// Both modes detach the cursor from the mouse, so the system never moves it;
// moves still carry the mouse's motion in their delta. Locked hides the
// cursor and parks it at the content's centre, and moves report the motion at
// that frozen position. Confined keeps the cursor visible and moves it itself,
// by each move's delta clamped to the content area: macOS has no call that
// confines the cursor, and letting the system move it and warping it back
// lets it show outside, or escape where no event arrives.
//
// Warps add their distance to the delta of the next event, so each one is
// remembered until an event shows it has seen it: a detached cursor sits
// exactly where the last warp put it, so an event's location names the last
// warp before it.
//
// The requested mode is in effect only while the window is focused; losing
// focus releases the cursor and regaining it takes it again. mode reports the
// mode in effect. The first move after focus returns loses its delta, which
// holds the motion made while the application was in the background.
//
// All methods run on the main thread.
type darwinCursor struct {
	system        cursorSystem
	requestedMode int
	effectiveMode int

	// lockedLocation is where the cursor was when Locked was first taken for
	// the current request, restored when the application releases the lock.
	// Taking the lock again on focus return keeps it.
	lockedLocation      darwin.NSPoint
	lockedLocationSaved bool

	// confinedX and confinedY are where Confined has put the cursor, in
	// window coordinates.
	confinedX float64
	confinedY float64

	// pendingWarps are the warps, oldest first, that no event has been seen
	// after yet.
	pendingWarps []cursorWarp

	// focused is the window's focus as update last saw it, once focusKnown.
	focused    bool
	focusKnown bool

	// staleDelta is set when the window regains focus: macOS puts the motion
	// made while the application was in the background into the first move
	// event after it is back, so that move's delta is dropped.
	staleDelta bool
}

// windowCursorSystem is the cursorSystem of a real macOS window.
type windowCursorSystem struct {
	window *darwin.Window
}

func (s windowCursorSystem) Hide()   { darwin.HideCursor() }
func (s windowCursorSystem) Unhide() { darwin.UnhideCursor() }

func (s windowCursorSystem) Associate(associated bool) {
	if err := darwin.AssociateCursorWithMouse(associated); err != nil {
		loggerPtr.Load().Warn("gogpu: CGAssociateMouseAndMouseCursorPosition failed", "err", err)
	}
}

func (s windowCursorSystem) Warp(point darwin.NSPoint) {
	if err := darwin.WarpCursor(point); err != nil {
		loggerPtr.Load().Warn("gogpu: CGWarpMouseCursorPosition failed", "err", err)
	}
}

func (s windowCursorSystem) Location() darwin.NSPoint   { return darwin.CursorLocation() }
func (s windowCursorSystem) Focused() bool              { return s.window.IsKeyWindow() }
func (s windowCursorSystem) ContentRect() darwin.NSRect { return s.window.ContentScreenRect() }

// setMode requests a cursor mode, applied now if the window is focused.
func (c *darwinCursor) setMode(mode int) {
	if mode != c.requestedMode {
		c.lockedLocationSaved = false
	}
	c.requestedMode = mode
	c.update()
}

// mode returns the cursor mode in effect: Normal while the window is not
// focused, whatever was requested.
func (c *darwinCursor) mode() int {
	return c.effectiveMode
}

// update brings the cursor in line with the requested mode and the window's
// focus. It is called when either changes.
func (c *darwinCursor) update() {
	focused := c.system.Focused()
	if focused && c.focusKnown && !c.focused {
		c.staleDelta = true
	}
	c.focused = focused
	c.focusKnown = true

	target := cursorModeNormal
	if focused {
		target = c.requestedMode
	}
	if target == c.effectiveMode {
		return
	}

	switch c.effectiveMode {
	case cursorModeLocked:
		// Put the cursor back only when the application lets it go; on focus
		// loss it stays where the user left the window.
		if focused {
			c.system.Warp(c.lockedLocation)
		}
		c.system.Associate(true)
		c.system.Unhide()
	case cursorModeConfined:
		c.system.Associate(true)
	}
	c.pendingWarps = c.pendingWarps[:0]

	c.effectiveMode = target

	switch target {
	case cursorModeLocked:
		if !c.lockedLocationSaved {
			c.lockedLocation = c.system.Location()
			c.lockedLocationSaved = true
		}
		c.warp(c.centre())
		c.system.Hide()
	case cursorModeConfined:
		content := c.system.ContentRect()
		location := c.system.Location()
		c.confinedX = location.X - content.Origin.X
		c.confinedY = content.Origin.Y + content.Size.Height - location.Y
		c.system.Associate(false)
		c.confine()
	}
}

// contentChanged keeps the cursor in the content area after the window was
// resized: Locked re-centres it, Confined brings it back inside.
func (c *darwinCursor) contentChanged() {
	switch c.effectiveMode {
	case cursorModeLocked:
		c.warp(c.centre())
	case cursorModeConfined:
		c.confine()
	}
}

// move turns a mouse move event into the motion to report. x and y are the
// event's window coordinates and deltaX, deltaY its delta, y down. reported
// is false when the move changes nothing: a Locked move without motion, or a
// Confined one against the edge.
func (c *darwinCursor) move(x, y, deltaX, deltaY float64) (motion pointerMotion, reported bool) {
	stale := c.staleDelta
	c.staleDelta = false
	if c.effectiveMode == cursorModeNormal {
		return pointerMotion{X: x, Y: y}, true
	}

	// Take out the warps this event has seen: those up to the one that put
	// the cursor where the event found it.
	seen := 0
	for i, warp := range c.pendingWarps {
		if math.Abs(warp.X-x) <= warpMatchTolerance && math.Abs(warp.Y-y) <= warpMatchTolerance {
			seen = i + 1
		}
	}
	for _, warp := range c.pendingWarps[:seen] {
		deltaX -= warp.DeltaX
		deltaY -= warp.DeltaY
	}
	c.pendingWarps = append(c.pendingWarps[:0], c.pendingWarps[seen:]...)
	if stale {
		deltaX = 0
		deltaY = 0
	}

	if c.effectiveMode == cursorModeLocked {
		motion.X, motion.Y = c.centre()
		motion.DeltaX = deltaX
		motion.DeltaY = deltaY
		return motion, deltaX != 0 || deltaY != 0
	}

	fromX := c.confinedX
	fromY := c.confinedY
	c.confinedX += deltaX
	c.confinedY += deltaY
	c.confine()
	return pointerMotion{X: c.confinedX, Y: c.confinedY}, c.confinedX != fromX || c.confinedY != fromY
}

// confine clamps the Confined position to the content area and warps the
// cursor there if it is not there already.
func (c *darwinCursor) confine() {
	size := c.system.ContentRect().Size
	c.confinedX = min(max(c.confinedX, 0), size.Width-1)
	c.confinedY = min(max(c.confinedY, 0), size.Height-1)
	if c.system.Location() != c.screenPoint(c.confinedX, c.confinedY) {
		c.warp(c.confinedX, c.confinedY)
	}
}

// warp moves the cursor to a window point, remembers it for the next event's
// delta, and detaches the cursor from the mouse. Detaching again right after
// the warp also ends the moment macOS otherwise freezes the cursor after one.
func (c *darwinCursor) warp(x, y float64) {
	from := c.system.Location()
	to := c.screenPoint(x, y)
	if len(c.pendingWarps) == pendingWarpLimit {
		c.pendingWarps = append(c.pendingWarps[:0], c.pendingWarps[1:]...)
	}
	c.pendingWarps = append(c.pendingWarps, cursorWarp{
		X:      x,
		Y:      y,
		DeltaX: to.X - from.X,
		DeltaY: from.Y - to.Y,
	})
	c.system.Warp(to)
	c.system.Associate(false)
}

// centre is the content area's centre in window coordinates.
func (c *darwinCursor) centre() (x, y float64) {
	size := c.system.ContentRect().Size
	return size.Width / 2, size.Height / 2
}

// screenPoint converts a window point to screen coordinates.
func (c *darwinCursor) screenPoint(x, y float64) darwin.NSPoint {
	content := c.system.ContentRect()
	return darwin.NSPoint{
		X: content.Origin.X + x,
		Y: content.Origin.Y + content.Size.Height - y,
	}
}
