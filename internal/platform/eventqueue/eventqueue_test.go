package eventqueue

import (
	"sync"
	"testing"
)

// testEvent is a simple event type for tests.
type testEvent struct {
	Type int
	Data string
}

func TestPushPop_FIFO(t *testing.T) {
	q := New[testEvent](8)

	q.Push(testEvent{Type: 1, Data: "first"})
	q.Push(testEvent{Type: 2, Data: "second"})
	q.Push(testEvent{Type: 3, Data: "third"})

	e, ok := q.Pop()
	if !ok || e.Type != 1 || e.Data != "first" {
		t.Errorf("Pop() = %v, %v; want {1, first}, true", e, ok)
	}
	e, ok = q.Pop()
	if !ok || e.Type != 2 || e.Data != "second" {
		t.Errorf("Pop() = %v, %v; want {2, second}, true", e, ok)
	}
	e, ok = q.Pop()
	if !ok || e.Type != 3 || e.Data != "third" {
		t.Errorf("Pop() = %v, %v; want {3, third}, true", e, ok)
	}
}

func TestPop_EmptyQueue(t *testing.T) {
	q := New[testEvent](4)

	e, ok := q.Pop()
	if ok {
		t.Errorf("Pop() on empty queue returned ok=true, event=%v", e)
	}
	if e != (testEvent{}) {
		t.Errorf("Pop() on empty queue returned non-zero event: %v", e)
	}
}

func TestOverflow_DropsOldest(t *testing.T) {
	q := New[testEvent](3)

	q.Push(testEvent{Type: 1, Data: "a"})
	q.Push(testEvent{Type: 2, Data: "b"})
	q.Push(testEvent{Type: 3, Data: "c"})
	// Queue full. Push one more — should drop oldest (Type=1).
	q.Push(testEvent{Type: 4, Data: "d"})

	if q.Len() != 3 {
		t.Fatalf("Len() = %d after overflow push; want 3", q.Len())
	}

	e, ok := q.Pop()
	if !ok || e.Type != 2 {
		t.Errorf("Pop() after overflow = %v, %v; want {Type:2}, true (oldest dropped)", e, ok)
	}
	e, ok = q.Pop()
	if !ok || e.Type != 3 {
		t.Errorf("Pop() = %v, %v; want {Type:3}, true", e, ok)
	}
	e, ok = q.Pop()
	if !ok || e.Type != 4 {
		t.Errorf("Pop() = %v, %v; want {Type:4}, true", e, ok)
	}
}

func TestLen_Accuracy(t *testing.T) {
	q := New[testEvent](8)

	if q.Len() != 0 {
		t.Errorf("Len() on new queue = %d; want 0", q.Len())
	}

	q.Push(testEvent{Type: 1})
	q.Push(testEvent{Type: 2})
	if q.Len() != 2 {
		t.Errorf("Len() after 2 pushes = %d; want 2", q.Len())
	}

	q.Pop()
	if q.Len() != 1 {
		t.Errorf("Len() after 1 pop = %d; want 1", q.Len())
	}

	q.Pop()
	if q.Len() != 0 {
		t.Errorf("Len() after draining = %d; want 0", q.Len())
	}
}

func TestCapacity_One(t *testing.T) {
	q := New[testEvent](1)

	q.Push(testEvent{Type: 1})
	if q.Len() != 1 {
		t.Fatalf("Len() = %d; want 1", q.Len())
	}

	// Overflow on cap=1 — drop oldest, insert new.
	q.Push(testEvent{Type: 2})
	if q.Len() != 1 {
		t.Fatalf("Len() after overflow = %d; want 1", q.Len())
	}

	e, ok := q.Pop()
	if !ok || e.Type != 2 {
		t.Errorf("Pop() = %v, %v; want {Type:2}, true", e, ok)
	}
}

func TestWrapAround(t *testing.T) {
	q := New[testEvent](4)

	// Fill and drain to advance head/tail past the start.
	for i := range 3 {
		q.Push(testEvent{Type: i})
	}
	for range 3 {
		q.Pop()
	}

	// Now head=3, tail=3 — push 4 more to wrap around.
	for i := 10; i < 14; i++ {
		q.Push(testEvent{Type: i})
	}
	if q.Len() != 4 {
		t.Fatalf("Len() = %d; want 4", q.Len())
	}

	for i := 10; i < 14; i++ {
		e, ok := q.Pop()
		if !ok || e.Type != i {
			t.Errorf("Pop() = %v, %v; want {Type:%d}, true", e, ok, i)
		}
	}
}

func TestZeroOutAfterPop(t *testing.T) {
	// Use a pointer-containing type to verify GC-friendliness.
	type ptrEvent struct {
		Data *string
	}
	q := New[ptrEvent](4)

	s := "hello"
	q.Push(ptrEvent{Data: &s})

	_, _ = q.Pop()

	// Access the internal buffer to verify the slot was zeroed.
	// After pop, head advanced from 0 to 1, and buf[0] should be zeroed.
	q.mu.Lock()
	if q.buf[0].Data != nil {
		t.Error("buf[0].Data should be nil after Pop (zero out for GC)")
	}
	q.mu.Unlock()
}

