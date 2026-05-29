//go:build darwin

package platform

import (
	"testing"

	"github.com/gogpu/gogpu/internal/platform/darwin"
)

// TestDarwinNSStringToGo verifies conversion of ObjC NSString to Go string.
func TestDarwinNSStringToGo(t *testing.T) {
	t.Run("nil ID returns empty string", func(t *testing.T) {
		if got := darwinNSStringToGo(0); got != "" {
			t.Errorf("darwinNSStringToGo(0) = %q, want empty", got)
		}
	})

	t.Run("ASCII round-trip", func(t *testing.T) {
		ns := darwin.NewNSString("hello")
		if ns == nil {
			t.Fatal("NewNSString returned nil")
		}
		defer ns.Release()
		if got := darwinNSStringToGo(ns.ID()); got != "hello" {
			t.Errorf("darwinNSStringToGo = %q, want %q", got, "hello")
		}
	})

	t.Run("Unicode round-trip", func(t *testing.T) {
		const input = "こんにちは 🌏"
		ns := darwin.NewNSString(input)
		if ns == nil {
			t.Fatal("NewNSString returned nil")
		}
		defer ns.Release()
		if got := darwinNSStringToGo(ns.ID()); got != input {
			t.Errorf("darwinNSStringToGo = %q, want %q", got, input)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		ns := darwin.NewNSString("")
		if ns == nil {
			t.Fatal("NewNSString returned nil")
		}
		defer ns.Release()
		if got := darwinNSStringToGo(ns.ID()); got != "" {
			t.Errorf("darwinNSStringToGo(\"\") = %q, want empty", got)
		}
	})
}

// TestDarwinBuildContentTypesArray verifies NSArray<UTType> construction.
// UTType is guaranteed available on the project's minimum macOS (12, go.mod: go 1.25).
func TestDarwinBuildContentTypesArray(t *testing.T) {
	t.Run("nil filters returns nil ID", func(t *testing.T) {
		if arr := darwinBuildContentTypesArray(nil); !arr.IsNil() {
			t.Error("expected nil array for nil filters")
		}
	})

	t.Run("empty filters returns nil ID", func(t *testing.T) {
		if arr := darwinBuildContentTypesArray([]FileTypeFilter{}); !arr.IsNil() {
			t.Error("expected nil array for empty filters")
		}
	})

	t.Run("filters with no valid extensions returns nil ID", func(t *testing.T) {
		arr := darwinBuildContentTypesArray([]FileTypeFilter{
			{Name: "Nothing", Extensions: []string{"", "*.", "."}},
		})
		if !arr.IsNil() {
			t.Error("expected nil array when all extensions are blank")
		}
	})

	t.Run("known extensions produce non-nil UTType array", func(t *testing.T) {
		filters := []FileTypeFilter{
			{Name: "Images", Extensions: []string{"*.png", "jpg"}},
		}
		arr := darwinBuildContentTypesArray(filters)
		if arr.IsNil() {
			t.Fatal("expected non-nil NSArray for known extensions")
		}
		count := arr.GetUint64(darwin.RegisterSelector("count"))
		if count == 0 {
			t.Error("expected at least one UTType in array")
		}
	})

	t.Run("multiple filters merged into one array", func(t *testing.T) {
		filters := []FileTypeFilter{
			{Name: "Images", Extensions: []string{"png", "jpg"}},
			{Name: "Docs", Extensions: []string{"pdf"}},
		}
		arr := darwinBuildContentTypesArray(filters)
		if arr.IsNil() {
			t.Fatal("expected non-nil NSArray")
		}
		count := arr.GetUint64(darwin.RegisterSelector("count"))
		if count == 0 {
			t.Error("expected UTType entries for known extensions")
		}
	})
}
