//go:build windows

package platform

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestShowWindowsPrintDialogCopiesRangesAndReleasesDialogMemory(t *testing.T) {
	var got printDlgEx
	var freed []uintptr
	calls := windowsPrintSyscalls{
		printDlgEx: func(dlg *printDlgEx) uint32 {
			got = *dlg
			if dlg.nPageRanges != 2 {
				t.Fatalf("nPageRanges = %d, want 2", dlg.nPageRanges)
			}
			if dlg.lpPageRanges == 0 {
				t.Fatal("lpPageRanges is nil")
			}
			dlg.hDC = 41
			dlg.hDevMode = 42
			dlg.hDevNames = 43
			dlg.dwResultAction = pdResultPrint
			return 0
		},
		globalFree: func(handle uintptr) { freed = append(freed, handle) },
	}
	result, err := showWindowsPrintDialog(99, PrintRequest{
		Document: PrintDocument{MIMEType: "application/pdf", Data: []byte("pdf")},
		Options: PrintOptions{
			PageRanges: []PrintPageRange{{From: 2, To: 4}, {From: 8, To: 9}},
			Copies:     3,
		},
	}, calls)
	if err != nil {
		t.Fatalf("showWindowsPrintDialog() error = %v", err)
	}
	if result.hdc != 41 || result.canceled {
		t.Fatalf("result = %+v, want hdc 41 and not canceled", result)
	}
	if got.hwndOwner != 99 || got.flags&(pdReturnDC|pdUseDevModeCopiesAndCollate|pdPageNums) != (pdReturnDC|pdUseDevModeCopiesAndCollate|pdPageNums) {
		t.Fatalf("dialog owner/flags = hwnd %d flags %#x", got.hwndOwner, got.flags)
	}
	if got.nCopies != 3 {
		t.Fatalf("nCopies = %d, want 3", got.nCopies)
	}
	if got.nStartPage != startPageGeneral {
		t.Fatalf("nStartPage = %#x, want START_PAGE_GENERAL %#x", got.nStartPage, startPageGeneral)
	}
	if !reflect.DeepEqual(freed, []uintptr{42, 43}) {
		t.Fatalf("freed handles = %v, want [42 43]", freed)
	}
}

func TestShowWindowsPrintDialogCancelReleasesHDCAndMemory(t *testing.T) {
	var freed []uintptr
	var deleted []uintptr
	calls := windowsPrintSyscalls{
		printDlgEx: func(dlg *printDlgEx) uint32 {
			dlg.hDC = 7
			dlg.hDevMode = 8
			dlg.hDevNames = 9
			dlg.dwResultAction = pdResultCancel
			return 0
		},
		deleteDC:   func(hdc uintptr) int32 { deleted = append(deleted, hdc); return 1 },
		globalFree: func(handle uintptr) { freed = append(freed, handle) },
	}
	result, err := showWindowsPrintDialog(0, PrintRequest{Document: PrintDocument{MIMEType: "application/pdf", Data: []byte("pdf")}}, calls)
	if err != nil {
		t.Fatalf("showWindowsPrintDialog() error = %v", err)
	}
	if !result.canceled || result.hdc != 0 {
		t.Fatalf("result = %+v, want canceled result without HDC", result)
	}
	if !reflect.DeepEqual(deleted, []uintptr{7}) || !reflect.DeepEqual(freed, []uintptr{8, 9}) {
		t.Fatalf("cleanup = delete %v/free %v, want delete [7], free [8 9]", deleted, freed)
	}
}

