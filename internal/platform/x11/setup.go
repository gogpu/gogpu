//go:build linux

package x11

import (
	"errors"
	"fmt"
)

// Setup response status codes.
const (
	SetupFailed    = 0
	SetupSuccess   = 1
	SetupAuthenticate = 2
)

// SetupInfo contains information from the X server setup response.
type SetupInfo struct {
	// Protocol version
	ProtocolMajorVersion uint16
	ProtocolMinorVersion uint16

	// Vendor information
	Vendor       string
	VendorLength uint16
	ReleaseNumber uint32

	// Resource ID allocation
	ResourceIDBase uint32
	ResourceIDMask uint32

	// Limits
	MaxRequestLength   uint16
	MotionBufferSize   uint32
	MaxKeycode         uint8
	MinKeycode         uint8
	ImageByteOrder     uint8
	BitmapBitOrder     uint8
	BitmapScanlineUnit uint8
	BitmapScanlinePad  uint8

	// Screens
	Screens []ScreenInfo

	// Pixmap formats
	PixmapFormats []PixmapFormat
}

// ScreenInfo contains information about an X screen.
type ScreenInfo struct {
	Root                ResourceID
	DefaultColormap     ResourceID
	WhitePixel          uint32
	BlackPixel          uint32
	CurrentInputMasks   uint32
	WidthInPixels       uint16
	HeightInPixels      uint16
	WidthInMillimeters  uint16
	HeightInMillimeters uint16
	MinInstalledMaps    uint16
	MaxInstalledMaps    uint16
	RootVisual          uint32
	BackingStores       uint8
	SaveUnders          bool
	RootDepth           uint8
	AllowedDepthsCount  uint8
	Depths              []DepthInfo
}

// DepthInfo contains information about a supported color depth.
type DepthInfo struct {
	Depth       uint8
	VisualsCount uint16
	Visuals     []VisualType
}

// VisualType contains information about a visual.
type VisualType struct {
	VisualID        uint32
	Class           uint8
	BitsPerRGBValue uint8
	ColormapEntries uint16
	RedMask         uint32
	GreenMask       uint32
	BlueMask        uint32
}

// PixmapFormat contains pixmap format information.
type PixmapFormat struct {
	Depth        uint8
	BitsPerPixel uint8
	ScanlinePad  uint8
}

// Errors from setup.
var (
	ErrSetupFailed       = errors.New("x11: connection setup failed")
	ErrSetupAuthenticate = errors.New("x11: server requires additional authentication")
	ErrInvalidSetup      = errors.New("x11: invalid setup response")
)

// buildSetupRequest builds the initial connection setup request.
func buildSetupRequest(order ByteOrder, authName string, authData []byte) []byte {
	authNameLen := len(authName)
	authDataLen := len(authData)

	// Calculate total size with padding
	authNamePadded := authNameLen + pad(authNameLen)
	authDataPadded := authDataLen + pad(authDataLen)
	totalLen := 12 + authNamePadded + authDataPadded

	e := NewEncoder(order)

	// Byte order
	e.PutUint8(byte(order))
	// Unused
	e.PutUint8(0)
	// Protocol version
	e.PutUint16(11) // Major
	e.PutUint16(0)  // Minor
	// Auth name length
	e.PutUint16(uint16(authNameLen))
	// Auth data length
	e.PutUint16(uint16(authDataLen))
	// Unused
	e.PutUint16(0)

	// Auth name with padding
	e.PutBytes([]byte(authName))
	e.PutPadN(pad(authNameLen))

	// Auth data with padding
	e.PutBytes(authData)
	e.PutPadN(pad(authDataLen))

	result := e.Bytes()

	// Ensure we return exactly the expected length
	if len(result) != totalLen {
		// Pad to expected length if needed
		for len(result) < totalLen {
			result = append(result, 0)
		}
	}

	return result
}

