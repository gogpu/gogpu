// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package thread

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestCallReturnsValue(t *testing.T) {
	th := New()
	defer th.Stop()

	got := th.Call(func() any { return 42 })
	if got != 42 {
		t.Fatalf("Call() = %v, want 42", got)
	}
}

func TestCallVoidRuns(t *testing.T) {
	th := New()
	defer th.Stop()

	ran := false
	th.CallVoid(func() { ran = true })
	if !ran {
		t.Fatal("CallVoid did not run f")
	}
}

// panicMarker is a named frame that must appear in the captured stack (#481).
func panicMarker() {
	panic("boom")
}

func TestCallRepanicPreservesOriginalStack(t *testing.T) {
	th := New()
	defer th.Stop()

	stderr := captureStderr(t, func() {
		defer func() {
			rec := recover()
			if rec == nil {
				t.Fatal("expected panic")
			}
			if rec != "boom" {
				t.Fatalf("panic value = %v, want boom", rec)
			}
		}()
		th.Call(func() any {
			panicMarker()
			return nil
		})
	})

	if !strings.Contains(stderr, "panicMarker") {
		t.Fatalf("original stack missing panicMarker;\nstderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "panic on thread: boom") {
		t.Fatalf("expected panic header in stderr;\nstderr:\n%s", stderr)
	}
}

func TestCallVoidRepanicPreservesOriginalStack(t *testing.T) {
	th := New()
	defer th.Stop()

	stderr := captureStderr(t, func() {
		defer func() {
			rec := recover()
			if rec == nil {
				t.Fatal("expected panic")
			}
			if rec != "boom" {
				t.Fatalf("panic value = %v, want boom", rec)
			}
		}()
		th.CallVoid(panicMarker)
	})

	if !strings.Contains(stderr, "panicMarker") {
		t.Fatalf("original stack missing panicMarker;\nstderr:\n%s", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	return <-done
}