func TestSpoolWindowsDocumentUsesGDIPassthroughAndCleansUp(t *testing.T) {
	var callsSeen []string
	var chunkSizes []int
	calls := windowsPrintSyscalls{
		startDoc: func(_ uintptr, info *printDocInfo) int32 {
			callsSeen = append(callsSeen, "start-doc")
			if info.lpszDatatype == nil || info.cbSize <= 0 {
				t.Fatal("StartDocW received incomplete DOCINFO")
			}
			return 1
		},
		startPage: func(uintptr) int32 { callsSeen = append(callsSeen, "start-page"); return 1 },
		escape: func(_ uintptr, escape int32, count uint32, input, _ uintptr) int32 {
			if escape != passthroughEscape {
				t.Fatalf("escape code = %d, want %d", escape, passthroughEscape)
			}
			callsSeen = append(callsSeen, "escape")
			if input == 0 {
				t.Fatal("PASSTHROUGH input pointer is nil")
			}
			chunkSizes = append(chunkSizes, int(count))
			return int32(count)
		},
		endPage:  func(uintptr) int32 { callsSeen = append(callsSeen, "end-page"); return 1 },
		endDoc:   func(uintptr) int32 { callsSeen = append(callsSeen, "end-doc"); return 1 },
		deleteDC: func(uintptr) int32 { callsSeen = append(callsSeen, "delete-dc"); return 1 },
		abortDoc: func(uintptr) int32 { callsSeen = append(callsSeen, "abort-doc"); return 1 },
	}
	job := newWindowsPrintJob()
	want := bytesForPrintTest(3*printEscapeChunkSize + 17)
	err := spoolWindowsDocument(context.Background(), PrintRequest{
		Document: PrintDocument{Name: "report.pdf", MIMEType: "application/pdf", Data: want},
	}, 55, job, calls)
	if err != nil {
		t.Fatalf("spoolWindowsDocument() error = %v", err)
	}
	if !reflect.DeepEqual(callsSeen, []string{"start-doc", "start-page", "escape", "escape", "escape", "escape", "end-page", "end-doc", "delete-dc"}) {
		t.Fatalf("call order = %v", callsSeen)
	}
	var got int
	for _, size := range chunkSizes {
		got += size
	}
	if got != len(want) {
		t.Fatalf("spooled byte count = %d, want %d", got, len(want))
	}
}

func TestSpoolWindowsDocumentCancellationAbortsBeforeStart(t *testing.T) {
	var aborted, deleted bool
	calls := windowsPrintSyscalls{
		startDoc: func(uintptr, *printDocInfo) int32 { t.Fatal("StartDocW called after cancellation"); return 0 },
		abortDoc: func(uintptr) int32 { aborted = true; return 1 },
		deleteDC: func(uintptr) int32 { deleted = true; return 1 },
	}
	job := newWindowsPrintJob()
	job.Cancel()
	err := spoolWindowsDocument(context.Background(), PrintRequest{Document: PrintDocument{Data: []byte("pdf"), MIMEType: "application/pdf"}}, 12, job, calls)
	if !errors.Is(err, context.Canceled) || !aborted || !deleted {
		t.Fatalf("error=%v aborted=%v deleted=%v, want canceled/abort/delete", err, aborted, deleted)
	}
}

func TestSpoolWindowsDocumentClearsAbortBeforeDeletingHDC(t *testing.T) {
	job := newWindowsPrintJob()
	var abortCalls, deleteCalls int
	job.setAbort(func() { abortCalls++ })
	calls := windowsPrintSyscalls{
		startDoc:  func(uintptr, *printDocInfo) int32 { return 1 },
		startPage: func(uintptr) int32 { return 1 },
		escape:    func(uintptr, int32, uint32, uintptr, uintptr) int32 { return 1 },
		endPage:   func(uintptr) int32 { return 1 },
		endDoc:    func(uintptr) int32 { return 1 },
		deleteDC: func(uintptr) int32 {
			deleteCalls++
			// If clearAbort ran after DeleteDC, this cancellation would invoke the
			// hook against the released HDC.
			job.Cancel()
			return 1
		},
	}
	if err := spoolWindowsDocument(context.Background(), PrintRequest{
		Document: PrintDocument{MIMEType: "application/pdf", Data: []byte("pdf")},
	}, 12, job, calls); err != nil {
		t.Fatalf("spoolWindowsDocument() error = %v", err)
	}
	if abortCalls != 0 || deleteCalls != 1 {
		t.Fatalf("abort calls=%d delete calls=%d, want 0/1", abortCalls, deleteCalls)
	}
}

func bytesForPrintTest(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}
