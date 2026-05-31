//go:build linux

// Linux native file dialog implementation.
//
// Selection order:
//  1. xdg-desktop-portal D-Bus client (org.freedesktop.portal.FileChooser) — works on
//     both X11 and Wayland sessions with any compositor that ships the portal.
//  2. zenity subprocess — GNOME/GTK desktops without the portal daemon.
//  3. kdialog subprocess — KDE desktops without the portal daemon.
//
// If none of the three is reachable the functions return a descriptive error.
// User cancellation is represented as (nil, nil) / ("", nil) — not an error.
package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// showOpenFileDialog tries xdg-desktop-portal first, then zenity, then kdialog.
// Returns nil, nil if the user cancels without selecting a file.
func showOpenFileDialog(opts FileDialogOptions) ([]string, error) {
	if hasDBusSession() {
		paths, err := portalOpenFile("", opts)
		if err == nil {
			return paths, nil
		}
		// Only fall back to subprocess when the portal is unreachable (connect/send
		// failed before the dialog was shown). A mid-dialog error (socket closed
		// after METHOD_RETURN) is surfaced directly — falling back would open a
		// second dialog on top of the still-visible portal chooser.
		if !errors.Is(err, errPortalUnavailable) {
			return nil, err
		}
	}
	return subprocOpenFile(opts)
}

// showSaveFileDialog tries xdg-desktop-portal first, then zenity, then kdialog.
// Returns "", nil if the user cancels without confirming a filename.
func showSaveFileDialog(opts FileDialogOptions) (string, error) {
	if hasDBusSession() {
		path, err := portalSaveFile("", opts)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, errPortalUnavailable) {
			return "", err
		}
	}
	return subprocSaveFile(opts)
}

// hasDBusSession reports whether DBUS_SESSION_BUS_ADDRESS is set, which is a
// prerequisite for the xdg-desktop-portal path.
func hasDBusSession() bool {
	return os.Getenv("DBUS_SESSION_BUS_ADDRESS") != ""
}

// Argument constants shared across zenity/kdialog builders.
const (
	zenityFileSelection = "--file-selection"
	zenitySaveFlag      = "--save"
	zenityConfirmOW     = "--confirm-overwrite"
	kdialogGetSaveFile  = "--getsavefilename"
)

// subprocOpenFile probes PATH for zenity then kdialog and runs the first found.
// Returns a descriptive error when neither tool is available.
func subprocOpenFile(opts FileDialogOptions) ([]string, error) {
	if _, err := exec.LookPath("zenity"); err == nil {
		return zenityOpen(opts)
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		return kdialogOpen(opts)
	}
	return nil, fmt.Errorf("file dialog: neither zenity nor kdialog found — install zenity or wait for xdg-portal support")
}

// subprocSaveFile probes PATH for zenity then kdialog and runs the first found.
func subprocSaveFile(opts FileDialogOptions) (string, error) {
	if _, err := exec.LookPath("zenity"); err == nil {
		return zenitySave(opts)
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		return kdialogSave(opts)
	}
	return "", fmt.Errorf("file dialog: neither zenity nor kdialog found — install zenity or wait for xdg-portal support")
}

// --- zenity ---

// zenityOpen runs "zenity --file-selection [...]" and returns the selected paths.
// Exit code 1 from zenity means the user canceled; that returns nil, nil.
func zenityOpen(opts FileDialogOptions) ([]string, error) {
	out, err := exec.Command("zenity", zenityOpenArgs(opts)...).Output()
	if err != nil {
		if isExitCode(err, 1) {
			return nil, nil
		}
		return nil, fmt.Errorf("file dialog: zenity: %w", err)
	}
	return splitNewlines(strings.TrimRight(string(out), "\n")), nil
}

