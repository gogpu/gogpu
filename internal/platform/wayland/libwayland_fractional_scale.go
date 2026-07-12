//go:build linux

package wayland

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

var fractionalIfaces struct {
	once sync.Once

	scaleMgr   cWlInterface
	scale      cWlInterface
	viewporter cWlInterface
	viewport   cWlInterface

	scaleMgrMethods   [2]cWlMessage
	scaleMethods      [1]cWlMessage
	scaleEvents       [1]cWlMessage
	viewporterMethods [2]cWlMessage
	viewportMethods   [3]cWlMessage
	nullTypes         [4]uintptr
}

var (
	fractionalScaleListener [1]uintptr
	fractionalHandlesMu     sync.Mutex
	fractionalHandles       = map[uintptr]*LibwaylandHandle{}
)

func initFractionalScaleInterfaces() {
	fractionalIfaces.once.Do(func() {
		nt := uintptr(unsafe.Pointer(&fractionalIfaces.nullTypes[0]))

		fractionalIfaces.scaleMgrMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		fractionalIfaces.scaleMgrMethods[1] = cWlMessage{cstr("get_fractional_scale\x00"), cstr("no\x00"), nt}
		fractionalIfaces.scaleMgr = cWlInterface{
			Name:        cstr("wp_fractional_scale_manager_v1\x00"),
			Version:     1,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&fractionalIfaces.scaleMgrMethods[0])),
		}

		fractionalIfaces.scaleMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		fractionalIfaces.scaleEvents[0] = cWlMessage{cstr("preferred_scale\x00"), cstr("u\x00"), nt}
		fractionalIfaces.scale = cWlInterface{
			Name:        cstr("wp_fractional_scale_v1\x00"),
			Version:     1,
			MethodCount: 1,
			Methods:     uintptr(unsafe.Pointer(&fractionalIfaces.scaleMethods[0])),
			EventCount:  1,
			Events:      uintptr(unsafe.Pointer(&fractionalIfaces.scaleEvents[0])),
		}

		fractionalIfaces.viewporterMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		fractionalIfaces.viewporterMethods[1] = cWlMessage{cstr("get_viewport\x00"), cstr("no\x00"), nt}
		fractionalIfaces.viewporter = cWlInterface{
			Name:        cstr("wp_viewporter\x00"),
			Version:     1,
			MethodCount: 2,
			Methods:     uintptr(unsafe.Pointer(&fractionalIfaces.viewporterMethods[0])),
		}

		fractionalIfaces.viewportMethods[0] = cWlMessage{cstr("destroy\x00"), cstr("\x00"), nt}
		fractionalIfaces.viewportMethods[1] = cWlMessage{cstr("set_source\x00"), cstr("ffff\x00"), nt}
		fractionalIfaces.viewportMethods[2] = cWlMessage{cstr("set_destination\x00"), cstr("ii\x00"), nt}
		fractionalIfaces.viewport = cWlInterface{
			Name:        cstr("wp_viewport\x00"),
			Version:     1,
			MethodCount: 3,
			Methods:     uintptr(unsafe.Pointer(&fractionalIfaces.viewportMethods[0])),
		}

		fractionalScaleListener[0] = ffi.NewCallback(fractionalPreferredScaleCb)
	})
}

func fractionalPreferredScaleCb(_, scaleObj, scale uintptr) {
	fractionalHandlesMu.Lock()
	h := fractionalHandles[scaleObj]
	fractionalHandlesMu.Unlock()
	if h == nil {
		return
	}
	h.scaleMu.Lock()
	h.fractionalScale = float64(scale) / 120.0
	h.scaleMu.Unlock()
}

func (h *LibwaylandHandle) SetupFractionalScale(scaleName, scaleVersion, viewporterName, viewporterVersion uint32) error {
	if scaleName == 0 || viewporterName == 0 {
		return nil
	}
	initFractionalScaleInterfaces()

	if scaleVersion > 1 {
		scaleVersion = 1
	}
	scaleMgr, err := h.registryBind(scaleName, unsafe.Pointer(&fractionalIfaces.scaleMgr), scaleVersion)
	if err != nil {
		return fmt.Errorf("wayland: bind fractional scale manager: %w", err)
	}
	h.fractionalScaleMgr = scaleMgr

	scaleObj, err := h.marshalConstructorObj(scaleMgr, 1, unsafe.Pointer(&fractionalIfaces.scale), h.surface)
	if err != nil {
		return fmt.Errorf("wayland: create fractional scale: %w", err)
	}
	h.fractionalScaleObj = scaleObj
	if err := h.addListener(scaleObj, uintptr(unsafe.Pointer(&fractionalScaleListener[0]))); err != nil {
		return fmt.Errorf("wayland: add fractional scale listener: %w", err)
	}
	fractionalHandlesMu.Lock()
	fractionalHandles[scaleObj] = h
	fractionalHandlesMu.Unlock()

	if viewporterVersion > 1 {
		viewporterVersion = 1
	}
	viewporter, err := h.registryBind(viewporterName, unsafe.Pointer(&fractionalIfaces.viewporter), viewporterVersion)
	if err != nil {
		return fmt.Errorf("wayland: bind viewporter: %w", err)
	}
	h.viewporter = viewporter
	viewport, err := h.marshalConstructorObj(viewporter, 1, unsafe.Pointer(&fractionalIfaces.viewport), h.surface)
	if err != nil {
		return fmt.Errorf("wayland: create viewport: %w", err)
	}
	h.viewport = viewport

	if err := h.flush(); err != nil {
		return err
	}
	return h.roundtrip()
}

func (h *LibwaylandHandle) FractionalScale() float64 {
	h.scaleMu.Lock()
	defer h.scaleMu.Unlock()
	return h.fractionalScale
}

func (h *LibwaylandHandle) SetViewportDestination(width, height int32) {
	if h.viewport == 0 || width <= 0 || height <= 0 {
		return
	}
	if h.viewportDestW == width && h.viewportDestH == height {
		return
	}
	h.viewportDestW = width
	h.viewportDestH = height
	h.marshalVoid(h.viewport, 2, uintptr(width), uintptr(height))
}
