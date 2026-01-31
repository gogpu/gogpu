// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

package gogpu

import (
	"math"
	"sync"
	"time"

	"github.com/gogpu/gpucontext"
)

// GestureRecognizer computes gestures from pointer events.
//
// Following Vello's per-frame pattern, this recognizer tracks active pointers
// and computes gesture deltas (zoom, rotation, translation) once at the end
// of each frame. This approach provides smooth, predictable gestures by
// avoiding jitter from individual pointer movements.
//
// Usage:
//
//	recognizer := NewGestureRecognizer()
//
//	// During event processing:
//	recognizer.HandlePointer(pointerEvent)
//
//	// At end of frame:
//	gesture := recognizer.EndFrame()
//	if gesture.NumPointers >= 2 {
//	    camera.ApplyGesture(gesture)
//	}
//
// Thread safety: GestureRecognizer is safe for concurrent use.
type GestureRecognizer struct {
	mu             sync.Mutex
	activePointers map[int]*pointerState
	gestureState   *gestureState
	touchesChanged bool
	lastTimestamp  time.Duration
}

// pointerState tracks the current and previous position of a pointer.
type pointerState struct {
	ID    int
	X, Y  float64
	PrevX float64
	PrevY float64
}

// gestureState tracks the previous frame's gesture values for delta calculation.
type gestureState struct {
	prevCenter   gpucontext.Point
	prevDistance float64
	prevAngle    float64
}

// NewGestureRecognizer creates a new GestureRecognizer.
func NewGestureRecognizer() *GestureRecognizer {
	return &GestureRecognizer{
		activePointers: make(map[int]*pointerState),
	}
}

// HandlePointer processes a pointer event and updates internal state.
//
// Call this for each pointer event received during the frame.
// The recognizer tracks pointer positions and computes gestures at EndFrame().
func (g *GestureRecognizer) HandlePointer(e gpucontext.PointerEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.lastTimestamp = e.Timestamp

	switch e.Type {
	case gpucontext.PointerDown:
		// New pointer - add to active set
		g.activePointers[e.PointerID] = &pointerState{
			ID:    e.PointerID,
			X:     e.X,
			Y:     e.Y,
			PrevX: e.X,
			PrevY: e.Y,
		}
		g.touchesChanged = true

	case gpucontext.PointerMove:
		// Update existing pointer position
		if p, ok := g.activePointers[e.PointerID]; ok {
			p.PrevX = p.X
			p.PrevY = p.Y
			p.X = e.X
			p.Y = e.Y
		}

	case gpucontext.PointerUp, gpucontext.PointerCancel:
		// Remove pointer from active set
		delete(g.activePointers, e.PointerID)
		g.touchesChanged = true
	}
}

// EndFrame computes gesture deltas and resets for the next frame.
//
// This should be called once at the end of each frame, after all pointer
// events have been processed. It returns a GestureEvent containing the
// computed deltas for zoom, rotation, and translation.
//
// If fewer than 2 pointers are active, returns an empty GestureEvent
// with NumPointers set to the current count.
func (g *GestureRecognizer) EndFrame() gpucontext.GestureEvent {
	g.mu.Lock()
	defer g.mu.Unlock()

	numPointers := len(g.activePointers)

	// Not enough pointers for a gesture
	if numPointers < 2 {
		g.gestureState = nil
		g.touchesChanged = false
		return gpucontext.GestureEvent{
			NumPointers: numPointers,
			ZoomDelta:   1.0, // No zoom change
			Timestamp:   g.lastTimestamp,
		}
	}

	// Calculate current centroid and average distance
	center := g.calculateCentroid()
	distance := g.calculateAverageDistance(center)
	angle := g.calculateAngle(center)
	pinchType := g.classifyPinchType()

	var result gpucontext.GestureEvent
	result.NumPointers = numPointers
	result.Center = center
	result.PinchType = pinchType
	result.Timestamp = g.lastTimestamp
	result.ZoomDelta = 1.0 // Default: no zoom

	// If we have previous state AND touches haven't changed, compute deltas
	if g.gestureState != nil && !g.touchesChanged {
		// Zoom delta: ratio of current distance to previous
		if g.gestureState.prevDistance > 0 {
			result.ZoomDelta = distance / g.gestureState.prevDistance
		}

		// Rotation delta: change in angle
		result.RotationDelta = angle - g.gestureState.prevAngle

		// Translation delta: change in centroid
		result.TranslationDelta = gpucontext.Point{
			X: center.X - g.gestureState.prevCenter.X,
			Y: center.Y - g.gestureState.prevCenter.Y,
		}

		// Calculate 2D zoom (non-proportional stretch)
		result.ZoomDelta2D = g.calculate2DZoomDelta()
	}

	// Update gesture state for next frame
	g.gestureState = &gestureState{
		prevCenter:   center,
		prevDistance: distance,
		prevAngle:    angle,
	}
	g.touchesChanged = false

	// Update previous positions for next frame
	for _, p := range g.activePointers {
		p.PrevX = p.X
		p.PrevY = p.Y
	}

	return result
}

