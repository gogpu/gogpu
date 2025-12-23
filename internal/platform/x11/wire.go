//go:build linux

package x11

import (
	"encoding/binary"
	"errors"
)

// ByteOrder represents the X11 protocol byte order.
// Clients can choose big-endian ('B') or little-endian ('l').
type ByteOrder byte

const (
	// MSBFirst is big-endian byte order (0x42 = 'B').
	MSBFirst ByteOrder = 'B'
	// LSBFirst is little-endian byte order (0x6c = 'l').
	LSBFirst ByteOrder = 'l'
)

// ResourceID represents an X11 resource identifier (Window, Pixmap, GC, etc.).
type ResourceID uint32

// Atom represents an interned string identifier.
type Atom uint32

// Timestamp represents an X11 timestamp (milliseconds since server start).
type Timestamp uint32

// Predefined atoms (from X11 protocol).
const (
	AtomNone             Atom = 0
	AtomPrimary          Atom = 1
	AtomSecondary        Atom = 2
	AtomArc              Atom = 3
	AtomAtom             Atom = 4
	AtomBitmap           Atom = 5
	AtomCardinal         Atom = 6
	AtomColormap         Atom = 7
	AtomCursor           Atom = 8
	AtomCutBuffer0       Atom = 9
	AtomCutBuffer1       Atom = 10
	AtomCutBuffer2       Atom = 11
	AtomCutBuffer3       Atom = 12
	AtomCutBuffer4       Atom = 13
	AtomCutBuffer5       Atom = 14
	AtomCutBuffer6       Atom = 15
	AtomCutBuffer7       Atom = 16
	AtomDrawable         Atom = 17
	AtomFont             Atom = 18
	AtomInteger          Atom = 19
	AtomPixmap           Atom = 20
	AtomPoint            Atom = 21
	AtomRectangle        Atom = 22
	AtomResourceManager  Atom = 23
	AtomRGBColorMap      Atom = 24
	AtomRGBBestMap       Atom = 25
	AtomRGBBlueMap       Atom = 26
	AtomRGBDefaultMap    Atom = 27
	AtomRGBGrayMap       Atom = 28
	AtomRGBGreenMap      Atom = 29
	AtomRGBRedMap        Atom = 30
	AtomString           Atom = 31
	AtomVisualID         Atom = 32
	AtomWindow           Atom = 33
	AtomWMCommand        Atom = 34
	AtomWMHints          Atom = 35
	AtomWMClientMachine  Atom = 36
	AtomWMIconName       Atom = 37
	AtomWMIconSize       Atom = 38
	AtomWMName           Atom = 39
	AtomWMNormalHints    Atom = 40
	AtomWMSizeHints      Atom = 41
	AtomWMZoomHints      Atom = 42
	AtomMinSpace         Atom = 43
	AtomNormSpace        Atom = 44
	AtomMaxSpace         Atom = 45
	AtomEndSpace         Atom = 46
	AtomSuperscriptX     Atom = 47
	AtomSuperscriptY     Atom = 48
	AtomSubscriptX       Atom = 49
	AtomSubscriptY       Atom = 50
	AtomUnderlinePos     Atom = 51
	AtomUnderlineThick   Atom = 52
	AtomStrikeoutAscent  Atom = 53
	AtomStrikeoutDescent Atom = 54
	AtomItalicAngle      Atom = 55
	AtomXHeight          Atom = 56
	AtomQuadWidth        Atom = 57
	AtomWeight           Atom = 58
	AtomPointSize        Atom = 59
	AtomResolution       Atom = 60
	AtomCopyright        Atom = 61
	AtomNotice           Atom = 62
	AtomFontName         Atom = 63
	AtomFamilyName       Atom = 64
	AtomFullName         Atom = 65
	AtomCapHeight        Atom = 66
	AtomWMClass          Atom = 67
	AtomWMTransientFor   Atom = 68
)

// CurrentTime is a special timestamp value meaning "now".
const CurrentTime Timestamp = 0

