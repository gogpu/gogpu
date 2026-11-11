//go:build darwin

package darwin

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// UCKeyTranslate's arguments for a key's label: its display character, with
// no dead key processed.
const (
	ucKeyActionDisplay           = 3
	ucKeyTranslateNoDeadKeysMask = 1 << 0
)

// cfNotificationSuspensionBehaviorDeliverImmediately delivers a distributed
// notification while the application is inactive too.
const cfNotificationSuspensionBehaviorDeliverImmediately = 4

// KeyboardLayout is a copy of a keyboard layout's key map, the 'uchr' data
// UCKeyTranslate reads. The zero value labels every key 0.
type KeyboardLayout struct {
	keyMap []byte
}

// InputSourceObserver is a registration of ObserveInputSourceChanges.
type InputSourceObserver uintptr

// textInputSources holds the Text Input Sources and CoreFoundation symbols
// that read keyboard layouts and observe their changes. The CoreFoundation
// objects they answer, toll-free bridged, are reached through Objective-C
// messages.
var textInputSources struct {
	once sync.Once
	err  error

	copyCurrentKeyboardLayoutInputSource unsafe.Pointer
	getInputSourceProperty               unsafe.Pointer
	createInputSourceList                unsafe.Pointer
	keyTranslate                         unsafe.Pointer
	getKeyboardType                      unsafe.Pointer
	addObserver                          unsafe.Pointer
	removeObserver                       unsafe.Pointer

	// The CFStringRef constants: the property keys and the notification's name.
	propertyUnicodeKeyLayoutData uintptr
	propertyInputSourceID        uintptr
	selectedInputSourceChanged   uintptr

	distributedCenter uintptr
	// observerCallback calls the function inputSourceObservers holds for the
	// observer it is given: one callback serves every registration.
	observerCallback uintptr

	cifNoArgs         *types.CallInterface // () -> pointer
	cifProperty       *types.CallInterface // (pointer, pointer) -> pointer
	cifList           *types.CallInterface // (pointer, Boolean) -> pointer
	cifKeyboardType   *types.CallInterface // () -> UInt8
	cifKeyTranslate   *types.CallInterface // UCKeyTranslate
	cifAddObserver    *types.CallInterface // (center, observer, callback, name, object, behavior) -> void
	cifRemoveObserver *types.CallInterface // (center, observer, name, object) -> void
}

// inputSourceObservers maps each InputSourceObserver to its function. Main
// thread only.
var inputSourceObservers = map[InputSourceObserver]func(){}

// lastInputSourceObserver numbers the observers: CoreFoundation tells them
// apart by the value alone. Main thread only.
var lastInputSourceObserver InputSourceObserver

// CurrentKeyboardLayout answers the keyboard layout in use: the selected input
// source's, or the one an input method types with. Call it on the main thread.
func CurrentKeyboardLayout() (KeyboardLayout, error) {
	if err := initTextInputSources(); err != nil {
		return KeyboardLayout{}, err
	}
	var source ID
	callTextInputSources(textInputSources.cifNoArgs, textInputSources.copyCurrentKeyboardLayoutInputSource,
		unsafe.Pointer(&source))
	if source.IsNil() {
		return KeyboardLayout{}, errors.New("darwin: no keyboard layout input source")
	}
	defer source.Send(selectors.release)
	return keyboardLayoutOfSource(source)
}

// InstalledKeyboardLayout answers the layout of an installed keyboard layout
// input source, such as "com.apple.keylayout.French", whether the user enabled
// it or not. Call it on the main thread.
func InstalledKeyboardLayout(inputSourceID string) (KeyboardLayout, error) {
	if err := initTextInputSources(); err != nil {
		return KeyboardLayout{}, err
	}
	id := NewNSString(inputSourceID)
	defer id.Release()
	properties := ID(GetClass("NSMutableDictionary")).Send(selectors.alloc).Send(selectors.init)
	if id.ID().IsNil() || properties.IsNil() {
		return KeyboardLayout{}, errors.New("darwin: no properties to find the input source by")
	}
	defer properties.Send(selectors.release)
	properties.SendPtrs(RegisterSelector("setObject:forKey:"), id.ID().Ptr(), textInputSources.propertyInputSourceID)

	includeAllInstalled := uint8(1)
	var sources ID
	callTextInputSources(textInputSources.cifList, textInputSources.createInputSourceList,
		unsafe.Pointer(&sources), unsafe.Pointer(&properties), unsafe.Pointer(&includeAllInstalled))
	if sources.IsNil() {
		return KeyboardLayout{}, fmt.Errorf("darwin: no input source %q installed", inputSourceID)
	}
	defer sources.Send(selectors.release)
	source := sources.Send(RegisterSelector("firstObject"))
	if source.IsNil() {
		return KeyboardLayout{}, fmt.Errorf("darwin: no input source %q installed", inputSourceID)
	}
	return keyboardLayoutOfSource(source)
}