// Reset clears all state.
//
// Call this when gestures should be canceled (e.g., on window blur).
func (g *GestureRecognizer) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.activePointers = make(map[int]*pointerState)
	g.gestureState = nil
	g.touchesChanged = false
}

// NumActivePointers returns the current number of active pointers.
func (g *GestureRecognizer) NumActivePointers() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.activePointers)
}

// calculateCentroid computes the average position of all active pointers.
func (g *GestureRecognizer) calculateCentroid() gpucontext.Point {
	if len(g.activePointers) == 0 {
		return gpucontext.Point{}
	}

	var sumX, sumY float64
	for _, p := range g.activePointers {
		sumX += p.X
		sumY += p.Y
	}

	n := float64(len(g.activePointers))
	return gpucontext.Point{
		X: sumX / n,
		Y: sumY / n,
	}
}

// calculateAverageDistance computes the average distance from centroid.
func (g *GestureRecognizer) calculateAverageDistance(center gpucontext.Point) float64 {
	if len(g.activePointers) == 0 {
		return 0
	}

	var totalDist float64
	for _, p := range g.activePointers {
		dx := p.X - center.X
		dy := p.Y - center.Y
		totalDist += math.Sqrt(dx*dx + dy*dy)
	}

	return totalDist / float64(len(g.activePointers))
}

// calculateAngle computes the angle from centroid to the first pointer.
// This is used for rotation detection.
func (g *GestureRecognizer) calculateAngle(center gpucontext.Point) float64 {
	// Find the pointer with the lowest ID (for consistency)
	var firstPointer *pointerState
	for _, p := range g.activePointers {
		if firstPointer == nil || p.ID < firstPointer.ID {
			firstPointer = p
		}
	}

	if firstPointer == nil {
		return 0
	}

	dx := firstPointer.X - center.X
	dy := firstPointer.Y - center.Y
	return math.Atan2(dy, dx)
}

// classifyPinchType determines the pinch type based on finger geometry.
func (g *GestureRecognizer) classifyPinchType() gpucontext.PinchType {
	if len(g.activePointers) < 2 {
		return gpucontext.PinchNone
	}

	// Calculate bounding box of all pointers
	minX, maxX, minY, maxY := g.calculateBoundingBox()

	dx := maxX - minX
	dy := maxY - minY

	// Classification: one axis dominates by 3x
	if dx > dy*3 {
		return gpucontext.PinchHorizontal
	}
	if dy > dx*3 {
		return gpucontext.PinchVertical
	}
	return gpucontext.PinchProportional
}

// calculateBoundingBox returns the bounding box of all active pointers.
func (g *GestureRecognizer) calculateBoundingBox() (minX, maxX, minY, maxY float64) {
	first := true
	for _, p := range g.activePointers {
		if first {
			minX, maxX = p.X, p.X
			minY, maxY = p.Y, p.Y
			first = false
			continue
		}
		minX, maxX = minMax(minX, maxX, p.X)
		minY, maxY = minMax(minY, maxY, p.Y)
	}
	return
}

// minMax updates minimum and maximum values with a new value.
func minMax(minVal, maxVal, val float64) (float64, float64) {
	if val < minVal {
		minVal = val
	}
	if val > maxVal {
		maxVal = val
	}
	return minVal, maxVal
}

// calculatePrevBoundingBox returns the bounding box of previous positions.
func (g *GestureRecognizer) calculatePrevBoundingBox() (minX, maxX, minY, maxY float64) {
	first := true
	for _, p := range g.activePointers {
		if first {
			minX, maxX = p.PrevX, p.PrevX
			minY, maxY = p.PrevY, p.PrevY
			first = false
			continue
		}
		minX, maxX = minMax(minX, maxX, p.PrevX)
		minY, maxY = minMax(minY, maxY, p.PrevY)
	}
	return
}

// calculate2DZoomDelta computes non-proportional zoom deltas.
func (g *GestureRecognizer) calculate2DZoomDelta() gpucontext.Point {
	if len(g.activePointers) < 2 || g.gestureState == nil {
		return gpucontext.Point{X: 1.0, Y: 1.0}
	}

	// Calculate current and previous bounding boxes
	minX, maxX, minY, maxY := g.calculateBoundingBox()
	prevMinX, prevMaxX, prevMinY, prevMaxY := g.calculatePrevBoundingBox()

	currentWidth := maxX - minX
	currentHeight := maxY - minY
	prevWidth := prevMaxX - prevMinX
	prevHeight := prevMaxY - prevMinY

	zoomX := 1.0
	zoomY := 1.0

	if prevWidth > 0 {
		zoomX = currentWidth / prevWidth
	}
	if prevHeight > 0 {
		zoomY = currentHeight / prevHeight
	}

	return gpucontext.Point{X: zoomX, Y: zoomY}
}