// parseSetupResponse parses the server's setup response.
func parseSetupResponse(order ByteOrder, data []byte) (*SetupInfo, error) {
	if len(data) < 8 {
		return nil, ErrInvalidSetup
	}

	d := NewDecoder(order, data)

	// Read status
	status, err := d.Uint8()
	if err != nil {
		return nil, err
	}

	switch status {
	case SetupFailed:
		// Read failure reason
		reasonLen, _ := d.Uint8()
		_, _ = d.Uint16() // protocol major
		_, _ = d.Uint16() // protocol minor
		_, _ = d.Uint16() // additional data length
		reason, _ := d.String(int(reasonLen))
		return nil, fmt.Errorf("%w: %s", ErrSetupFailed, reason)

	case SetupAuthenticate:
		return nil, ErrSetupAuthenticate

	case SetupSuccess:
		return parseSetupSuccess(d)

	default:
		return nil, ErrInvalidSetup
	}
}

// parseSetupSuccess parses a successful setup response.
func parseSetupSuccess(d *Decoder) (*SetupInfo, error) {
	info := &SetupInfo{}

	// Skip unused byte after status
	if err := d.Skip(1); err != nil {
		return nil, err
	}

	// Protocol version
	var err error
	info.ProtocolMajorVersion, err = d.Uint16()
	if err != nil {
		return nil, err
	}
	info.ProtocolMinorVersion, err = d.Uint16()
	if err != nil {
		return nil, err
	}

	// Additional data length (in 4-byte units)
	_, err = d.Uint16()
	if err != nil {
		return nil, err
	}

	// Release number
	info.ReleaseNumber, err = d.Uint32()
	if err != nil {
		return nil, err
	}

	// Resource ID base and mask
	info.ResourceIDBase, err = d.Uint32()
	if err != nil {
		return nil, err
	}
	info.ResourceIDMask, err = d.Uint32()
	if err != nil {
		return nil, err
	}

	// Motion buffer size
	info.MotionBufferSize, err = d.Uint32()
	if err != nil {
		return nil, err
	}

	// Vendor length
	info.VendorLength, err = d.Uint16()
	if err != nil {
		return nil, err
	}

	// Maximum request length
	info.MaxRequestLength, err = d.Uint16()
	if err != nil {
		return nil, err
	}

	// Number of screens
	numScreens, err := d.Uint8()
	if err != nil {
		return nil, err
	}

	// Number of pixmap formats
	numFormats, err := d.Uint8()
	if err != nil {
		return nil, err
	}

	// Image byte order
	info.ImageByteOrder, err = d.Uint8()
	if err != nil {
		return nil, err
	}

	// Bitmap bit order
	info.BitmapBitOrder, err = d.Uint8()
	if err != nil {
		return nil, err
	}

	// Bitmap scanline unit
	info.BitmapScanlineUnit, err = d.Uint8()
	if err != nil {
		return nil, err
	}

	// Bitmap scanline pad
	info.BitmapScanlinePad, err = d.Uint8()
	if err != nil {
		return nil, err
	}

	// Min and max keycode
	info.MinKeycode, err = d.Uint8()
	if err != nil {
		return nil, err
	}
	info.MaxKeycode, err = d.Uint8()
	if err != nil {
		return nil, err
	}

	// Skip 4 bytes of unused
	if err := d.Skip(4); err != nil {
		return nil, err
	}

	// Read vendor string
	info.Vendor, err = d.String(int(info.VendorLength))
	if err != nil {
		return nil, err
	}
	if err := d.SkipPad(int(info.VendorLength)); err != nil {
		return nil, err
	}

	// Read pixmap formats
	info.PixmapFormats = make([]PixmapFormat, numFormats)
	for i := range info.PixmapFormats {
		format, err := parsePixmapFormat(d)
		if err != nil {
			return nil, err
		}
		info.PixmapFormats[i] = format
	}

	// Read screens
	info.Screens = make([]ScreenInfo, numScreens)
	for i := range info.Screens {
		screen, err := parseScreenInfo(d)
		if err != nil {
			return nil, err
		}
		info.Screens[i] = screen
	}

	return info, nil
}