// X11 request opcodes.
const (
	OpcodeCreateWindow            = 1
	OpcodeChangeWindowAttrs       = 2
	OpcodeGetWindowAttrs          = 3
	OpcodeDestroyWindow           = 4
	OpcodeDestroySubwindows       = 5
	OpcodeChangeSaveSet           = 6
	OpcodeReparentWindow          = 7
	OpcodeMapWindow               = 8
	OpcodeMapSubwindows           = 9
	OpcodeUnmapWindow             = 10
	OpcodeUnmapSubwindows         = 11
	OpcodeConfigureWindow         = 12
	OpcodeCirculateWindow         = 13
	OpcodeGetGeometry             = 14
	OpcodeQueryTree               = 15
	OpcodeInternAtom              = 16
	OpcodeGetAtomName             = 17
	OpcodeChangeProperty          = 18
	OpcodeDeleteProperty          = 19
	OpcodeGetProperty             = 20
	OpcodeListProperties          = 21
	OpcodeSetSelectionOwner       = 22
	OpcodeGetSelectionOwner       = 23
	OpcodeConvertSelection        = 24
	OpcodeSendEvent               = 25
	OpcodeGrabPointer             = 26
	OpcodeUngrabPointer           = 27
	OpcodeGrabButton              = 28
	OpcodeUngrabButton            = 29
	OpcodeChangeActivePointerGrab = 30
	OpcodeGrabKeyboard            = 31
	OpcodeUngrabKeyboard          = 32
	OpcodeGrabKey                 = 33
	OpcodeUngrabKey               = 34
	OpcodeAllowEvents             = 35
	OpcodeGrabServer              = 36
	OpcodeUngrabServer            = 37
	OpcodeQueryPointer            = 38
	OpcodeGetMotionEvents         = 39
	OpcodeTranslateCoords         = 40
	OpcodeWarpPointer             = 41
	OpcodeSetInputFocus           = 42
	OpcodeGetInputFocus           = 43
	OpcodeQueryKeymap             = 44
	OpcodeOpenFont                = 45
	OpcodeCloseFont               = 46
	OpcodeQueryFont               = 47
	OpcodeQueryTextExtents        = 48
	OpcodeListFonts               = 49
	OpcodeListFontsWithInfo       = 50
	OpcodeSetFontPath             = 51
	OpcodeGetFontPath             = 52
	OpcodeCreatePixmap            = 53
	OpcodeFreePixmap              = 54
	OpcodeCreateGC                = 55
	OpcodeChangeGC                = 56
	OpcodeCopyGC                  = 57
	OpcodeSetDashes               = 58
	OpcodeSetClipRectangles       = 59
	OpcodeFreeGC                  = 60
	OpcodeClearArea               = 61
	OpcodeCopyArea                = 62
	OpcodeCopyPlane               = 63
	OpcodePolyPoint               = 64
	OpcodePolyLine                = 65
	OpcodePolySegment             = 66
	OpcodePolyRectangle           = 67
	OpcodePolyArc                 = 68
	OpcodeFillPoly                = 69
	OpcodePolyFillRectangle       = 70
	OpcodePolyFillArc             = 71
	OpcodePutImage                = 72
	OpcodeGetImage                = 73
	OpcodePolyText8               = 74
	OpcodePolyText16              = 75
	OpcodeImageText8              = 76
	OpcodeImageText16             = 77
	OpcodeCreateColormap          = 78
	OpcodeFreeColormap            = 79
	OpcodeCopyColormapAndFree     = 80
	OpcodeInstallColormap         = 81
	OpcodeUninstallColormap       = 82
	OpcodeListInstalledColormaps  = 83
	OpcodeAllocColor              = 84
	OpcodeAllocNamedColor         = 85
	OpcodeAllocColorCells         = 86
	OpcodeAllocColorPlanes        = 87
	OpcodeFreeColors              = 88
	OpcodeStoreColors             = 89
	OpcodeStoreNamedColor         = 90
	OpcodeQueryColors             = 91
	OpcodeLookupColor             = 92
	OpcodeCreateCursor            = 93
	OpcodeCreateGlyphCursor       = 94
	OpcodeFreeCursor              = 95
	OpcodeRecolorCursor           = 96
	OpcodeQueryBestSize           = 97
	OpcodeQueryExtension          = 98
	OpcodeListExtensions          = 99
	OpcodeChangeKeyboardMapping   = 100
	OpcodeGetKeyboardMapping      = 101
	OpcodeChangeKeyboardControl   = 102
	OpcodeGetKeyboardControl      = 103
	OpcodeBell                    = 104
	OpcodeChangePointerControl    = 105
	OpcodeGetPointerControl       = 106
	OpcodeSetScreenSaver          = 107
	OpcodeGetScreenSaver          = 108
	OpcodeChangeHosts             = 109
	OpcodeListHosts               = 110
	OpcodeSetAccessControl        = 111
	OpcodeSetCloseDownMode        = 112
	OpcodeKillClient              = 113
	OpcodeRotateProperties        = 114
	OpcodeForceScreenSaver        = 115
	OpcodeSetPointerMapping       = 116
	OpcodeGetPointerMapping       = 117
	OpcodeSetModifierMapping      = 118
	OpcodeGetModifierMapping      = 119
	OpcodeNoOperation             = 127
)

