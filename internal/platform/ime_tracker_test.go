package platform

import (
	"testing"
	"unicode/utf16"
)

func TestIMETrackerLifecycleAndResultEchoSuppression(t *testing.T) {
	var tracker imeTracker
	if tracker.start() {
		t.Fatal("disabled tracker started a composition")
	}
	if canceled := tracker.setEnabled(true); canceled {
		t.Fatal("enabling tracker canceled a composition")
	}
	if !tracker.start() {
		t.Fatal("enabled tracker did not start")
	}
	if tracker.start() {
		t.Fatal("duplicate start opened a second session")
	}

	tracker.addResult("你")
	tracker.addResult("好")
	committed, ok := tracker.end()
	if !ok || committed != "你好" {
		t.Fatalf("end = (%q, %v), want (你好, true)", committed, ok)
	}

	for _, unit := range utf16.Encode([]rune("你好")) {
		if !tracker.consumeCharUnit(unit) {
			t.Fatalf("result echo code unit %#x was not suppressed", unit)
		}
	}
	if tracker.consumeCharUnit('你') {
		t.Fatal("suppression sequence remained after consuming the result")
	}
}

func TestIMETrackerSurrogateAndMismatch(t *testing.T) {
	var tracker imeTracker
	tracker.setEnabled(true)
	tracker.start()
	tracker.addResult("😀")
	units := utf16.Encode([]rune("😀"))
	if len(units) != 2 || !tracker.consumeCharUnit(units[0]) || !tracker.consumeCharUnit(units[1]) {
		t.Fatalf("supplementary result was not consumed as UTF-16 pair: %#x", units)
	}

	tracker.addResult("中")
	if tracker.consumeCharUnit('x') {
		t.Fatal("mismatched code unit was suppressed")
	}
	if tracker.consumeCharUnit('中') {
		t.Fatal("mismatch left a stale suppression sequence")
	}
}

func TestIMETrackerCancelAndDisable(t *testing.T) {
	var tracker imeTracker
	tracker.setEnabled(true)
	tracker.start()
	tracker.addResult("未完成")
	if !tracker.cancel() {
		t.Fatal("active composition was not canceled")
	}
	for _, unit := range utf16.Encode([]rune("未完成")) {
		if !tracker.consumeCharUnit(unit) {
			t.Fatalf("canceled result echo %#x was not suppressed", unit)
		}
	}
	if tracker.cancel() {
		t.Fatal("second cancel reported an inactive composition")
	}
	if committed, ok := tracker.end(); ok || committed != "" {
		t.Fatalf("canceled composition ended as (%q, %v)", committed, ok)
	}

	tracker.start()
	tracker.addResult("x")
	if !tracker.setEnabled(false) {
		t.Fatal("disabling active IME did not report cancellation")
	}
	if !tracker.consumeCharUnit('x') {
		t.Fatal("disabled result echo was not suppressed")
	}
	if tracker.start() {
		t.Fatal("disabled tracker accepted a new composition")
	}
	tracker.beforeKeyDown() // stale result echoes must not cross a new gesture
}

func TestIMETrackerBeforeKeyDownExpiresUnmatchedEcho(t *testing.T) {
	var tracker imeTracker
	tracker.setEnabled(true)
	tracker.start()
	tracker.addResult("a")
	tracker.end()
	tracker.beforeKeyDown()
	if tracker.consumeCharUnit('a') {
		t.Fatal("suppression survived the next keyboard gesture")
	}
}

func TestIMETrackerEnsureActiveWithoutStartMessage(t *testing.T) {
	var tracker imeTracker
	tracker.setEnabled(true)
	if !tracker.ensureActive() {
		t.Fatal("enabled tracker did not recover a composition without START")
	}
	if tracker.ensureActive() {
		t.Fatal("ensureActive reopened an already-active composition")
	}
}

func TestIMETrackerPostEndCharResult(t *testing.T) {
	var tracker imeTracker
	tracker.setEnabled(true)
	tracker.start()
	if committed, ok := tracker.end(); !ok || committed != "" {
		t.Fatalf("empty end = (%q, %v)", committed, ok)
	}
	tracker.beginCharResult()
	if !tracker.hasPendingCharResult() {
		t.Fatal("beginCharResult did not mark a pending result")
	}
	if !tracker.consumeCharResultUnit('x') {
		t.Fatal("single post-END result code unit was not collected")
	}
	if committed, ok := tracker.finishCharResult(); !ok || committed != "x" {
		t.Fatalf("single post-END commit = (%q, %v)", committed, ok)
	}
	tracker.beginCharResult()
	if !tracker.consumeCharResultUnits(utf16.Encode([]rune("你好"))) {
		t.Fatal("post-END result was not collected")
	}
	committed, ok := tracker.finishCharResult()
	if !ok || committed != "你好" {
		t.Fatalf("post-END commit = (%q, %v), want 你好", committed, ok)
	}
}

func TestIMETrackerBoundaryPaths(t *testing.T) {
	var tracker imeTracker
	if tracker.ensureActive() {
		t.Fatal("disabled tracker became active")
	}
	tracker.setEnabled(true)
	tracker.addResult("")
	if tracker.consumeCharResultUnit('x') || tracker.consumeCharResultUnits(nil) {
		t.Fatal("inactive post-END result consumed input")
	}
	if committed, ok := tracker.finishCharResult(); ok || committed != "" {
		t.Fatalf("inactive finish = (%q, %v)", committed, ok)
	}
	tracker.beginCharResult()
	if tracker.consumeCharResultUnits(nil) {
		t.Fatal("empty post-END unit list consumed")
	}
	if committed, ok := tracker.finishCharResult(); !ok || committed != "" {
		t.Fatalf("empty post-END finish = (%q, %v)", committed, ok)
	}
}
