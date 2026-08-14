package platform

import (
	"context"
	"errors"
)

// ErrPrintUnavailable means that the selected native print service is not
// available in the current process/session.  The public App.Print boundary
// maps this setup condition to gogpu.ErrPrintUnsupported while preserving
// backend detail for direct platform callers.
var ErrPrintUnavailable = errors.New("gogpu: native printing unavailable")

// PrintDocument is the platform-layer representation of a complete document.
// The app layer copies Data before passing a request to an asynchronous
// backend, so implementations own the request bytes for the duration of a
// PrintJob.
type PrintDocument struct {
	Name     string
	MIMEType string
	Data     []byte
}

// PrintPageRange is an inclusive, one-based page range.
type PrintPageRange struct {
	From int
	To   int
}

// PrintOptions describes native UI ownership and document settings.
type PrintOptions struct {
	// Parent identifies the owning platform window. Zero means the application
	// parent when there is no primary window ID available.
	Parent WindowID
	Title  string
	// PageRanges is empty for all pages; ranges are inclusive and one-based.
	PageRanges []PrintPageRange
	// Copies is zero for the platform default (one copy).
	Copies int
}

// PrintRequest is the immutable request accepted by a native print backend.
type PrintRequest struct {
	Document PrintDocument
	Options  PrintOptions
}

// PrintJob is an asynchronous native print operation. Done receives exactly
// one terminal error and then closes: nil means success, context.Canceled
// means cancellation, and any other error means a platform/spool failure.
type PrintJob interface {
	Done() <-chan error
	Cancel()
}

// PrintManager is an optional PlatformManager capability. It is deliberately
// separate from PlatformManager so existing platform implementations remain
// source-compatible until their native print backend is ready.
//
// Implementations must honor ctx cancellation, keep the parent relationship
// for the lifetime of the native operation, and release all native resources
// before sending the terminal value on PrintJob.Done.
type PrintManager interface {
	StartPrint(ctx context.Context, request PrintRequest) (PrintJob, error)
}
