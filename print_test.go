package gogpu

import (
	"context"
	"errors"
	"testing"

	"github.com/gogpu/gogpu/internal/platform"
)

type printTestJob struct {
	done     chan error
	canceled bool
}

func newPrintTestJob() *printTestJob {
	return &printTestJob{done: make(chan error, 1)}
}

func (j *printTestJob) Done() <-chan error { return j.done }
func (j *printTestJob) Cancel() {
	if !j.canceled {
		j.canceled = true
		j.done <- context.Canceled
		close(j.done)
	}
}

type printTestManager struct {
	mockManager
	request platform.PrintRequest
	ctx     context.Context
	job     *printTestJob
	err     error
}

func (m *printTestManager) StartPrint(ctx context.Context, request platform.PrintRequest) (platform.PrintJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.ctx = ctx
	m.request = request
	m.job = newPrintTestJob()
	return m.job, nil
}

func TestNewPDFDocumentOwnsInput(t *testing.T) {
	pdf := []byte("%PDF-test")
	doc := NewPDFDocument("report.pdf", pdf)
	pdf[0] = 'x'
	if doc.MIMEType != PrintMIMETypePDF {
		t.Fatalf("MIMEType = %q, want %q", doc.MIMEType, PrintMIMETypePDF)
	}
	if string(doc.Data) != "%PDF-test" {
		t.Fatalf("Data = %q, want original PDF bytes", doc.Data)
	}
}

func TestPrintRejectsInvalidRequests(t *testing.T) {
	app := NewApp(DefaultConfig())
	if _, err := app.Print(context.Background(), PrintDocument{}, PrintOptions{}); !errors.Is(err, ErrInvalidPrintDocument) {
		t.Fatalf("invalid document error = %v, want ErrInvalidPrintDocument", err)
	}
	if _, err := app.Print(nil, NewPDFDocument("x.pdf", []byte("pdf")), PrintOptions{}); !errors.Is(err, ErrNilPrintContext) {
		t.Fatalf("nil context error = %v, want ErrNilPrintContext", err)
	}
	if _, err := app.Print(context.Background(), NewPDFDocument("x.pdf", []byte("pdf")), PrintOptions{Copies: -1}); !errors.Is(err, ErrInvalidPrintOptions) {
		t.Fatalf("invalid options error = %v, want ErrInvalidPrintOptions", err)
	}
	if _, err := app.Print(context.Background(), NewPDFDocument("x.pdf", []byte("pdf")), PrintOptions{PageRanges: []PrintPageRange{{From: 0, To: 1}}}); !errors.Is(err, ErrInvalidPrintOptions) {
		t.Fatalf("invalid page range error = %v, want ErrInvalidPrintOptions", err)
	}
}

func TestPrintUnsupportedWithoutPlatformCapability(t *testing.T) {
	doc := NewPDFDocument("x.pdf", []byte("pdf"))
	if _, err := NewApp(DefaultConfig()).Print(context.Background(), doc, PrintOptions{}); !errors.Is(err, ErrPrintUnsupported) {
		t.Fatalf("nil manager error = %v, want ErrPrintUnsupported", err)
	}
	if _, err := (&App{manager: &mockManager{}}).Print(context.Background(), doc, PrintOptions{}); !errors.Is(err, ErrPrintUnsupported) {
		t.Fatalf("manager without PrintManager error = %v, want ErrPrintUnsupported", err)
	}
}

func TestPrintDelegatesCopiedRequestAndPrimaryParent(t *testing.T) {
	mgr := &printTestManager{}
	app := &App{
		manager:    mgr,
		platWindow: &mockWindow{windowID: 42},
	}
	pdf := []byte("pdf bytes")
	ranges := []PrintPageRange{{From: 1, To: 2}}
	ctx := context.WithValue(context.Background(), struct{}{}, "request")
	job, err := app.Print(ctx, PrintDocument{Name: "x.pdf", MIMEType: PrintMIMETypePDF, Data: pdf}, PrintOptions{
		Title:      "Print x",
		PageRanges: ranges,
		Copies:     2,
	})
	if err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	pdf[0] = 'X'
	ranges[0].From = 99
	if mgr.ctx != ctx {
		t.Fatal("Print() did not forward context")
	}
	if mgr.request.Options.Parent != platform.WindowID(42) {
		t.Fatalf("Parent = %d, want 42", mgr.request.Options.Parent)
	}
	if string(mgr.request.Document.Data) != "pdf bytes" {
		t.Fatalf("request Data = %q, want copied bytes", mgr.request.Document.Data)
	}
	if mgr.request.Options.PageRanges[0].From != 1 {
		t.Fatalf("request PageRanges was not copied: %+v", mgr.request.Options.PageRanges)
	}
	if mgr.request.Options.Title != "Print x" || mgr.request.Options.Copies != 2 {
		t.Fatalf("request options = %+v, want title/copies preserved", mgr.request.Options)
	}

	select {
	case got := <-job.Done():
		t.Fatalf("initial Done value = %v, want no value until backend completes", got)
	default:
	}
	job.Cancel()
	if got := <-job.Done(); !errors.Is(got, context.Canceled) {
		t.Fatalf("canceled Done value = %v, want context.Canceled", got)
	}
}

func TestPrintCanceledContextDoesNotStartJob(t *testing.T) {
	mgr := &printTestManager{}
	app := &App{manager: mgr}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app.Print(ctx, NewPDFDocument("x.pdf", []byte("pdf")), PrintOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled Print() error = %v, want context.Canceled", err)
	}
	if mgr.job != nil {
		t.Fatal("pre-canceled request should not start a print job")
	}
}

func TestPrintJobCancellationIsTerminalAndIdempotent(t *testing.T) {
	job := newPrintTestJob()
	job.Cancel()
	job.Cancel()
	if !job.canceled {
		t.Fatal("Cancel did not mark job canceled")
	}
	if got := <-job.Done(); !errors.Is(got, context.Canceled) {
		t.Fatalf("Done() = %v, want context.Canceled", got)
	}
	if _, open := <-job.Done(); open {
		t.Fatal("Done channel should close after terminal cancellation")
	}
}