// X11 event codes.
const (
	EventKeyPress         = 2
	EventKeyRelease       = 3
	EventButtonPress      = 4
	EventButtonRelease    = 5
	EventMotionNotify     = 6
	EventEnterNotify      = 7
	EventLeaveNotify      = 8
	EventFocusIn          = 9
	EventFocusOut         = 10
	EventKeymapNotify     = 11
	EventExpose           = 12
	EventGraphicsExposure = 13
	EventNoExposure       = 14
	EventVisibilityNotify = 15
	EventCreateNotify     = 16
	EventDestroyNotify    = 17
	EventUnmapNotify      = 18
	EventMapNotify        = 19
	EventMapRequest       = 20
	EventReparentNotify   = 21
	EventConfigureNotify  = 22
	EventConfigureRequest = 23
	EventGravityNotify    = 24
	EventResizeRequest    = 25
	EventCirculateNotify  = 26
	EventCirculateRequest = 27
	EventPropertyNotify   = 28
	EventSelectionClear   = 29
	EventSelectionRequest = 30
	EventSelectionNotify  = 31
	EventColormapNotify   = 32
	EventClientMessage    = 33
	EventMappingNotify    = 34
)

// X11 error codes.
const (
	ErrorRequest        = 1
	ErrorValue          = 2
	ErrorWindow         = 3
	ErrorPixmap         = 4
	ErrorAtom           = 5
	ErrorCursor         = 6
	ErrorFont           = 7
	ErrorMatch          = 8
	ErrorDrawable       = 9
	ErrorAccess         = 10
	ErrorAlloc          = 11
	ErrorColormap       = 12
	ErrorGContext       = 13
	ErrorIDChoice       = 14
	ErrorName           = 15
	ErrorLength         = 16
	ErrorImplementation = 17
)

// Window class values.
const (
	WindowClassCopyFromParent = 0
	WindowClassInputOutput    = 1
	WindowClassInputOnly      = 2
)

