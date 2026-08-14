package platform

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrintJobCancelIsIdempotentAndTerminal(t *testing.T) {
	job := newPrintJob()
	var calls atomic.Int32
	job.setCancel(func() { calls.Add(1) })

	job.Cancel()
	job.Cancel()
	job.complete(errors.New("native success raced cancellation"))

	if got := <-job.Done(); !errors.Is(got, context.Canceled) {
		t.Fatalf("Done() = %v, want context.Canceled", got)
	}
	if _, open := <-job.Done(); open {
		t.Fatal("Done channel should close after one terminal value")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cancel hook calls = %d, want 1", got)
	}
}

func TestPrintJobCompletionWinsBeforeCancellation(t *testing.T) {
	job := newPrintJob()
	var calls atomic.Int32
	job.setCancel(func() { calls.Add(1) })
	job.complete(nil)
	job.Cancel()

	if got := <-job.Done(); got != nil {
		t.Fatalf("Done() = %v, want nil", got)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("cancel hook calls = %d, want 0 after completion", got)
	}
}

func TestPrintJobCancellationBeforeNativeHook(t *testing.T) {
	job := newPrintJob()
	job.Cancel()
	called := make(chan struct{})
	job.setCancel(func() { close(called) })

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("cancel hook was not called after late installation")
	}
	job.complete(nil)
	if got := <-job.Done(); !errors.Is(got, context.Canceled) {
		t.Fatalf("Done() = %v, want context.Canceled", got)
	}
}

func TestWatchPrintContextCancelsJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := newPrintJob()
	hookCalled := make(chan struct{})
	job.setCancel(func() { close(hookCalled) })
	watchPrintContext(ctx, job)
	cancel()

	select {
	case <-hookCalled:
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not reach native hook")
	}
	job.complete(nil)
	if got := <-job.Done(); !errors.Is(got, context.Canceled) {
		t.Fatalf("Done() = %v, want context.Canceled", got)
	}
}

func TestPrintJobConcurrentCancelCallsOneHook(t *testing.T) {
	job := newPrintJob()
	var calls atomic.Int32
	job.setCancel(func() { calls.Add(1) })

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job.Cancel()
		}()
	}
	wg.Wait()
	job.complete(nil)

	if got := calls.Load(); got != 1 {
		t.Fatalf("cancel hook calls = %d, want 1", got)
	}
	if got := <-job.Done(); !errors.Is(got, context.Canceled) {
		t.Fatalf("Done() = %v, want context.Canceled", got)
	}
}

func TestPrintJobCompletionWaitsForCancellationHook(t *testing.T) {
	job := newPrintJob()
	hookStarted := make(chan struct{})
	releaseHook := make(chan struct{})
	job.setCancel(func() {
		close(hookStarted)
		<-releaseHook
	})

	go job.Cancel()
	select {
	case <-hookStarted:
	case <-time.After(time.Second):
		t.Fatal("cancellation hook did not start")
	}

	completed := make(chan struct{})
	go func() {
		job.complete(nil)
		close(completed)
	}()
	select {
	case <-completed:
		t.Fatal("completion published while cancellation hook was still running")
	case <-time.After(20 * time.Millisecond):
	}

	close(releaseHook)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("completion remained blocked after cancellation hook returned")
	}
	if got := <-job.Done(); !errors.Is(got, context.Canceled) {
		t.Fatalf("Done() = %v, want context.Canceled", got)
	}
}
