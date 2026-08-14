# Native printing contract

`App.Print` is the backend-neutral seam for native print dialogs and spoolers.
It accepts a complete document; gogpu does not render pages or generate PDF
bytes. `NewPDFDocument` is the portable helper for the common PDF case.
Platform implementations own format-specific rendering/spooling (for example,
Windows may render PDF before submitting a printer job).

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
- Validation, unsupported platforms, and setup failures are returned by
  `App.Print` and do not create a job. Native dialog/spool failures are
  reported through `Done`.

The platform capability is the optional `internal/platform.PrintManager`
interface. Existing backends intentionally do not implement it yet; native
Windows, macOS, and Linux portal work can be added independently without
changing this contract. Browser remains unsupported until it has a matching
print API.