// parsePixmapFormat parses a pixmap format structure.
func parsePixmapFormat(d *Decoder) (PixmapFormat, error) {
	var f PixmapFormat
	var err error

	f.Depth, err = d.Uint8()
	if err != nil {
		return f, err
	}

	f.BitsPerPixel, err = d.Uint8()
	if err != nil {
		return f, err
	}

	f.ScanlinePad, err = d.Uint8()
	if err != nil {
		return f, err
	}

	// Skip 5 bytes of padding
	if err := d.Skip(5); err != nil {
		return f, err
	}

	return f, nil
}

// parseScreenInfo parses a screen structure.
func parseScreenInfo(d *Decoder) (ScreenInfo, error) {
	var s ScreenInfo
	var err error

	root, err := d.Uint32()
	if err != nil {
		return s, err
	}
	s.Root = ResourceID(root)

	colormap, err := d.Uint32()
	if err != nil {
		return s, err
	}
	s.DefaultColormap = ResourceID(colormap)

	s.WhitePixel, err = d.Uint32()
	if err != nil {
		return s, err
	}

	s.BlackPixel, err = d.Uint32()
	if err != nil {
		return s, err
	}

	s.CurrentInputMasks, err = d.Uint32()
	if err != nil {
		return s, err
	}

	s.WidthInPixels, err = d.Uint16()
	if err != nil {
		return s, err
	}

	s.HeightInPixels, err = d.Uint16()
	if err != nil {
		return s, err
	}

	s.WidthInMillimeters, err = d.Uint16()
	if err != nil {
		return s, err
	}

	s.HeightInMillimeters, err = d.Uint16()
	if err != nil {
		return s, err
	}

	s.MinInstalledMaps, err = d.Uint16()
	if err != nil {
		return s, err
	}

	s.MaxInstalledMaps, err = d.Uint16()
	if err != nil {
		return s, err
	}

	s.RootVisual, err = d.Uint32()
	if err != nil {
		return s, err
	}

	s.BackingStores, err = d.Uint8()
	if err != nil {
		return s, err
	}

	saveUnders, err := d.Uint8()
	if err != nil {
		return s, err
	}
	s.SaveUnders = saveUnders != 0

	s.RootDepth, err = d.Uint8()
	if err != nil {
		return s, err
	}

	s.AllowedDepthsCount, err = d.Uint8()
	if err != nil {
		return s, err
	}

	// Parse depths
	s.Depths = make([]DepthInfo, s.AllowedDepthsCount)
	for i := range s.Depths {
		depth, err := parseDepthInfo(d)
		if err != nil {
			return s, err
		}
		s.Depths[i] = depth
	}

	return s, nil
}

// parseDepthInfo parses a depth structure.
func parseDepthInfo(d *Decoder) (DepthInfo, error) {
	var di DepthInfo
	var err error

	di.Depth, err = d.Uint8()
	if err != nil {
		return di, err
	}

	// Skip 1 byte unused
	if err := d.Skip(1); err != nil {
		return di, err
	}

	di.VisualsCount, err = d.Uint16()
	if err != nil {
		return di, err
	}

	// Skip 4 bytes unused
	if err := d.Skip(4); err != nil {
		return di, err
	}

	// Parse visuals
	di.Visuals = make([]VisualType, di.VisualsCount)
	for i := range di.Visuals {
		visual, err := parseVisualType(d)
		if err != nil {
			return di, err
		}
		di.Visuals[i] = visual
	}

	return di, nil
}

// parseVisualType parses a visual type structure.
func parseVisualType(d *Decoder) (VisualType, error) {
	var v VisualType
	var err error

	v.VisualID, err = d.Uint32()
	if err != nil {
		return v, err
	}

	v.Class, err = d.Uint8()
	if err != nil {
		return v, err
	}

	v.BitsPerRGBValue, err = d.Uint8()
	if err != nil {
		return v, err
	}

	v.ColormapEntries, err = d.Uint16()
	if err != nil {
		return v, err
	}

	v.RedMask, err = d.Uint32()
	if err != nil {
		return v, err
	}

	v.GreenMask, err = d.Uint32()
	if err != nil {
		return v, err
	}

	v.BlueMask, err = d.Uint32()
	if err != nil {
		return v, err
	}

	// Skip 4 bytes unused
	if err := d.Skip(4); err != nil {
		return v, err
	}

	return v, nil
}
