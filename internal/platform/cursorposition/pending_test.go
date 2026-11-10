package cursorposition

import "testing"

func TestPendingMotionAtSetPositionArrives(t *testing.T) {
	var pending Pending
	pending.Add(10, 20)

	if !pending.Arrived(10, 20) {
		t.Fatal("motion at the set position not taken for its own")
	}
	if pending.Arrived(10, 20) {
		t.Error("a second motion at the same point taken for the same position")
	}
}

func TestPendingMotionElsewhereIsTheUsers(t *testing.T) {
	var pending Pending
	pending.Add(10, 20)

	if pending.Arrived(11, 20) {
		t.Error("motion away from the set position taken for its own")
	}
	// The position's own motion may still come: it was queued after the user's.
	if !pending.Arrived(10, 20) {
		t.Error("the position's motion after the user's not taken for its own")
	}
}

func TestPendingArrivalForgetsEarlierPositions(t *testing.T) {
	var pending Pending
	pending.Add(1, 1)
	pending.Add(2, 2)
	pending.Add(3, 3)

	// The motion of position 2 arrives; position 1's never will.
	if !pending.Arrived(2, 2) {
		t.Fatal("motion at the second position not taken for it")
	}
	if pending.Arrived(1, 1) {
		t.Error("an earlier position still pending after a later one arrived")
	}
	if !pending.Arrived(3, 3) {
		t.Error("a later position forgotten when an earlier one arrived")
	}
}

func TestPendingRepeatedPositionArrivesInOrder(t *testing.T) {
	var pending Pending
	pending.Add(1, 1)
	pending.Add(2, 2)
	pending.Add(1, 1)

	for i, point := range [][2]int32{{1, 1}, {2, 2}, {1, 1}} {
		if !pending.Arrived(point[0], point[1]) {
			t.Errorf("motion %d at %v not taken for its position", i, point)
		}
	}
}

func TestPendingClearForgetsAll(t *testing.T) {
	var pending Pending
	pending.Add(1, 1)

	pending.Clear()

	if pending.Arrived(1, 1) {
		t.Error("a position still pending after Clear")
	}
}

func TestPendingDropsOldestBeyondLimit(t *testing.T) {
	var pending Pending
	for i := range int32(limit + 1) {
		pending.Add(i, i)
	}

	if pending.Arrived(0, 0) {
		t.Error("the oldest position kept beyond the limit")
	}
	if !pending.Arrived(limit, limit) {
		t.Error("the newest position dropped")
	}
}
