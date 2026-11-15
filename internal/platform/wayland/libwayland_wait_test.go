//go:build linux

package wayland

import (
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// fakeConn is one connection of the read protocol with no libwayland behind it:
// a pipe stands in for the socket, and the test drives "another thread read the
// socket" by making PrepareRead fail.
type fakeConn struct {
	fd      int
	prepare bool // what PrepareRead reports
	reads   int
	cancels int
}

func (f *fakeConn) GetDisplayFD() int { return f.fd }
func (f *fakeConn) PrepareRead() bool { return f.prepare }
func (f *fakeConn) FlushConn() error  { return nil }
func (f *fakeConn) ReadEvents()       { f.reads++ }
func (f *fakeConn) CancelRead()       { f.cancels++ }

func pipeReadEnd(t *testing.T) (read, write int) {
	t.Helper()
	var fds [2]int
	if err := unix.Pipe(fds[:]); err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { _ = unix.Close(fds[0]); _ = unix.Close(fds[1]) })
	return fds[0], fds[1]
}

// TestWaitForReadableQueuedEventsReturnImmediately is the ui#240 regression: a
// second thread read the socket, so events sit in the queue while the socket is
// empty. The wait must return at once instead of sleeping on the fd.
func TestWaitForReadableQueuedEventsReturnImmediately(t *testing.T) {
	read, _ := pipeReadEnd(t)
	conn := &fakeConn{fd: read, prepare: false}

	done := make(chan struct{})
	go func() {
		WaitForReadable([]EventConn{conn}, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReadable slept on an empty socket although events were queued")
	}
	if conn.reads != 0 || conn.cancels != 0 {
		t.Errorf("reads = %d, cancels = %d, want 0/0: no intent was taken", conn.reads, conn.cancels)
	}
}

// TestWaitForReadablePendingConnCancelsPreparedOnes checks that an early return
// releases the read intents already taken on the other connections.
func TestWaitForReadablePendingConnCancelsPreparedOnes(t *testing.T) {
	read1, _ := pipeReadEnd(t)
	read2, _ := pipeReadEnd(t)
	first := &fakeConn{fd: read1, prepare: true}
	second := &fakeConn{fd: read2, prepare: false}

	done := make(chan struct{})
	go func() {
		WaitForReadable([]EventConn{first, second}, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReadable did not return when the second connection had queued events")
	}
	if first.cancels != 1 {
		t.Errorf("first cancels = %d, want 1", first.cancels)
	}
	if second.cancels != 0 || second.reads != 0 {
		t.Errorf("second reads/cancels = %d/%d, want 0/0", second.reads, second.cancels)
	}
}

// TestWaitForReadableReadsWhenDataArrives drives the normal path: the intent is
// held, the socket becomes readable, the events are read.
func TestWaitForReadableReadsWhenDataArrives(t *testing.T) {
	read, write := pipeReadEnd(t)
	conn := &fakeConn{fd: read, prepare: true}
	if _, err := unix.Write(write, []byte{1}); err != nil {
		t.Fatalf("write: %v", err)
	}

	WaitForReadable([]EventConn{conn}, nil)

	if conn.reads != 1 || conn.cancels != 0 {
		t.Errorf("reads = %d, cancels = %d, want 1/0", conn.reads, conn.cancels)
	}
}

// TestWaitForReadableWakePipeCancelsRead covers WakeUp: the extra wake fd becomes
// ready, the read intent is released without reading.
func TestWaitForReadableWakePipeCancelsRead(t *testing.T) {
	read, _ := pipeReadEnd(t)
	wakeRead, wakeWrite := pipeReadEnd(t)
	conn := &fakeConn{fd: read, prepare: true}

	done := make(chan struct{})
	go func() {
		WaitForReadable([]EventConn{conn}, []unix.PollFd{{Fd: int32(wakeRead), Events: unix.POLLIN}})
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("WaitForReadable returned before the wake pipe was written")
	case <-time.After(100 * time.Millisecond):
	}

	if _, err := unix.Write(wakeWrite, []byte{1}); err != nil {
		t.Fatalf("write wake pipe: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReadable did not wake up on the wake pipe")
	}
	if conn.reads != 0 || conn.cancels != 1 {
		t.Errorf("reads = %d, cancels = %d, want 0/1", conn.reads, conn.cancels)
	}
}
