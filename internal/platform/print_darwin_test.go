//go:build darwin

package platform

import (
	"context"
	"errors"
	"testing"
)

func TestDarwinStartPrintRejectsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&darwinPlatform{}).StartPrint(ctx, PrintRequest{
		Document: PrintDocument{MIMEType: "application/pdf", Data: []byte("pdf")},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StartPrint() error = %v, want context.Canceled", err)
	}
}

func TestDarwinStartPrintRejectsUnsupportedOrInvalidInputBeforeNativeSetup(t *testing.T) {
	backend := &darwinPlatform{}
	if _, err := backend.StartPrint(context.Background(), PrintRequest{
		Document: PrintDocument{MIMEType: "text/plain", Data: []byte("text")},
	}); !errors.Is(err, ErrPrintUnsupportedMIME) {
		t.Fatalf("unsupported MIME error = %v, want ErrPrintUnsupportedMIME", err)
	}
	if _, err := backend.StartPrint(context.Background(), PrintRequest{
		Document: PrintDocument{MIMEType: "application/pdf", Data: []byte("pdf")},
		Options:  PrintOptions{PageRanges: []PrintPageRange{{From: 0, To: 1}}},
	}); !errors.Is(err, ErrPrintInvalidOptions) {
		t.Fatalf("invalid range error = %v, want ErrPrintInvalidOptions", err)
	}
}

func TestDarwinStartPrintRejectsUnavailableParentWithoutCreatingJob(t *testing.T) {
	job, err := (&darwinPlatform{}).StartPrint(context.Background(), PrintRequest{
		Document: PrintDocument{MIMEType: "application/pdf", Data: []byte("pdf")},
	})
	if job != nil {
		t.Fatal("unavailable backend returned a PrintJob")
	}
	if !errors.Is(err, ErrPrintUnavailable) {
		t.Fatalf("unavailable backend error = %v, want ErrPrintUnavailable", err)
	}
}
