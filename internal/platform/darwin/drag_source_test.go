//go:build darwin

package darwin

import (
	"path/filepath"
	"testing"
)

func TestDraggingFrameAt_NonZero(t *testing.T) {
	loc := MakePoint(100, 200)

	frame := draggingFrameAt(loc, defaultDragIconPoints, 0)
	if frame.Size.Width <= 0 || frame.Size.Height <= 0 {
		t.Fatalf("draggingFrame must be non-zero, got %+v", frame)
	}
	if frame.Size.Width != defaultDragIconPoints || frame.Size.Height != defaultDragIconPoints {
		t.Fatalf("unexpected size: %+v", frame.Size)
	}
	if frame.Origin.X != 100-16 || frame.Origin.Y != 200-16 {
		t.Fatalf("expected centered origin, got %+v", frame.Origin)
	}
}

func TestDraggingFrameAt_ZeroSizeFallback(t *testing.T) {
	// AppKit aborts on {{0,0},{0,0}} — size < 1 must fall back to default.
	for _, size := range []CGFloat{0, -1, 0.5} {
		frame := draggingFrameAt(MakePoint(0, 0), size, 0)
		if frame.Size.Width < 1 || frame.Size.Height < 1 {
			t.Fatalf("size=%v produced zero/invalid frame: %+v", size, frame)
		}
		if frame.Size.Width != defaultDragIconPoints {
			t.Fatalf("size=%v: expected fallback %v, got %v", size, defaultDragIconPoints, frame.Size.Width)
		}
	}
}

func TestDraggingFrameAt_MultiItemOffset(t *testing.T) {
	loc := MakePoint(50, 50)
	f0 := draggingFrameAt(loc, 32, 0)
	f1 := draggingFrameAt(loc, 32, 1)
	f2 := draggingFrameAt(loc, 32, 2)

	if f1.Origin.X != f0.Origin.X+4 || f1.Origin.Y != f0.Origin.Y-4 {
		t.Fatalf("index 1 offset wrong: f0=%+v f1=%+v", f0.Origin, f1.Origin)
	}
	if f2.Origin.X != f0.Origin.X+8 || f2.Origin.Y != f0.Origin.Y-8 {
		t.Fatalf("index 2 offset wrong: f0=%+v f2=%+v", f0.Origin, f2.Origin)
	}
	for i, f := range []NSRect{f0, f1, f2} {
		if f.Size.Width <= 0 || f.Size.Height <= 0 {
			t.Fatalf("item %d has zero frame: %+v", i, f)
		}
	}
}

func TestAbsoluteDragPaths(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "tmp", "file.txt")
	got := absoluteDragPaths([]string{
		abs,
		"relative/file.txt",
		"",
		"./also-relative",
		filepath.Join(string(filepath.Separator), "var", "log"),
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 absolute paths, got %d: %v", len(got), got)
	}
	if got[0] != abs {
		t.Fatalf("got[0]=%q want %q", got[0], abs)
	}
	if !filepath.IsAbs(got[1]) {
		t.Fatalf("got[1] not absolute: %q", got[1])
	}
}

func TestAbsoluteDragPaths_AllRelative(t *testing.T) {
	got := absoluteDragPaths([]string{"a", "b/c", ""})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestDragSourceReentrancyGuard(t *testing.T) {
	view := ID(0xdeadbeef)

	if !tryAcquireDragSource(view) {
		t.Fatal("first acquire should succeed")
	}
	if !isDragSourceActive(view) {
		t.Fatal("expected active after acquire")
	}
	if tryAcquireDragSource(view) {
		t.Fatal("second acquire must fail while active")
	}

	releaseDragSource(view)
	if isDragSourceActive(view) {
		t.Fatal("expected inactive after release")
	}
	if !tryAcquireDragSource(view) {
		t.Fatal("acquire after release should succeed")
	}
	releaseDragSource(view)

	// Nil view is never acquirable.
	if tryAcquireDragSource(0) {
		t.Fatal("nil view must not acquire")
	}
}
