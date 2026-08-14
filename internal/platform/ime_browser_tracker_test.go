package platform

import "testing"

func TestBrowserIMETrackerCompositionEchoSuppression(t *testing.T) {
	var tracker browserIMETracker
	tracker.setEnabled(true)
	if !tracker.start() {
		t.Fatal("composition did not start")
	}
	if text, consumed := tracker.input("insertCompositionText", "拼"); text != "" || !consumed {
		t.Fatalf("composition input = (%q, %v), want consumed", text, consumed)
	}
	committed, canceled, ok := tracker.end("拼音")
	if !ok || canceled || committed != "拼音" {
		t.Fatalf("end = (%q, canceled=%v, ok=%v)", committed, canceled, ok)
	}
	if text, consumed := tracker.input("insertText", "拼音"); text != "" || !consumed {
		t.Fatalf("composition echo = (%q, %v), want consumed", text, consumed)
	}
	if text, consumed := tracker.input("insertText", "x"); text != "x" || consumed {
		t.Fatalf("ordinary input = (%q, %v), want x/unconsumed", text, consumed)
	}
}

func TestBrowserIMETrackerPreEndCommitAndCancel(t *testing.T) {
	var tracker browserIMETracker
	tracker.setEnabled(true)
	tracker.start()
	tracker.input("insertText", "候选")
	committed, canceled, ok := tracker.end("")
	if !ok || canceled || committed != "候选" {
		t.Fatalf("pre-end commit = (%q, canceled=%v, ok=%v)", committed, canceled, ok)
	}

	tracker.start()
	if !tracker.cancel() {
		t.Fatal("active composition did not cancel")
	}
	if _, canceled, ok := tracker.end(""); ok || canceled {
		t.Fatalf("canceled composition unexpectedly ended: canceled=%v, ok=%v", canceled, ok)
	}
}

func TestBrowserIMETrackerPreEndCommitDoesNotArmStaleEcho(t *testing.T) {
	var tracker browserIMETracker
	tracker.setEnabled(true)
	tracker.start()
	tracker.input("insertText", "同じ")
	if committed, canceled, ok := tracker.end(""); !ok || canceled || committed != "同じ" {
		t.Fatalf("pre-end commit = (%q, canceled=%v, ok=%v)", committed, canceled, ok)
	}
	if text, consumed := tracker.input("insertText", "同じ"); text != "同じ" || consumed {
		t.Fatalf("same ordinary input after pre-end commit = (%q, %v), want unconsumed", text, consumed)
	}
}

func TestBrowserIMETrackerDisableCancelsAndDropsState(t *testing.T) {
	var tracker browserIMETracker
	tracker.setEnabled(true)
	tracker.start()
	if !tracker.setEnabled(false) {
		t.Fatal("disable did not report active cancellation")
	}
	if tracker.start() {
		t.Fatal("disabled tracker accepted a composition")
	}
	if text, consumed := tracker.input("insertText", "text"); text != "" || !consumed {
		t.Fatalf("disabled input = (%q, %v), want ignored", text, consumed)
	}
}

func TestBrowserIMETrackerBoundaryPaths(t *testing.T) {
	var tracker browserIMETracker
	if tracker.ensureActive() {
		t.Fatal("disabled tracker became active")
	}
	tracker.setEnabled(true)
	if text, consumed := tracker.input("insertText", ""); text != "" || !consumed {
		t.Fatalf("empty input = (%q, %v), want consumed", text, consumed)
	}
	if committed, canceled, ok := tracker.end(""); ok || canceled || committed != "" {
		t.Fatalf("inactive end = (%q, canceled=%v, ok=%v)", committed, canceled, ok)
	}
	if !tracker.ensureActive() {
		t.Fatal("enabled tracker did not recover missing compositionstart")
	}
	if tracker.ensureActive() {
		t.Fatal("active tracker reopened composition")
	}
	tracker.cancel()
	if tracker.cancel() {
		t.Fatal("inactive tracker canceled")
	}
	tracker.start()
	if committed, canceled, ok := tracker.end(""); !ok || !canceled || committed != "" {
		t.Fatalf("empty active end = (%q, canceled=%v, ok=%v)", committed, canceled, ok)
	}
}
