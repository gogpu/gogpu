//go:build darwin

package platform

import (
	"strings"
	"unsafe"

	"github.com/gogpu/gogpu/internal/platform/darwin"
)

// nsModalResponseOK is the return value of NSPanel.runModal on user confirmation.
const nsModalResponseOK int64 = 1

func showOpenFileDialog(opts FileDialogOptions) ([]string, error) {
	panel := darwin.GetClass("NSOpenPanel").Send(darwin.RegisterSelector("openPanel"))
	if panel.IsNil() {
		return nil, nil
	}

	applyDialogCommon(panel, opts)
	panel.SendBool(darwin.RegisterSelector("setCanChooseFiles:"), !opts.Directory)
	panel.SendBool(darwin.RegisterSelector("setCanChooseDirectories:"), opts.Directory)
	panel.SendBool(darwin.RegisterSelector("setAllowsMultipleSelection:"), opts.Multiple)

	if panel.GetInt64(darwin.RegisterSelector("runModal")) != nsModalResponseOK {
		return nil, nil
	}

	urls := panel.Send(darwin.RegisterSelector("URLs"))
	if urls.IsNil() {
		return nil, nil
	}
	count := urls.GetUint64(darwin.RegisterSelector("count"))
	paths := make([]string, 0, int(count))
	for i := uint64(0); i < count; i++ {
		url := urls.SendUint(darwin.RegisterSelector("objectAtIndex:"), i)
		if path := darwinNSStringToGo(url.Send(darwin.RegisterSelector("path"))); path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func showSaveFileDialog(opts FileDialogOptions) (string, error) {
	panel := darwin.GetClass("NSSavePanel").Send(darwin.RegisterSelector("savePanel"))
	if panel.IsNil() {
		return "", nil
	}

	applyDialogCommon(panel, opts)

	if opts.DefaultFilename != "" {
		ns := darwin.NewNSString(opts.DefaultFilename)
		if ns != nil {
			defer ns.Release()
			panel.SendPtr(darwin.RegisterSelector("setNameFieldStringValue:"), uintptr(ns.ID()))
		}
	}

	if panel.GetInt64(darwin.RegisterSelector("runModal")) != nsModalResponseOK {
		return "", nil
	}

	url := panel.Send(darwin.RegisterSelector("URL"))
	if url.IsNil() {
		return "", nil
	}
	return darwinNSStringToGo(url.Send(darwin.RegisterSelector("path"))), nil
}

// applyDialogCommon sets title, initial directory, and file type filters on a panel.
func applyDialogCommon(panel darwin.ID, opts FileDialogOptions) {
	if opts.Title != "" {
		ns := darwin.NewNSString(opts.Title)
		if ns != nil {
			defer ns.Release()
			panel.SendPtr(darwin.RegisterSelector("setTitle:"), uintptr(ns.ID()))
		}
	}

	if opts.InitialDirectory != "" {
		pathNS := darwin.NewNSString(opts.InitialDirectory)
		if pathNS != nil {
			defer pathNS.Release()
			urlClass := darwin.ID(darwin.GetClass("NSURL"))
			dirURL := urlClass.SendPtr(darwin.RegisterSelector("fileURLWithPath:"), uintptr(pathNS.ID()))
			if !dirURL.IsNil() {
				panel.SendPtr(darwin.RegisterSelector("setDirectoryURL:"), uintptr(dirURL))
			}
		}
	}

	if len(opts.Filters) > 0 && !opts.Directory {
		if arr := darwinBuildExtArray(opts.Filters); !arr.IsNil() {
			panel.SendPtr(darwin.RegisterSelector("setAllowedFileTypes:"), uintptr(arr))
		}
	}
}

// darwinBuildExtArray builds an NSMutableArray of extension NSStrings for setAllowedFileTypes:.
// Extensions are stripped of "*." or "." prefixes; e.g. "*.png" → "png".
func darwinBuildExtArray(filters []FileTypeFilter) darwin.ID {
	arr := darwin.ID(darwin.GetClass("NSMutableArray")).Send(darwin.RegisterSelector("array"))
	if arr.IsNil() {
		return 0
	}
	added := 0
	for _, f := range filters {
		for _, e := range f.Extensions {
			e = strings.TrimPrefix(e, "*")
			e = strings.TrimPrefix(e, ".")
			if e == "" {
				continue
			}
			ns := darwin.NewNSString(e)
			if ns == nil {
				continue
			}
			arr.SendPtr(darwin.RegisterSelector("addObject:"), uintptr(ns.ID()))
			ns.Release()
			added++
		}
	}
	if added == 0 {
		return 0
	}
	return arr
}

// darwinNSStringToGo converts an ObjC NSString to a Go string.
func darwinNSStringToGo(nsstr darwin.ID) string {
	utf8Ptr := darwin.NSStringUTF8Ptr(nsstr)
	if utf8Ptr == 0 {
		return ""
	}
	length := darwin.NSStringLength(nsstr)
	if length == 0 {
		return ""
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(utf8Ptr)), length*4) //nolint:govet // ObjC UTF8String pointer, bounded by NSString length
	end := 0
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[:end])
}