// Event mask bits.
const (
	EventMaskNoEvent              = 0
	EventMaskKeyPress             = 1 << 0
	EventMaskKeyRelease           = 1 << 1
	EventMaskButtonPress          = 1 << 2
	EventMaskButtonRelease        = 1 << 3
	EventMaskEnterWindow          = 1 << 4
	EventMaskLeaveWindow          = 1 << 5
	EventMaskPointerMotion        = 1 << 6
	EventMaskPointerMotionHint    = 1 << 7
	EventMaskButton1Motion        = 1 << 8
	EventMaskButton2Motion        = 1 << 9
	EventMaskButton3Motion        = 1 << 10
	EventMaskButton4Motion        = 1 << 11
	EventMaskButton5Motion        = 1 << 12
	EventMaskButtonMotion         = 1 << 13
	EventMaskKeymapState          = 1 << 14
	EventMaskExposure             = 1 << 15
	EventMaskVisibilityChange     = 1 << 16
	EventMaskStructureNotify      = 1 << 17
	EventMaskResizeRedirect       = 1 << 18
	EventMaskSubstructureNotify   = 1 << 19
	EventMaskSubstructureRedirect = 1 << 20
	EventMaskFocusChange          = 1 << 21
	EventMaskPropertyChange       = 1 << 22
	EventMaskColormapChange       = 1 << 23
	EventMaskOwnerGrabButton      = 1 << 24
)

// CreateWindow value mask bits.
const (
	CWBackPixmap       = 1 << 0
	CWBackPixel        = 1 << 1
	CWBorderPixmap     = 1 << 2
	CWBorderPixel      = 1 << 3
	CWBitGravity       = 1 << 4
	CWWinGravity       = 1 << 5
	CWBackingStore     = 1 << 6
	CWBackingPlanes    = 1 << 7
	CWBackingPixel     = 1 << 8
	CWOverrideRedirect = 1 << 9
	CWSaveUnder        = 1 << 10
	CWEventMask        = 1 << 11
	CWDontPropagate    = 1 << 12
	CWColormap         = 1 << 13
	CWCursor           = 1 << 14
)

// Property mode values.
const (
	PropModeReplace = 0
	PropModePrepend = 1
	PropModeAppend  = 2
)

// Wire protocol errors.
var (
	ErrMessageTooLarge  = errors.New("x11: message exceeds maximum size")
	ErrMessageTooSmall  = errors.New("x11: message smaller than header")
	ErrBufferTooSmall   = errors.New("x11: buffer too small for message")
	ErrUnexpectedEOF    = errors.New("x11: unexpected end of message")
	ErrInvalidStringLen = errors.New("x11: invalid string length")
)

// Maximum message size (64KB as per X11 spec).
const maxMessageSize = 64 * 1024

// Encoder encodes X11 requests to wire format.
type Encoder struct {
	buf       []byte
	byteOrder binary.ByteOrder
}

// NewEncoder creates a new Encoder with the given byte order.
func NewEncoder(order ByteOrder) *Encoder {
	e := &Encoder{
		buf: make([]byte, 0, 256),
	}
	if order == MSBFirst {
		e.byteOrder = binary.BigEndian
	} else {
		e.byteOrder = binary.LittleEndian
	}
	return e
}

// Reset clears the encoder buffer for reuse.
func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
}

// Bytes returns the encoded data.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// Len returns the current buffer length.
func (e *Encoder) Len() int {
	return len(e.buf)
}

// PutUint8 appends a single byte.
func (e *Encoder) PutUint8(v uint8) {
	e.buf = append(e.buf, v)
}

// PutUint16 appends a 16-bit value.
func (e *Encoder) PutUint16(v uint16) {
	b := make([]byte, 2)
	e.byteOrder.PutUint16(b, v)
	e.buf = append(e.buf, b...)
}

// PutUint32 appends a 32-bit value.
func (e *Encoder) PutUint32(v uint32) {
	b := make([]byte, 4)
	e.byteOrder.PutUint32(b, v)
	e.buf = append(e.buf, b...)
}

// PutInt16 appends a signed 16-bit value.
func (e *Encoder) PutInt16(v int16) {
	e.PutUint16(uint16(v))
}

// PutInt32 appends a signed 32-bit value.
func (e *Encoder) PutInt32(v int32) {
	e.PutUint32(uint32(v))
}

// PutBytes appends raw bytes.
func (e *Encoder) PutBytes(data []byte) {
	e.buf = append(e.buf, data...)
}