// zenitySave runs "zenity --file-selection --save [...]" and returns the chosen path.
// Exit code 1 means the user canceled; that returns "", nil.
func zenitySave(opts FileDialogOptions) (string, error) {
	out, err := exec.Command("zenity", zenitySaveArgs(opts)...).Output()
	if err != nil {
		if isExitCode(err, 1) {
			return "", nil
		}
		return "", fmt.Errorf("file dialog: zenity: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// zenityOpenArgs builds the argument slice for a "zenity --file-selection" command.
func zenityOpenArgs(opts FileDialogOptions) []string {
	args := []string{zenityFileSelection}
	if opts.Directory {
		args = append(args, "--directory")
	}
	if opts.Multiple {
		args = append(args, "--multiple", "--separator=\n")
	}
	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	if opts.InitialDirectory != "" {
		args = append(args, "--filename="+opts.InitialDirectory+"/")
	}
	// Filters are irrelevant (and can cause exit errors on older zenity) in directory mode.
	if !opts.Directory {
		for _, f := range opts.Filters {
			if spec := zenityFilterSpec(f); spec != "" {
				args = append(args, "--file-filter="+spec)
			}
		}
	}
	return args
}

// zenitySaveArgs builds the argument slice for a "zenity --file-selection --save" command.
func zenitySaveArgs(opts FileDialogOptions) []string {
	args := []string{zenityFileSelection, zenitySaveFlag, zenityConfirmOW}
	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	startPath := opts.InitialDirectory
	switch {
	case startPath != "" && opts.DefaultFilename != "":
		startPath += "/" + opts.DefaultFilename
	case opts.DefaultFilename != "":
		startPath = opts.DefaultFilename
	case startPath != "":
		startPath += "/" // trailing slash = directory hint (no pre-filled filename)
	}
	if startPath != "" {
		args = append(args, "--filename="+startPath)
	}
	for _, f := range opts.Filters {
		if spec := zenityFilterSpec(f); spec != "" {
			args = append(args, "--file-filter="+spec)
		}
	}
	return args
}

// zenityFilterSpec formats a single FileTypeFilter as "Name | *.ext1 *.ext2".
// Returns an empty string when all extensions are blank (caller must skip it).
func zenityFilterSpec(f FileTypeFilter) string {
	parts := make([]string, 0, len(f.Extensions))
	for _, e := range f.Extensions {
		e = strings.TrimPrefix(strings.TrimPrefix(e, "*"), ".")
		if e != "" {
			parts = append(parts, "*."+e)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return f.Name + " | " + strings.Join(parts, " ")
}

// --- kdialog ---

// kdialogOpen runs the appropriate kdialog variant and returns the selected paths.
// Exit code 1 means the user canceled; that returns nil, nil.
func kdialogOpen(opts FileDialogOptions) ([]string, error) {
	out, err := exec.Command("kdialog", kdialogOpenArgs(opts)...).Output()
	if err != nil {
		if isExitCode(err, 1) {
			return nil, nil
		}
		return nil, fmt.Errorf("file dialog: kdialog: %w", err)
	}
	return kdialogParsePaths(strings.TrimRight(string(out), "\n"), opts.Multiple), nil
}

// kdialogSave runs "kdialog --getsavefilename" and returns the chosen path.
// Exit code 1 means the user canceled; that returns "", nil.
func kdialogSave(opts FileDialogOptions) (string, error) {
	out, err := exec.Command("kdialog", kdialogSaveArgs(opts)...).Output()
	if err != nil {
		if isExitCode(err, 1) {
			return "", nil
		}
		return "", fmt.Errorf("file dialog: kdialog: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// kdialogOpenArgs builds the argument slice for a kdialog open-file command.
// Chooses --getopenfilename, --getopenfilenames, or --getexistingdirectory
// depending on opts.Multiple and opts.Directory.
func kdialogOpenArgs(opts FileDialogOptions) []string {
	var args []string
	switch {
	case opts.Directory:
		args = append(args, "--getexistingdirectory")
	case opts.Multiple:
		args = append(args, "--getopenfilenames")
	default:
		args = append(args, "--getopenfilename")
	}

	startDir := opts.InitialDirectory
	if startDir == "" {
		startDir = "."
	}
	args = append(args, startDir)

	if !opts.Directory {
		if spec := kdialogFilterSpec(opts.Filters); spec != "" {
			args = append(args, spec)
		}
	}
	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	return args
}

// kdialogSaveArgs builds the argument slice for "kdialog --getsavefilename".
func kdialogSaveArgs(opts FileDialogOptions) []string {
	args := []string{kdialogGetSaveFile}

	startDir := opts.InitialDirectory
	if startDir == "" {
		startDir = "."
	}
	if opts.DefaultFilename != "" {
		startDir += "/" + opts.DefaultFilename
	}
	args = append(args, startDir)

	if spec := kdialogFilterSpec(opts.Filters); spec != "" {
		args = append(args, spec)
	}
	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	return args
}

// kdialogFilterSpec formats a filter list as "Images (*.png *.jpg);;Docs (*.pdf)".
// Returns an empty string when no valid extensions are present.
func kdialogFilterSpec(filters []FileTypeFilter) string {
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		exts := make([]string, 0, len(f.Extensions))
		for _, e := range f.Extensions {
			e = strings.TrimPrefix(strings.TrimPrefix(e, "*"), ".")
			if e != "" {
				exts = append(exts, "*."+e)
			}
		}
		if len(exts) > 0 {
			parts = append(parts, f.Name+" ("+strings.Join(exts, " ")+")")
		}
	}
	return strings.Join(parts, ";;")
}

// kdialogParsePaths parses kdialog output into a path slice.
// Single-file mode: the entire trimmed line is one path.
// Multiple-file mode (--getopenfilenames): paths are space-separated.
// Note: space separation is ambiguous for paths that contain spaces — this is
// a known kdialog limitation with no clean workaround.
func kdialogParsePaths(raw string, multiple bool) []string {
	if raw == "" {
		return nil
	}
	if !multiple {
		return []string{raw}
	}
	fields := strings.Fields(raw)
	result := make([]string, 0, len(fields))
	for _, p := range fields {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// splitNewlines splits a newline-delimited string into non-empty lines.
// Used to parse zenity multiple-selection output (--separator=\n).
func splitNewlines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// isExitCode reports whether err is an *exec.ExitError with the given exit code.
func isExitCode(err error, code int) bool {
	var e *exec.ExitError
	return errors.As(err, &e) && e.ExitCode() == code
}

// --- portal API ---

// portalOpenFile calls org.freedesktop.portal.FileChooser.OpenFile and blocks until
// the user responds.  parentWindow is "wayland:<handle>", "x11:<xid>", or "" for
// an unparented dialog (acceptable per xdg-portal spec).
// Returns nil, nil if the user cancels.
// Returns errPortalUnavailable if the portal cannot be reached before the dialog is shown.
func portalOpenFile(parentWindow string, opts FileDialogOptions) ([]string, error) {
	conn, err := dbusConnect()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPortalUnavailable, err)
	}
	defer conn.rw.Close()

	token := dbusNewToken()
	handlePath := dbusHandlePath(conn.name, token)

	body := encodeFileChooserBody(parentWindow, opts.Title, opts, token, false)
	callSerial, err := conn.sendCall(
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.FileChooser",
		"OpenFile",
		"ssa{sv}",
		body,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: send OpenFile: %w", errPortalUnavailable, err)
	}
	return conn.waitResponse(callSerial, handlePath)
}

// portalSaveFile calls org.freedesktop.portal.FileChooser.SaveFile and blocks until
// the user responds.  Returns "", nil if the user cancels.
// Returns errPortalUnavailable if the portal cannot be reached before the dialog is shown.
func portalSaveFile(parentWindow string, opts FileDialogOptions) (string, error) {
	conn, err := dbusConnect()
	if err != nil {
		return "", fmt.Errorf("%w: %w", errPortalUnavailable, err)
	}
	defer conn.rw.Close()

	token := dbusNewToken()
	handlePath := dbusHandlePath(conn.name, token)

	body := encodeFileChooserBody(parentWindow, opts.Title, opts, token, true)
	callSerial, err := conn.sendCall(
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.FileChooser",
		"SaveFile",
		"ssa{sv}",
		body,
	)
	if err != nil {
		return "", fmt.Errorf("%w: send SaveFile: %w", errPortalUnavailable, err)
	}

	paths, err := conn.waitResponse(callSerial, handlePath)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// encodeFileChooserBody encodes the ssa{sv} body for both OpenFile and SaveFile
// portal method calls.  isSave controls inclusion of save-specific options
// (current_name) and exclusion of open-specific options (multiple).
func encodeFileChooserBody(parentWindow, title string, opts FileDialogOptions, token string, isSave bool) []byte {
	b := newMsgBuf(0) // body starts at an 8-byte-aligned boundary → base 0 is correct
	b.str(parentWindow)
	b.str(title)

	lp, cp := b.arrayStart(8) // a{sv}: dict entries {sv} are struct-aligned (8)

	b.padTo(8)
	b.str("handle_token")
	b.variantStr(token)

	if opts.Multiple && !isSave {
		b.padTo(8)
		b.str("multiple")
		b.variantBool(true)
	}

	if opts.Directory {
		b.padTo(8)
		b.str("directory")
		b.variantBool(true)
	}

	if len(opts.Filters) > 0 && !opts.Directory {
		b.padTo(8)
		b.str("filters")
		b.sig("a(sa(us))")
		encodePortalFilters(b, opts.Filters)
	}

	if opts.InitialDirectory != "" {
		b.padTo(8)
		b.str("current_folder")
		// ay: NUL-terminated absolute path per xdg-portal FileChooser spec.
		b.variantByteArray(append([]byte(opts.InitialDirectory), 0))
	}

	if isSave && opts.DefaultFilename != "" {
		b.padTo(8)
		b.str("current_name")
		b.variantStr(opts.DefaultFilename)
	}

	b.arrayEnd(lp, cp)
	return b.data
}

// encodePortalFilters encodes a(sa(us)) for the portal "filters" option.
// Each FileTypeFilter becomes a (name, patterns) pair where every pattern is
// (0=glob, "*.ext").
func encodePortalFilters(b *msgBuf, filters []FileTypeFilter) {
	lp, cp := b.arrayStart(8) // outer array: (sa(us)) structs are 8-byte aligned

	for _, f := range filters {
		b.padTo(8) // (sa(us)) struct alignment
		b.str(f.Name)

		ilp, icp := b.arrayStart(8) // inner a(us): (us) structs are 8-byte aligned
		for _, ext := range f.Extensions {
			ext = strings.TrimPrefix(strings.TrimPrefix(ext, "*"), ".")
			if ext == "" {
				continue
			}
			b.padTo(8) // (us) struct alignment
			b.u32(0)   // pattern type 0 = glob
			b.str("*." + ext)
		}
		b.arrayEnd(ilp, icp)
	}

	b.arrayEnd(lp, cp)
}
