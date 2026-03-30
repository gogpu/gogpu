//go:build linux

package wayland

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// CSDManager manages client-side decorations using 4 wl_subsurfaces.
// It creates and maintains title bar (top) and border (left/right/bottom)
// subsurfaces around the main content surface.
type CSDManager struct {
	display       *Display
	compositor    *WlCompositor
	subcompositor *WlSubcompositor
	shm           *WlShm
	parent        *WlSurface // main content surface
	painter       CSDPainter

	// Decoration subsurfaces
	top    *decorSurface
	left   *decorSurface
	right  *decorSurface
	bottom *decorSurface

	// Window dimensions (content area)
	contentWidth  int
	contentHeight int

	// Decoration state
	state CSDState
}

// decorSurface holds a decoration subsurface and its SHM buffer.
type decorSurface struct {
	surface    *WlSurface
	subsurface *WlSubsurface
	pool       *WlShmPool
	buffer     *WlBuffer
	fd         int    // shm file descriptor
	data       []byte // mmap'd pixel data
	width      int
	height     int
}

// NewCSDManager creates a CSD manager for the given parent surface.
func NewCSDManager(
	display *Display,
	compositor *WlCompositor,
	subcompositor *WlSubcompositor,
	shm *WlShm,
	parent *WlSurface,
	painter CSDPainter,
) *CSDManager {
	if painter == nil {
		painter = DefaultCSDPainter{}
	}
	return &CSDManager{
		display:       display,
		compositor:    compositor,
		subcompositor: subcompositor,
		shm:           shm,
		parent:        parent,
		painter:       painter,
		state:         CSDState{Focused: true},
	}
}

// TitleBarHeight returns the title bar height in pixels.
func (m *CSDManager) TitleBarHeight() int {
	return m.painter.TitleBarHeight()
}

// BorderWidth returns the side/bottom border width in pixels.
func (m *CSDManager) BorderWidth() int {
	return m.painter.BorderWidth()
}

// Create creates all 4 decoration subsurfaces and their initial buffers.
func (m *CSDManager) Create(contentWidth, contentHeight int, title string) error {
	m.contentWidth = contentWidth
	m.contentHeight = contentHeight
	m.state.Title = title

	tbH := m.painter.TitleBarHeight()
	bW := m.painter.BorderWidth()

	var err error

	// Top (title bar): full width including borders, title bar height
	totalW := contentWidth + bW*2
	m.top, err = m.createDecorSurface(totalW, tbH)
	if err != nil {
		return fmt.Errorf("csd: create top surface: %w", err)
	}
	// Position: top-left corner of the window (above and left of content)
	if err := m.top.subsurface.SetPosition(-int32(bW), -int32(tbH)); err != nil {
		return err
	}

	// Left border
	m.left, err = m.createDecorSurface(bW, contentHeight)
	if err != nil {
		return fmt.Errorf("csd: create left surface: %w", err)
	}
	if err := m.left.subsurface.SetPosition(-int32(bW), 0); err != nil {
		return err
	}

	// Right border
	m.right, err = m.createDecorSurface(bW, contentHeight)
	if err != nil {
		return fmt.Errorf("csd: create right surface: %w", err)
	}
	if err := m.right.subsurface.SetPosition(int32(contentWidth), 0); err != nil {
		return err
	}

	// Bottom border
	m.bottom, err = m.createDecorSurface(totalW, bW)
	if err != nil {
		return fmt.Errorf("csd: create bottom surface: %w", err)
	}
	if err := m.bottom.subsurface.SetPosition(-int32(bW), int32(contentHeight)); err != nil {
		return err
	}

	// Initial paint
	m.paint()

	return nil
}

// paint renders all decoration surfaces and commits them.
func (m *CSDManager) paint() {
	if m.top != nil {
		m.painter.PaintTitleBar(m.top.data, m.top.width, m.top.height, m.state)
		m.commitDecorSurface(m.top)
	}
	if m.left != nil {
		m.painter.PaintBorder(m.left.data, m.left.width, m.left.height, CSDEdgeLeft)
		m.commitDecorSurface(m.left)
	}
	if m.right != nil {
		m.painter.PaintBorder(m.right.data, m.right.width, m.right.height, CSDEdgeRight)
		m.commitDecorSurface(m.right)
	}
	if m.bottom != nil {
		m.painter.PaintBorder(m.bottom.data, m.bottom.width, m.bottom.height, CSDEdgeBottom)
		m.commitDecorSurface(m.bottom)
	}
}

// RepaintTitleBar repaints only the title bar (for hover/press state changes).
func (m *CSDManager) RepaintTitleBar() {
	if m.top == nil {
		return
	}
	m.painter.PaintTitleBar(m.top.data, m.top.width, m.top.height, m.state)
	m.commitDecorSurface(m.top)
}

// SetState updates the decoration state and repaints the title bar.
func (m *CSDManager) SetState(state CSDState) {
	m.state = state
	m.RepaintTitleBar()
}

// SetFocused updates the focused state.
func (m *CSDManager) SetFocused(focused bool) {
	m.state.Focused = focused
	m.RepaintTitleBar()
}

// HitTestTop performs hit-testing on the title bar subsurface.
func (m *CSDManager) HitTestTop(x, y int) CSDHitResult {
	if m.top == nil {
		return CSDHitNone
	}
	return m.painter.HitTestTitleBar(x, y, m.top.width, m.top.height)
}