// PutPad pads the buffer to a 4-byte boundary.
func (e *Encoder) PutPad() {
	pad := (4 - len(e.buf)%4) % 4
	for i := 0; i < pad; i++ {
		e.buf = append(e.buf, 0)
	}
}

// PutPadN pads with n zero bytes.
func (e *Encoder) PutPadN(n int) {
	for i := 0; i < n; i++ {
		e.buf = append(e.buf, 0)
	}
}

// PutString appends a string with padding to 4-byte boundary.
// Does NOT include length prefix (X11 strings are length-prefixed separately).
func (e *Encoder) PutString(s string) {
	e.buf = append(e.buf, s...)
	e.PutPad()
}

// Decoder decodes X11 responses from wire format.
type Decoder struct {
	buf       []byte
	offset    int
	byteOrder binary.ByteOrder
}

// NewDecoder creates a new Decoder with the given byte order.
func NewDecoder(order ByteOrder, buf []byte) *Decoder {
	d := &Decoder{
		buf:    buf,
		offset: 0,
	}
	if order == MSBFirst {
		d.byteOrder = binary.BigEndian
	} else {
		d.byteOrder = binary.LittleEndian
	}
	return d
}

// Reset resets the decoder with a new buffer.
func (d *Decoder) Reset(buf []byte) {
	d.buf = buf
	d.offset = 0
}

// Remaining returns the number of unread bytes.
func (d *Decoder) Remaining() int {
	return len(d.buf) - d.offset
}

// Offset returns the current read position.
func (d *Decoder) Offset() int {
	return d.offset
}

// Skip advances the offset by n bytes.
func (d *Decoder) Skip(n int) error {
	if d.offset+n > len(d.buf) {
		return ErrUnexpectedEOF
	}
	d.offset += n
	return nil
}

// Uint8 reads a single byte.
func (d *Decoder) Uint8() (uint8, error) {
	if d.offset >= len(d.buf) {
		return 0, ErrUnexpectedEOF
	}
	v := d.buf[d.offset]
	d.offset++
	return v, nil
}

// Uint16 reads a 16-bit value.
func (d *Decoder) Uint16() (uint16, error) {
	if d.offset+2 > len(d.buf) {
		return 0, ErrUnexpectedEOF
	}
	v := d.byteOrder.Uint16(d.buf[d.offset:])
	d.offset += 2
	return v, nil
}

// Uint32 reads a 32-bit value.
func (d *Decoder) Uint32() (uint32, error) {
	if d.offset+4 > len(d.buf) {
		return 0, ErrUnexpectedEOF
	}
	v := d.byteOrder.Uint32(d.buf[d.offset:])
	d.offset += 4
	return v, nil
}

// Int16 reads a signed 16-bit value.
func (d *Decoder) Int16() (int16, error) {
	v, err := d.Uint16()
	return int16(v), err
}

// Int32 reads a signed 32-bit value.
func (d *Decoder) Int32() (int32, error) {
	v, err := d.Uint32()
	return int32(v), err
}

// Bytes reads n bytes from the buffer.
func (d *Decoder) Bytes(n int) ([]byte, error) {
	if d.offset+n > len(d.buf) {
		return nil, ErrUnexpectedEOF
	}
	data := make([]byte, n)
	copy(data, d.buf[d.offset:d.offset+n])
	d.offset += n
	return data, nil
}

// String reads n bytes as a string.
func (d *Decoder) String(n int) (string, error) {
	data, err := d.Bytes(n)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SkipPad skips padding to align to 4-byte boundary based on length.
func (d *Decoder) SkipPad(length int) error {
	pad := (4 - length%4) % 4
	return d.Skip(pad)
}

// pad calculates padding needed for 4-byte alignment.
func pad(n int) int {
	return (4 - n%4) % 4
}

// requestLength calculates the request length in 4-byte units.
func requestLength(dataLen int) uint16 {
	return uint16((dataLen + 3) / 4)
}