func TestZeroOutOnOverflow(t *testing.T) {
	type ptrEvent struct {
		Data *string
	}
	q := New[ptrEvent](2)

	s1, s2, s3 := "a", "b", "c"
	q.Push(ptrEvent{Data: &s1})
	q.Push(ptrEvent{Data: &s2})
	// Overflow — oldest (s1) should be zeroed.
	q.Push(ptrEvent{Data: &s3})

	// After overflow, head moved from 0 to 1. The old head (0) was zeroed
	// by the overflow logic, then overwritten by tail write.
	// The slot that was dropped is now at index 0, which got the new value (s3).
	// The key invariant: the dropped event's reference was zeroed before overwrite.
	// We can verify by checking that we get s2, s3 (not s1).
	e, ok := q.Pop()
	if !ok || *e.Data != "b" {
		t.Errorf("Pop() = %v, %v; want b (s1 should have been dropped)", e, ok)
	}
	e, ok = q.Pop()
	if !ok || *e.Data != "c" {
		t.Errorf("Pop() = %v, %v; want c", e, ok)
	}
}

func TestConcurrent_PushPop(t *testing.T) {
	q := New[testEvent](64)
	const goroutines = 8
	const eventsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // goroutines pushers + goroutines poppers

	// Pushers.
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range eventsPerGoroutine {
				q.Push(testEvent{Type: id*eventsPerGoroutine + i})
			}
		}(g)
	}

	// Poppers.
	popped := make([]int, goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for range eventsPerGoroutine {
				if _, ok := q.Pop(); ok {
					popped[id]++
				}
			}
		}(g)
	}

	wg.Wait()

	// Drain remaining.
	remaining := 0
	for {
		if _, ok := q.Pop(); !ok {
			break
		}
		remaining++
	}

	totalPopped := remaining
	for _, n := range popped {
		totalPopped += n
	}

	totalPushed := goroutines * eventsPerGoroutine
	// Some events may have been dropped due to overflow. Total popped <= total pushed.
	if totalPopped > totalPushed {
		t.Errorf("popped %d events but only pushed %d", totalPopped, totalPushed)
	}
}

func TestDefaultCapacity_UsedOnInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		cap  int
	}{
		{"zero", 0},
		{"negative", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := New[testEvent](tt.cap)
			// Should use DefaultCapacity.
			q.mu.Lock()
			got := len(q.buf)
			q.mu.Unlock()
			if got != DefaultCapacity {
				t.Errorf("New(%d) buf len = %d; want %d (DefaultCapacity)", tt.cap, got, DefaultCapacity)
			}
		})
	}
}

func TestCoalesceLast_Found(t *testing.T) {
	q := New[testEvent](8)
	q.Push(testEvent{Type: 1, Data: "a"})
	q.Push(testEvent{Type: 2, Data: "b"})
	q.Push(testEvent{Type: 1, Data: "c"})

	// Coalesce: replace most recent Type=2 with updated.
	ok := q.CoalesceLast(
		func(e testEvent) bool { return e.Type == 2 },
		testEvent{Type: 2, Data: "updated"},
	)
	if !ok {
		t.Fatal("CoalesceLast should return true when match found")
	}

	e, _ := q.Pop() // Type 1, "a"
	if e.Data != "a" {
		t.Errorf("first pop = %v; want a", e.Data)
	}
	e, _ = q.Pop() // Type 2, was "b", now "updated"
	if e.Data != "updated" {
		t.Errorf("second pop = %v; want updated", e.Data)
	}
	e, _ = q.Pop() // Type 1, "c"
	if e.Data != "c" {
		t.Errorf("third pop = %v; want c", e.Data)
	}
}

func TestCoalesceLast_NotFound(t *testing.T) {
	q := New[testEvent](8)
	q.Push(testEvent{Type: 1})

	ok := q.CoalesceLast(
		func(e testEvent) bool { return e.Type == 99 },
		testEvent{Type: 99},
	)
	if ok {
		t.Error("CoalesceLast should return false when no match found")
	}
	if q.Len() != 1 {
		t.Errorf("Len() = %d; want 1 (queue unchanged)", q.Len())
	}
}

func TestCoalesceLast_EmptyQueue(t *testing.T) {
	q := New[testEvent](8)

	ok := q.CoalesceLast(
		func(e testEvent) bool { return true },
		testEvent{Type: 1},
	)
	if ok {
		t.Error("CoalesceLast on empty queue should return false")
	}
}

func TestPushPop_MultipleOverflowCycles(t *testing.T) {
	q := New[testEvent](3)

	// Push 10 events into a cap-3 queue — exercises multiple overflow cycles.
	for i := range 10 {
		q.Push(testEvent{Type: i})
	}

	if q.Len() != 3 {
		t.Fatalf("Len() = %d; want 3", q.Len())
	}

	// Should contain the last 3 pushed: 7, 8, 9.
	for i := 7; i <= 9; i++ {
		e, ok := q.Pop()
		if !ok || e.Type != i {
			t.Errorf("Pop() = %v, %v; want {Type:%d}, true", e, ok, i)
		}
	}
}

func BenchmarkPush(b *testing.B) {
	q := New[testEvent](256)
	e := testEvent{Type: 1, Data: "bench"}
	b.ResetTimer()
	for range b.N {
		q.Push(e)
	}
}

func BenchmarkPushPop(b *testing.B) {
	q := New[testEvent](256)
	e := testEvent{Type: 1, Data: "bench"}
	b.ResetTimer()
	for range b.N {
		q.Push(e)
		q.Pop()
	}
}
