//go:build darwin

package sound

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// darwinSoundName maps SystemSound to macOS system sound file names.
// These files are located in /System/Library/Sounds/.
func darwinSoundName(s SystemSound) string {
	switch s {
	case Click:
		return "Tink"
	case Alert:
		return "Glass"
	case Error:
		return "Basso"
	case Warning:
		return "Sosumi"
	case Success:
		return "Hero"
	default:
		return "Tink"
	}
}

// nsSoundRT holds the lazily-initialized Objective-C runtime state
// needed to play sounds via NSSound.
var nsSoundRT struct {
	once sync.Once
	err  error

	libobjc unsafe.Pointer
	appKit  unsafe.Pointer

	objcGetClass    unsafe.Pointer
	objcMsgSend     unsafe.Pointer
	selRegisterName unsafe.Pointer

	// Pre-registered selectors
	selSoundNamed uintptr
	selPlay       uintptr

	// Reusable call interfaces
	cif2 *types.CallInterface // 2 args: self, _cmd
	cif3 *types.CallInterface // 3 args: self, _cmd, arg1

	mu sync.Mutex // protects CIF usage (not concurrency-safe)
}

func initNSSound() error {
	nsSoundRT.once.Do(func() {
		nsSoundRT.err = loadNSSound()
	})
	return nsSoundRT.err
}

func loadNSSound() error {
	var err error

	nsSoundRT.libobjc, err = ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
	if err != nil {
		return fmt.Errorf("sound: failed to load libobjc: %w", err)
	}

	nsSoundRT.appKit, err = ffi.LoadLibrary(
		"/System/Library/Frameworks/AppKit.framework/AppKit")
	if err != nil {
		return fmt.Errorf("sound: failed to load AppKit: %w", err)
	}

	nsSoundRT.objcGetClass, err = ffi.GetSymbol(nsSoundRT.libobjc, "objc_getClass")
	if err != nil {
		return fmt.Errorf("sound: objc_getClass not found: %w", err)
	}

	nsSoundRT.objcMsgSend, err = ffi.GetSymbol(nsSoundRT.libobjc, "objc_msgSend")
	if err != nil {
		return fmt.Errorf("sound: objc_msgSend not found: %w", err)
	}

	nsSoundRT.selRegisterName, err = ffi.GetSymbol(nsSoundRT.libobjc, "sel_registerName")
	if err != nil {
		return fmt.Errorf("sound: sel_registerName not found: %w", err)
	}

	// Prepare call interfaces

	// cif2: 2 args (self, _cmd) -> returns pointer
	nsSoundRT.cif2 = &types.CallInterface{}
	err = ffi.PrepareCallInterface(
		nsSoundRT.cif2,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // self
			types.PointerTypeDescriptor, // _cmd
		},
	)
	if err != nil {
		return fmt.Errorf("sound: failed to prepare cif2: %w", err)
	}

	// cif3: 3 args (self, _cmd, arg1) -> returns pointer
	nsSoundRT.cif3 = &types.CallInterface{}
	err = ffi.PrepareCallInterface(
		nsSoundRT.cif3,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // self
			types.PointerTypeDescriptor, // _cmd
			types.PointerTypeDescriptor, // arg1
		},
	)
	if err != nil {
		return fmt.Errorf("sound: failed to prepare cif3: %w", err)
	}

	// Register selectors
	nsSoundRT.selSoundNamed, err = darwinRegisterSel("soundNamed:")
	if err != nil {
		return err
	}
	nsSoundRT.selPlay, err = darwinRegisterSel("play")
	if err != nil {
		return err
	}

	return nil
}

// darwinRegisterSel registers an Objective-C selector by name.
func darwinRegisterSel(name string) (uintptr, error) {
	nameBytes := append([]byte(name), 0)
	namePtr := unsafe.Pointer(&nameBytes[0])

	cif := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(
		cif,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor},
	); err != nil {
		return 0, fmt.Errorf("sound: failed to prepare sel CIF: %w", err)
	}

	var result uintptr
	if err := ffi.CallFunction(cif, nsSoundRT.selRegisterName, unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&namePtr)}); err != nil {
		return 0, fmt.Errorf("sound: sel_registerName(%q) call failed: %w", name, err)
	}
	if result == 0 {
		return 0, fmt.Errorf("sound: sel_registerName(%q) returned nil", name)
	}
	return result, nil
}

// darwinGetClass looks up an Objective-C class by name.
func darwinGetClass(name string) (uintptr, error) {
	nameBytes := append([]byte(name), 0)
	namePtr := unsafe.Pointer(&nameBytes[0])

	cif := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(
		cif,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor},
	); err != nil {
		return 0, fmt.Errorf("sound: failed to prepare class CIF: %w", err)
	}

	var result uintptr
	if err := ffi.CallFunction(cif, nsSoundRT.objcGetClass, unsafe.Pointer(&result),
		[]unsafe.Pointer{unsafe.Pointer(&namePtr)}); err != nil {
		return 0, fmt.Errorf("sound: objc_getClass(%q) call failed: %w", name, err)
	}
	if result == 0 {
		return 0, fmt.Errorf("sound: class %q not found", name)
	}
	return result, nil
}

