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

// TestDarwinBuildExtArray verifies NSMutableArray construction from FileTypeFilter slices.
func TestDarwinBuildExtArray(t *testing.T) {
	t.Run("nil filters returns nil ID", func(t *testing.T) {
		arr := darwinBuildExtArray(nil)
		if !arr.IsNil() {
			t.Error("expected nil ID for nil filters")
		}
	})

	t.Run("empty filters returns nil ID", func(t *testing.T) {
		arr := darwinBuildExtArray([]FileTypeFilter{})
		if !arr.IsNil() {
			t.Error("expected nil ID for empty filters")
		}
	})

	t.Run("filters with no valid extensions returns nil ID", func(t *testing.T) {
		arr := darwinBuildExtArray([]FileTypeFilter{
			{Name: "Nothing", Extensions: []string{"", "*.", "."}},
		})
		if !arr.IsNil() {
			t.Error("expected nil ID when all extensions are blank")
		}
	})

	t.Run("extensions are stripped and dedotted", func(t *testing.T) {
		filters := []FileTypeFilter{
			{Name: "Images", Extensions: []string{"*.png", "jpg", ".gif"}},
		}
		arr := darwinBuildExtArray(filters)
		if arr.IsNil() {
			t.Fatal("expected non-nil NSArray")
		}

		count := arr.GetUint64(darwin.RegisterSelector("count"))
		if count != 3 {
			t.Fatalf("array count = %d, want 3", count)
		}

		want := []string{"png", "jpg", "gif"}
		for i, w := range want {
			elem := arr.SendUint(darwin.RegisterSelector("objectAtIndex:"), uint64(i))
			got := darwinNSStringToGo(elem)
			if got != w {
				t.Errorf("element[%d] = %q, want %q", i, got, w)
			}
		}
	})

	t.Run("multiple filters are merged into one array", func(t *testing.T) {
		filters := []FileTypeFilter{
			{Name: "Images", Extensions: []string{"png", "jpg"}},
			{Name: "Docs", Extensions: []string{"pdf", "md"}},
		}
		arr := darwinBuildExtArray(filters)
		if arr.IsNil() {
			t.Fatal("expected non-nil NSArray")
		}

		count := arr.GetUint64(darwin.RegisterSelector("count"))
		if count != 4 {
			t.Fatalf("array count = %d, want 4", count)
		}
	})
}