// HitTestBorder performs hit-testing on a border subsurface.
func (m *CSDManager) HitTestBorder(edge CSDEdge, x, y, width, height int) CSDHitResult {
	corner := defaultCornerSize
	switch edge {
	case CSDEdgeLeft:
		if y < corner {
			return CSDHitResizeNW
		}
		if y >= height-corner {
			return CSDHitResizeSW
		}
		return CSDHitResizeW
	case CSDEdgeRight:
		if y < corner {
			return CSDHitResizeNE
		}
		if y >= height-corner {
			return CSDHitResizeSE
		}
		return CSDHitResizeE
	case CSDEdgeBottom:
		if x < corner {
			return CSDHitResizeSW
		}
		if x >= width-corner {
			return CSDHitResizeSE
		}
		return CSDHitResizeS
	}
	return CSDHitNone
}

// TopSurface returns the title bar wl_surface (for pointer event routing).
func (m *CSDManager) TopSurface() *WlSurface {
	if m.top == nil {
		return nil
	}
	return m.top.surface
}

// LeftSurface returns the left border wl_surface.
func (m *CSDManager) LeftSurface() *WlSurface {
	if m.left == nil {
		return nil
	}
	return m.left.surface
}

// RightSurface returns the right border wl_surface.
func (m *CSDManager) RightSurface() *WlSurface {
	if m.right == nil {
		return nil
	}
	return m.right.surface
}

// BottomSurface returns the bottom border wl_surface.
func (m *CSDManager) BottomSurface() *WlSurface {
	if m.bottom == nil {
		return nil
	}
	return m.bottom.surface
}

// State returns the current CSD state.
func (m *CSDManager) State() *CSDState {
	return &m.state
}

// Destroy destroys all decoration subsurfaces and frees resources.
func (m *CSDManager) Destroy() {
	m.destroyDecorSurface(m.top)
	m.destroyDecorSurface(m.left)
	m.destroyDecorSurface(m.right)
	m.destroyDecorSurface(m.bottom)
	m.top = nil
	m.left = nil
	m.right = nil
	m.bottom = nil
}

// --- Internal helpers ---

func (m *CSDManager) createDecorSurface(width, height int) (*decorSurface, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("csd: invalid dimensions %dx%d", width, height)
	}

	// Create wl_surface
	surface, err := m.compositor.CreateSurface()
	if err != nil {
		return nil, err
	}

	// Create wl_subsurface
	subsurface, err := m.subcompositor.GetSubsurface(surface, m.parent)
	if err != nil {
		surface.Destroy()
		return nil, err
	}

	// Sync mode — commits are atomic with parent
	if err := subsurface.SetSync(); err != nil {
		subsurface.Destroy()
		surface.Destroy()
		return nil, err
	}

	// Create SHM buffer
	stride := width * 4
	size := stride * height

	fd, err := createShmFD(size)
	if err != nil {
		subsurface.Destroy()
		surface.Destroy()
		return nil, fmt.Errorf("csd: create shm fd: %w", err)
	}

	data, err := unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Close(fd)
		subsurface.Destroy()
		surface.Destroy()
		return nil, fmt.Errorf("csd: mmap: %w", err)
	}

	pool, err := m.shm.CreatePool(fd, int32(size))
	if err != nil {
		unix.Munmap(data)
		unix.Close(fd)
		subsurface.Destroy()
		surface.Destroy()
		return nil, fmt.Errorf("csd: create pool: %w", err)
	}

	buffer, err := pool.CreateBuffer(0, int32(width), int32(height), int32(stride), ShmFormatARGB8888)
	if err != nil {
		pool.Destroy()
		unix.Munmap(data)
		unix.Close(fd)
		subsurface.Destroy()
		surface.Destroy()
		return nil, fmt.Errorf("csd: create buffer: %w", err)
	}

	return &decorSurface{
		surface:    surface,
		subsurface: subsurface,
		pool:       pool,
		buffer:     buffer,
		fd:         fd,
		data:       data,
		width:      width,
		height:     height,
	}, nil
}

func (m *CSDManager) commitDecorSurface(ds *decorSurface) {
	if ds == nil {
		return
	}
	_ = ds.surface.Attach(ds.buffer.ID(), 0, 0)
	_ = ds.surface.Damage(0, 0, int32(ds.width), int32(ds.height))
	_ = ds.surface.Commit()
}

func (m *CSDManager) destroyDecorSurface(ds *decorSurface) {
	if ds == nil {
		return
	}
	if ds.buffer != nil {
		ds.buffer.Destroy()
	}
	if ds.pool != nil {
		ds.pool.Destroy()
	}
	if ds.data != nil {
		unix.Munmap(ds.data)
	}
	if ds.fd > 0 {
		unix.Close(ds.fd)
	}
	if ds.subsurface != nil {
		ds.subsurface.Destroy()
	}
	if ds.surface != nil {
		ds.surface.Destroy()
	}
}

// createShmFD creates an anonymous shared memory file descriptor.
func createShmFD(size int) (int, error) {
	fd, err := unix.MemfdCreate("gogpu-csd", unix.MFD_CLOEXEC)
	if err != nil {
		return -1, err
	}
	if err := unix.Ftruncate(fd, int64(size)); err != nil {
		unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

// WlBuffer is defined in shm.go — reused here for CSD buffers.