// darwinCreateNSString creates an autoreleased NSString from a Go string.
func darwinCreateNSString(s string) (uintptr, error) {
	cls, err := darwinGetClass("NSString")
	if err != nil {
		return 0, err
	}

	selUTF8, err := darwinRegisterSel("stringWithUTF8String:")
	if err != nil {
		return 0, err
	}

	strBytes := append([]byte(s), 0)
	strPtr := unsafe.Pointer(&strBytes[0])

	nsSoundRT.mu.Lock()
	defer nsSoundRT.mu.Unlock()

	var result uintptr
	if err := ffi.CallFunction(nsSoundRT.cif3, nsSoundRT.objcMsgSend, unsafe.Pointer(&result),
		[]unsafe.Pointer{
			unsafe.Pointer(&cls),
			unsafe.Pointer(&selUTF8),
			unsafe.Pointer(&strPtr),
		}); err != nil {
		return 0, fmt.Errorf("sound: NSString stringWithUTF8String: failed: %w", err)
	}
	if result == 0 {
		return 0, errors.New("sound: NSString creation failed")
	}
	return result, nil
}

func platformPlay(s SystemSound) {
	if err := initNSSound(); err != nil {
		return
	}

	name := darwinSoundName(s)
	nsStr, err := darwinCreateNSString(name)
	if err != nil {
		return
	}

	cls, err := darwinGetClass("NSSound")
	if err != nil {
		return
	}

	// [NSSound soundNamed:name]
	nsSoundRT.mu.Lock()
	var soundObj uintptr
	err = ffi.CallFunction(nsSoundRT.cif3, nsSoundRT.objcMsgSend, unsafe.Pointer(&soundObj),
		[]unsafe.Pointer{
			unsafe.Pointer(&cls),
			unsafe.Pointer(&nsSoundRT.selSoundNamed),
			unsafe.Pointer(&nsStr),
		})
	nsSoundRT.mu.Unlock()

	if err != nil || soundObj == 0 {
		return
	}

	// [sound play]
	nsSoundRT.mu.Lock()
	_ = ffi.CallFunction(nsSoundRT.cif2, nsSoundRT.objcMsgSend, nil,
		[]unsafe.Pointer{
			unsafe.Pointer(&soundObj),
			unsafe.Pointer(&nsSoundRT.selPlay),
		})
	nsSoundRT.mu.Unlock()
}

func platformPlayFile(path string) error {
	if err := initNSSound(); err != nil {
		return fmt.Errorf("sound: NSSound init failed: %w", err)
	}

	nsStr, err := darwinCreateNSString(path)
	if err != nil {
		return fmt.Errorf("sound: failed to create NSString for path: %w", err)
	}

	cls, err := darwinGetClass("NSSound")
	if err != nil {
		return fmt.Errorf("sound: NSSound class not found: %w", err)
	}

	// [NSSound alloc]
	selAlloc, err := darwinRegisterSel("alloc")
	if err != nil {
		return err
	}

	nsSoundRT.mu.Lock()
	var obj uintptr
	err = ffi.CallFunction(nsSoundRT.cif2, nsSoundRT.objcMsgSend, unsafe.Pointer(&obj),
		[]unsafe.Pointer{
			unsafe.Pointer(&cls),
			unsafe.Pointer(&selAlloc),
		})
	nsSoundRT.mu.Unlock()

	if err != nil {
		return fmt.Errorf("sound: NSSound alloc failed: %w", err)
	}
	if obj == 0 {
		return errors.New("sound: NSSound alloc returned nil")
	}

	// [sound initWithContentsOfFile:path byReference:YES]
	selInit, err := darwinRegisterSel("initWithContentsOfFile:byReference:")
	if err != nil {
		return err
	}

	// Prepare a 4-arg CIF for initWithContentsOfFile:byReference:
	cif4 := &types.CallInterface{}
	if err := ffi.PrepareCallInterface(
		cif4,
		types.DefaultCall,
		types.PointerTypeDescriptor,
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // self
			types.PointerTypeDescriptor, // _cmd
			types.PointerTypeDescriptor, // path NSString
			types.UInt8TypeDescriptor,   // byReference BOOL
		},
	); err != nil {
		return fmt.Errorf("sound: failed to prepare init CIF: %w", err)
	}

	byRef := uint8(1) // YES

	nsSoundRT.mu.Lock()
	var initResult uintptr
	err = ffi.CallFunction(cif4, nsSoundRT.objcMsgSend, unsafe.Pointer(&initResult),
		[]unsafe.Pointer{
			unsafe.Pointer(&obj),
			unsafe.Pointer(&selInit),
			unsafe.Pointer(&nsStr),
			unsafe.Pointer(&byRef),
		})
	nsSoundRT.mu.Unlock()

	if err != nil {
		return fmt.Errorf("sound: NSSound initWithContentsOfFile call failed: %w", err)
	}
	if initResult == 0 {
		return fmt.Errorf("sound: NSSound initWithContentsOfFile failed for %q", path)
	}

	// [sound play]
	nsSoundRT.mu.Lock()
	_ = ffi.CallFunction(nsSoundRT.cif2, nsSoundRT.objcMsgSend, nil,
		[]unsafe.Pointer{
			unsafe.Pointer(&initResult),
			unsafe.Pointer(&nsSoundRT.selPlay),
		})
	nsSoundRT.mu.Unlock()

	return nil
}
