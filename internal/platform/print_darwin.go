//go:build darwin

package platform

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogpu/gogpu/internal/platform/darwin"
)

var (
	// ErrPrintUnsupportedMIME means this backend cannot consume the submitted
	// document format. PDFKit is the first native format supported by P1M.
	ErrPrintUnsupportedMIME = errors.New("gogpu: macOS print backend unsupported document type")
	// ErrPrintInvalidDocument means the accepted document payload is empty or
	// otherwise cannot be handed to PDFKit.
	ErrPrintInvalidDocument = errors.New("gogpu: macOS print document invalid")
	// ErrPrintParentUnavailable means a non-zero parent WindowID no longer maps
	// to a live macOS window owned by this platform manager.
	ErrPrintParentUnavailable = errors.New("gogpu: macOS print parent unavailable")
	// ErrPrintInvalidOptions protects direct internal callers that bypass the
	// public App.Print validator.
	ErrPrintInvalidOptions = errors.New("gogpu: macOS print options invalid")
)

const darwinPrintMIMETypePDF = "application/pdf"

var _ PrintManager = (*darwinPlatform)(nil)

// StartPrint implements the optional macOS native print capability. PDFKit
// consumes the caller-supplied complete PDF and creates an NSPrintOperation;
// AppKit owns the print panel and spooling lifecycle. Setup is performed on
// the main thread synchronously so malformed documents and missing native
// objects are returned as acceptance errors. The modal operation itself is
// queued asynchronously, allowing the caller to receive a PrintJob before the
// panel is dismissed.
func (p *darwinPlatform) StartPrint(ctx context.Context, request PrintRequest) (PrintJob, error) {
	if ctx == nil {
		return nil, context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Document.MIMEType != darwinPrintMIMETypePDF {
		return nil, fmt.Errorf("%w: %q", ErrPrintUnsupportedMIME, request.Document.MIMEType)
	}
	if len(request.Document.Data) == 0 {
		return nil, fmt.Errorf("%w: empty document", ErrPrintInvalidDocument)
	}
	if request.Options.Copies < 0 {
		return nil, fmt.Errorf("%w: negative copies", ErrPrintInvalidOptions)
	}
	for _, r := range request.Options.PageRanges {
		if r.From <= 0 || r.To < r.From {
			return nil, fmt.Errorf("%w: page range %d-%d", ErrPrintInvalidOptions, r.From, r.To)
		}
	}
	if p == nil {
		return nil, ErrPrintUnavailable
	}
	p.mu.RLock()
	app := p.app
	p.mu.RUnlock()
	if app == nil || !app.IsInitialized() {
		return nil, ErrPrintUnavailable
	}

	job := newPrintJob()
	var (
		parent   darwin.ID
		handle   *darwin.PrintHandle
		setupErr error
	)
	setup := func() {
		parent, setupErr = p.printParent(request.Options.Parent)
		if setupErr != nil {
			return
		}
		handle, setupErr = darwin.NewPrintHandle(darwin.PrintRequest{
			Name:       request.Document.Name,
			Data:       request.Document.Data,
			Title:      request.Options.Title,
			Copies:     request.Options.Copies,
			PageRanges: toDarwinPrintRanges(request.Options.PageRanges),
		}, parent)
	}

	// AppKit's main-thread check is cheap and avoids a needless selector hop
	// from normal App.Print calls made inside OnUpdate/OnDraw. Background callers
	// wait only for setup; the actual modal operation is always asynchronous.
	if darwin.IsMainThread() {
		setup()
	} else if err := darwin.PerformOnMain(setup, true); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPrintUnavailable, err)
	}
	if setupErr != nil {
		return nil, setupErr
	}
	if handle == nil {
		return nil, ErrPrintUnavailable
	}

	job.setCancel(func() {
		// NSPPrintOperation has no public cancel method. Sending cancel: to its
		// panel is the documented AppKit action and is safe only on the main
		// thread. If the panel is already gone, the terminal callback still maps
		// the caller's cancellation request to context.Canceled.
		_ = darwin.PerformOnMain(handle.Cancel, false)
	})

	run := func() {
		if job.canceled() {
			handle.Close()
			job.complete(context.Canceled)
			return
		}
		if err := handle.Run(func(success bool) {
			handle.Close()
			if success {
				job.complete(nil)
				return
			}
			// NSPrintOperation reports false for either user cancellation or a
			// native/spool failure. AppKit does not expose a separate terminal
			// error for this API; false is therefore the native cancellation
			// boundary required by the backend-neutral contract.
			job.complete(context.Canceled)
		}); err != nil {
			handle.Close()
			job.complete(err)
		}
	}
	if err := darwin.PerformOnMain(run, false); err != nil {
		handle.Close()
		return nil, fmt.Errorf("%w: %w", ErrPrintUnavailable, err)
	}
	watchPrintContext(ctx, job)
	return job, nil
}

func toDarwinPrintRanges(ranges []PrintPageRange) []darwin.PrintPageRange {
	if len(ranges) == 0 {
		return nil
	}
	out := make([]darwin.PrintPageRange, len(ranges))
	for i, r := range ranges {
		out[i] = darwin.PrintPageRange{From: r.From, To: r.To}
	}
	return out
}

func (p *darwinPlatform) printParent(parent WindowID) (darwin.ID, error) {
	if p == nil {
		return 0, ErrPrintUnavailable
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.app == nil || !p.app.IsInitialized() {
		return 0, ErrPrintUnavailable
	}
	if parent == 0 {
		if p.primary == nil || p.primary.window == nil {
			// nil docWindow requests an application-modal print panel, which is
			// valid when a caller submits a document before creating a window.
			return 0, nil //nolint:nilnil // zero parent intentionally requests an application-modal panel.
		}
		return p.primary.window.NSID(), nil
	}
	for _, w := range p.windows {
		if w != nil && w.id == parent && w.window != nil {
			return w.window.NSID(), nil
		}
	}
	return 0, fmt.Errorf("%w: %d", ErrPrintParentUnavailable, parent)
}
