package window

import (
	"runtime"
	"testing"
)

// BenchmarkDefaultConfig measures the cost of creating the default
// window configuration. Called once per window creation.
func BenchmarkDefaultConfig(b *testing.B) {
	b.ReportAllocs()
	var result Config
	for b.Loop() {
		result = DefaultConfig()
	}
	runtime.KeepAlive(result)
}

// BenchmarkWindowNew measures the cost of creating a new Window struct.
// This is the CPU-side allocation only (no platform window creation).
func BenchmarkWindowNew(b *testing.B) {
	b.ReportAllocs()
	cfg := DefaultConfig()
	b.ResetTimer()
	for b.Loop() {
		w, _ := New(cfg)
		runtime.KeepAlive(w)
	}
}

// BenchmarkWindowSize measures the cost of reading window dimensions.
// Called per-frame for viewport calculations.
func BenchmarkWindowSize(b *testing.B) {
	b.ReportAllocs()
	w, _ := New(DefaultConfig())
	var width, height int
	for b.Loop() {
		width, height = w.Size()
	}
	runtime.KeepAlive(width)
	runtime.KeepAlive(height)
}

// BenchmarkWindowSetSize measures the cost of setting window size.
func BenchmarkWindowSetSize(b *testing.B) {
	b.ReportAllocs()
	w, _ := New(DefaultConfig())
	for b.Loop() {
		w.SetSize(1024, 768)
	}
}

// BenchmarkWindowTitle measures the cost of reading window title.
func BenchmarkWindowTitle(b *testing.B) {
	b.ReportAllocs()
	w, _ := New(DefaultConfig())
	var result string
	for b.Loop() {
		result = w.Title()
	}
	runtime.KeepAlive(result)
}

// BenchmarkEventCreation measures the cost of creating a window event.
func BenchmarkEventCreation(b *testing.B) {
	b.ReportAllocs()
	b.Run("Resize", func(b *testing.B) {
		b.ReportAllocs()
		var result Event
		for b.Loop() {
			result = Event{
				Type:   EventTypeResize,
				Width:  1024,
				Height: 768,
			}
		}
		runtime.KeepAlive(result)
	})
	b.Run("Move", func(b *testing.B) {
		b.ReportAllocs()
		var result Event
		for b.Loop() {
			result = Event{
				Type: EventTypeMove,
				X:    100,
				Y:    200,
			}
		}
		runtime.KeepAlive(result)
	})
	b.Run("Close", func(b *testing.B) {
		b.ReportAllocs()
		var result Event
		for b.Loop() {
			result = Event{Type: EventTypeClose}
		}
		runtime.KeepAlive(result)
	})
}
