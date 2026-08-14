package gogpu

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogpu/gogpu/internal/platform"
)

const (
	// PrintMIMETypePDF identifies a PDF print document.
	PrintMIMETypePDF = "application/pdf"
)

// PrintDocument is the complete document submitted to a native print
// operation. Data is copied when Print is called, so the caller may reuse its
// source buffer after Print returns. A document is not rendered by gogpu;
// callers (for example, gg or ui) provide the final bytes and media type.
type PrintDocument struct {
	// Name is the user-facing document name. It may be empty when the native
	// backend supplies a default.
	Name string
	// MIMEType identifies the document bytes. PrintMIMETypePDF is the portable
	// format currently expected by native backends; other document types may be
	// added without changing the contract.
	MIMEType string
	// Data contains the complete document bytes. The bytes must remain a valid
	// document for the declared MIMEType.
	Data []byte
}

// NewPDFDocument creates a print document backed by PDF bytes. It copies pdf
// so the returned document owns its input data.
func NewPDFDocument(name string, pdf []byte) PrintDocument {
	return PrintDocument{
		Name:     name,
		MIMEType: PrintMIMETypePDF,
		Data:     append([]byte(nil), pdf...),
	}
}

// PrintPageRange is an inclusive, one-based page range. An empty range slice
// means that no explicit range was supplied; native backends print all pages.
type PrintPageRange struct {
	From int
	To   int
}

// PrintOptions controls the native print operation.
type PrintOptions struct {
	// Parent identifies the window that owns the native print UI. Zero uses the
	// app's primary window when one exists, or the application parent otherwise.
	Parent WindowID
	// Title is an optional native dialog title.
	Title string
	// PageRanges limits printing to these inclusive, one-based ranges. An empty
	// slice means all pages.
	PageRanges []PrintPageRange
	// Copies requests the number of copies. Zero uses the platform default (one).
	Copies int
}

// PrintJob represents an accepted native print operation.
//
// StartPrint returns after the platform has accepted the request; the native
// dialog/spool operation may still be running. Done returns a channel that
// receives exactly one value and is then closed: nil means the operation
// completed, context.Canceled means it was canceled, and any other error is a
// platform or spool failure. Cancel is idempotent and has no effect after Done
// has completed.
type PrintJob interface {
	Done() <-chan error
	Cancel()
}

var (
	// ErrPrintUnsupported means the selected platform has no native print
	// implementation yet. It is returned synchronously and no job is created.
	ErrPrintUnsupported = errors.New("gogpu: native printing unsupported")
	// ErrInvalidPrintDocument means the document has no media type or bytes.
	ErrInvalidPrintDocument = errors.New("gogpu: invalid print document")
	// ErrInvalidPrintOptions means a print option is outside the contract.
	ErrInvalidPrintOptions = errors.New("gogpu: invalid print options")
	// ErrNilPrintContext means a nil context was passed to Print. Callers must
	// pass context.Background() when they do not need cancellation.
	ErrNilPrintContext = errors.New("gogpu: nil print context")
)

func (d PrintDocument) validate() error {
	if d.MIMEType == "" || len(d.Data) == 0 {
		return fmt.Errorf("%w: MIMEType and Data are required", ErrInvalidPrintDocument)
	}
	return nil
}

func (o PrintOptions) validate() error {
	if o.Copies < 0 {
		return fmt.Errorf("%w: Copies must not be negative", ErrInvalidPrintOptions)
	}
	for _, r := range o.PageRanges {
		if r.From <= 0 || r.To < r.From {
			return fmt.Errorf("%w: page range %d-%d is not positive and inclusive", ErrInvalidPrintOptions, r.From, r.To)
		}
	}
	return nil
}

// Print submits a complete document to the platform's native print contract.
// The call is asynchronous after request acceptance: wait on PrintJob.Done or
// cancel it through the returned job/context. Before Run, the primary parent
// is unavailable and a backend may reject a request that requires a window.
// gogpu does not generate or paginate documents.
func (a *App) Print(ctx context.Context, document PrintDocument, opts PrintOptions) (PrintJob, error) {
	if ctx == nil {
		return nil, ErrNilPrintContext
	}
	if err := document.validate(); err != nil {
		return nil, err
	}
	if err := opts.validate(); err != nil {
		return nil, err
	}
	if a.manager == nil {
		return nil, ErrPrintUnsupported
	}
	manager, ok := a.manager.(platform.PrintManager)
	if !ok {
		return nil, ErrPrintUnsupported
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.Parent == 0 && a.platWindow != nil {
		opts.Parent = WindowID(a.platWindow.ID())
	}

	// Copy all caller-owned slices/bytes before an asynchronous backend can
	// retain the request. This makes the lifetime boundary explicit.
	request := platform.PrintRequest{
		Document: platform.PrintDocument{
			Name:     document.Name,
			MIMEType: document.MIMEType,
			Data:     append([]byte(nil), document.Data...),
		},
		Options: platform.PrintOptions{
			Parent: platform.WindowID(opts.Parent),
			Title:  opts.Title,
			Copies: opts.Copies,
		},
	}
	if len(opts.PageRanges) != 0 {
		request.Options.PageRanges = make([]platform.PrintPageRange, len(opts.PageRanges))
		for i, r := range opts.PageRanges {
			request.Options.PageRanges[i] = platform.PrintPageRange{From: r.From, To: r.To}
		}
	}
	return manager.StartPrint(ctx, request)
}
