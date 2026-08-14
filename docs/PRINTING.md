# Native printing contract

`App.Print` is the backend-neutral seam for native print dialogs and spoolers.
It accepts a complete document; gogpu does not render pages or generate PDF
bytes. `NewPDFDocument` is the portable helper for the common PDF case.
Windows sends raw PDF bytes through the GDI `PASSTHROUGH` escape and does not
render them. Only compatible PostScript/PCL or virtual PDF printer drivers that
accept the raw document format can print it; typical consumer inkjet drivers
may fail.

```go
document := gogpu.NewPDFDocument("invoice.pdf", pdfBytes)
job, err := app.Print(ctx, document, gogpu.PrintOptions{
	Title:      "Print invoice",
	PageRanges: []gogpu.PrintPageRange{{From: 1, To: 2}},
	Copies:     1,
})
if err != nil {
	// Request was not accepted (invalid input or no native backend).
	return err
}
if err := <-job.Done(); err != nil {
	// context.Canceled means the user/caller canceled; another error is a
	// native dialog or spool failure.
	return err
}
```

## Ownership and lifecycle

- `PrintDocument` contains the final bytes and MIME type. `App.Print` copies
  bytes and page ranges before invoking the asynchronous backend; callers may
  reuse their input after `Print` returns.
- `PrintOptions.Parent` identifies the owning `WindowID`. Zero selects the
  primary window when available, otherwise the application parent. A native
  implementation must keep that relationship until the job reaches `Done`.
- The request is accepted synchronously, but the dialog/spool operation is
  asynchronous. `Done` receives exactly one terminal error and then closes.
- Passing a canceled context, or calling `PrintJob.Cancel`, requests
  cancellation. A canceled job reports `context.Canceled`; cancellation is
  idempotent and has no effect after completion.
- Validation and unavailable print services are returned by `App.Print` and do
  not create a job (`ErrPrintUnsupported`); native dialog/spool failures are
  reported through `Done`.

The platform capability is the optional `internal/platform.PrintManager`
interface. Windows uses the native PrintDlgEx/GDI path, macOS uses PDFKit and
AppKit, and Linux X11/Wayland use xdg-desktop-portal. Browser remains
unsupported until it has a matching print API.
