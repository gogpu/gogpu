package gogpu

// AppLifecycle represents the application lifecycle state (ADR-026).
//
// Desktop apps transition: AppIdle → AppRunning → (exit).
// Mobile apps cycle through all states as the OS suspends/resumes.
// Web apps use Suspended/Resumed for page visibility.
//
// The lifecycle is universal — same API on all platforms. Desktop is the
// trivial case where Suspending/Suspended/Resuming never occur.
type AppLifecycle int

const (
	AppIdle       AppLifecycle = iota // App not started yet
	AppRunning                        // Normal execution, surfaces available
	AppSuspending                     // Grace period: drop GPU surfaces before suspend
	AppSuspended                      // Backgrounded, no rendering, surfaces may be gone
	AppResuming                       // Grace period: recreate surfaces after resume
)

// IsActive reports whether the app should process frames.
// True for Running, Suspending, and Resuming (grace periods allow one last/first frame).
func (s AppLifecycle) IsActive() bool {
	return s == AppRunning || s == AppSuspending || s == AppResuming
}

// String returns the lifecycle state name.
func (s AppLifecycle) String() string {
	switch s {
	case AppIdle:
		return "Idle"
	case AppRunning:
		return "Running"
	case AppSuspending:
		return "Suspending"
	case AppSuspended:
		return "Suspended"
	case AppResuming:
		return "Resuming"
	default:
		return "Unknown lifecycle"
	}
}