// Label answers the character the layout prints on a key, given by its macOS
// virtual key code, with no modifier down: the character Keyboard Viewer
// shows, a dead key's own included. It answers 0 when the key prints none.
func (l KeyboardLayout) Label(keyCode uint16) rune {
	if len(l.keyMap) == 0 {
		return 0
	}
	var keyboardType uint8
	callTextInputSources(textInputSources.cifKeyboardType, textInputSources.getKeyboardType, unsafe.Pointer(&keyboardType))
	keyMap := unsafe.Pointer(&l.keyMap[0])
	action := uint16(ucKeyActionDisplay)
	var modifiers uint32
	keyboardTypeArgument := uint32(keyboardType)
	options := uint32(ucKeyTranslateNoDeadKeysMask)
	var deadKeyState uint32
	deadKeyStatePointer := unsafe.Pointer(&deadKeyState)
	var text [4]uint16
	maxLength := uint64(len(text))
	var length uint64
	lengthPointer := unsafe.Pointer(&length)
	textPointer := unsafe.Pointer(&text[0])
	var status int32
	callTextInputSources(textInputSources.cifKeyTranslate, textInputSources.keyTranslate,
		unsafe.Pointer(&status), unsafe.Pointer(&keyMap), unsafe.Pointer(&keyCode), unsafe.Pointer(&action),
		unsafe.Pointer(&modifiers), unsafe.Pointer(&keyboardTypeArgument), unsafe.Pointer(&options),
		unsafe.Pointer(&deadKeyStatePointer), unsafe.Pointer(&maxLength), unsafe.Pointer(&lengthPointer),
		unsafe.Pointer(&textPointer))
	if status != 0 || length == 0 {
		return 0
	}
	return utf16.Decode(text[:length])[0]
}

// Equal reports whether two layouts print alike on every key.
func (l KeyboardLayout) Equal(other KeyboardLayout) bool {
	return bytes.Equal(l.keyMap, other.keyMap)
}

// ObserveInputSourceChanges calls changed each time the user selects another
// input source, until Stop. The main thread's run loop delivers the changes,
// while the application is inactive too. The layout in use may stay the same:
// an input method types with a layout of its own. Call it on the main thread.
func ObserveInputSourceChanges(changed func()) (InputSourceObserver, error) {
	if err := initTextInputSources(); err != nil {
		return 0, err
	}
	lastInputSourceObserver++
	observer := lastInputSourceObserver
	inputSourceObservers[observer] = changed
	var object uintptr
	behavior := int64(cfNotificationSuspensionBehaviorDeliverImmediately)
	callTextInputSources(textInputSources.cifAddObserver, textInputSources.addObserver, nil,
		unsafe.Pointer(&textInputSources.distributedCenter), unsafe.Pointer(&observer),
		unsafe.Pointer(&textInputSources.observerCallback), unsafe.Pointer(&textInputSources.selectedInputSourceChanged),
		unsafe.Pointer(&object), unsafe.Pointer(&behavior))
	return observer, nil
}

// Stop ends the observation. A second Stop does nothing. Call it on the main
// thread.
func (o InputSourceObserver) Stop() {
	if _, ok := inputSourceObservers[o]; !ok {
		return
	}
	delete(inputSourceObservers, o)
	var object uintptr
	callTextInputSources(textInputSources.cifRemoveObserver, textInputSources.removeObserver, nil,
		unsafe.Pointer(&textInputSources.distributedCenter), unsafe.Pointer(&o),
		unsafe.Pointer(&textInputSources.selectedInputSourceChanged), unsafe.Pointer(&object))
}

// keyboardLayoutOfSource copies the key map of a keyboard layout input source.
func keyboardLayoutOfSource(source ID) (KeyboardLayout, error) {
	var data ID
	callTextInputSources(textInputSources.cifProperty, textInputSources.getInputSourceProperty,
		unsafe.Pointer(&data), unsafe.Pointer(&source), unsafe.Pointer(&textInputSources.propertyUnicodeKeyLayoutData))
	if data.IsNil() {
		return KeyboardLayout{}, errors.New("darwin: the input source has no Unicode key layout")
	}
	length := data.GetUint64(selectors.length)
	if length == 0 {
		return KeyboardLayout{}, errors.New("darwin: the input source's key layout is empty")
	}
	keyMap := make([]byte, length)
	// SendPtrs is a Go wrapper around msgSend: pin the bytes it passes as a
	// uintptr for the call (P1-FFI-001).
	var pinner runtime.Pinner
	pinner.Pin(&keyMap[0])
	data.SendPtrs(RegisterSelector("getBytes:length:"), uintptr(unsafe.Pointer(&keyMap[0])), uintptr(length))
	pinner.Unpin()
	return KeyboardLayout{keyMap: keyMap}, nil
}

// callTextInputSources calls fn through cif with the addresses of its
// arguments, its result written at result. Once the symbols are loaded the
// call itself cannot fail.
func callTextInputSources(cif *types.CallInterface, fn, result unsafe.Pointer, arguments ...unsafe.Pointer) {
	_, _ = ffi.CallFunction(cif, fn, result, arguments)
}

// initTextInputSources loads the symbols once.
func initTextInputSources() error {
	textInputSources.once.Do(func() {
		textInputSources.err = loadTextInputSources()
	})
	return textInputSources.err
}

func loadTextInputSources() error {
	if err := initRuntime(); err != nil {
		return err
	}
	initSelectors()
	initClasses()
	textInputSourcesLibrary, err := ffi.LoadLibrary(
		"/System/Library/Frameworks/Carbon.framework/Frameworks/HIToolbox.framework/HIToolbox")
	if err != nil {
		return errors.Join(ErrLibraryNotLoaded, err)
	}

	symbols := []struct {
		library unsafe.Pointer
		name    string
		address *unsafe.Pointer
	}{
		{textInputSourcesLibrary, "TISCopyCurrentKeyboardLayoutInputSource", &textInputSources.copyCurrentKeyboardLayoutInputSource},
		{textInputSourcesLibrary, "TISGetInputSourceProperty", &textInputSources.getInputSourceProperty},
		{textInputSourcesLibrary, "TISCreateInputSourceList", &textInputSources.createInputSourceList},
		{textInputSourcesLibrary, "UCKeyTranslate", &textInputSources.keyTranslate},
		{textInputSourcesLibrary, "LMGetKbdType", &textInputSources.getKeyboardType},
		{objcRT.coreFoundation, "CFNotificationCenterAddObserver", &textInputSources.addObserver},
		{objcRT.coreFoundation, "CFNotificationCenterRemoveObserver", &textInputSources.removeObserver},
	}
	for _, symbol := range symbols {
		if *symbol.address, err = ffi.GetSymbol(symbol.library, symbol.name); err != nil {
			return errors.Join(ErrSymbolNotFound, err)
		}
	}
	constants := []struct {
		name  string
		value *uintptr
	}{
		{"kTISPropertyUnicodeKeyLayoutData", &textInputSources.propertyUnicodeKeyLayoutData},
		{"kTISPropertyInputSourceID", &textInputSources.propertyInputSourceID},
		{"kTISNotifySelectedKeyboardInputSourceChanged", &textInputSources.selectedInputSourceChanged},
	}
	for _, constant := range constants {
		address, err := ffi.GetSymbol(textInputSourcesLibrary, constant.name)
		if err != nil {
			return errors.Join(ErrSymbolNotFound, err)
		}
		*constant.value = *(*uintptr)(address)
	}

	pointer := types.PointerTypeDescriptor
	callInterfaces := []struct {
		cif        **types.CallInterface
		result     *types.TypeDescriptor
		parameters []*types.TypeDescriptor
	}{
		{&textInputSources.cifNoArgs, pointer, nil},
		{&textInputSources.cifProperty, pointer, []*types.TypeDescriptor{pointer, pointer}},
		{&textInputSources.cifList, pointer, []*types.TypeDescriptor{pointer, types.UInt8TypeDescriptor}},
		{&textInputSources.cifKeyboardType, types.UInt8TypeDescriptor, nil},
		{&textInputSources.cifKeyTranslate, types.SInt32TypeDescriptor, []*types.TypeDescriptor{
			pointer,                    // keyLayoutPtr
			types.UInt16TypeDescriptor, // virtualKeyCode
			types.UInt16TypeDescriptor, // keyAction
			types.UInt32TypeDescriptor, // modifierKeyState
			types.UInt32TypeDescriptor, // keyboardType
			types.UInt32TypeDescriptor, // keyTranslateOptions
			pointer,                    // deadKeyState
			types.UInt64TypeDescriptor, // maxStringLength
			pointer,                    // actualStringLength
			pointer,                    // unicodeString
		}},
		{&textInputSources.cifAddObserver, types.VoidTypeDescriptor,
			[]*types.TypeDescriptor{pointer, pointer, pointer, pointer, pointer, types.SInt64TypeDescriptor}},
		{&textInputSources.cifRemoveObserver, types.VoidTypeDescriptor, []*types.TypeDescriptor{pointer, pointer, pointer, pointer}},
	}
	for _, callInterface := range callInterfaces {
		*callInterface.cif = &types.CallInterface{}
		if err := ffi.PrepareCallInterface(*callInterface.cif, types.DefaultCall, callInterface.result, callInterface.parameters); err != nil {
			return err
		}
	}

	getDistributedCenter, err := ffi.GetSymbol(objcRT.coreFoundation, "CFNotificationCenterGetDistributedCenter")
	if err != nil {
		return errors.Join(ErrSymbolNotFound, err)
	}
	callTextInputSources(textInputSources.cifNoArgs, getDistributedCenter, unsafe.Pointer(&textInputSources.distributedCenter))
	textInputSources.observerCallback = ffi.NewCallback(func(_, observer, _, _, _ uintptr) uintptr {
		if changed, ok := inputSourceObservers[InputSourceObserver(observer)]; ok {
			changed()
		}
		return 0
	})
	return nil
}
