// Command apigolden emits a signature-level golden of this module's public API
// for one build context.
//
//	usage: go -C tools run ./apigolden -out apigolden/testdata/api_windows.txt
//	       go -C tools run ./apigolden -goos linux -out apigolden/testdata/api_linux.txt
//	       go -C tools run ./apigolden -goos darwin -goarch arm64 -out apigolden/testdata/api_darwin.txt
//	       go -C tools run ./apigolden -goos js -goarch wasm -out apigolden/testdata/api_wasm.txt
//
// The module root is found by walking up from the working directory to the
// go.mod whose module path is exactly github.com/gogpu/gogpu. Loading runs with
// GOWORK=off so the golden describes the module, not a developer workspace.
package main

import (
	"flag"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const modulePath = "github.com/gogpu/gogpu"

// Floors per build context. A context that is not listed is refused rather than
// given a weak default: a new context must be declared deliberately, with the
// numbers that make a partial load fail.
type floor struct {
	packages    int
	identifiers int
}

var contextFloors = map[string]floor{
	"windows/amd64/": {8, 328},
	"linux/amd64/":   {8, 328},
	"darwin/arm64/":  {8, 328},
	"js/wasm/":       {8, 328},
}

func floorFor(c config) (floor, error) {
	if c.minPackages > 0 || c.minIdentifiers > 0 {
		if c.minPackages <= 0 || c.minIdentifiers <= 0 {
			return floor{}, fmt.Errorf("both -min-packages and -min-identifiers are required to override the floors")
		}
		return floor{c.minPackages, c.minIdentifiers}, nil
	}
	key := c.goos + "/" + c.goarch + "/" + strings.Join(c.tags, ",")
	f, ok := contextFloors[key]
	if !ok {
		return floor{}, fmt.Errorf("no floor defined for context %q; add one in contextFloors or pass -min-packages/-min-identifiers", c.describe())
	}
	return f, nil
}

type config struct {
	root   string
	out    string
	goos   string
	goarch string
	tags   []string

	// Explicit floor overrides, for the first generation of a new context.
	minPackages    int
	minIdentifiers int
}

func main() {
	var c config
	var tags string
	flag.StringVar(&c.root, "root", "", "module root (default: nearest parent go.mod for github.com/gogpu/gogpu)")
	flag.StringVar(&c.out, "out", "", "output file (default: stdout)")
	flag.StringVar(&c.goos, "goos", "windows", "GOOS of the context")
	flag.StringVar(&c.goarch, "goarch", "amd64", "GOARCH of the context")
	flag.StringVar(&tags, "tags", "", "comma-separated build tags")
	flag.IntVar(&c.minPackages, "min-packages", 0, "override the context's minimum public-package count (both min flags required)")
	flag.IntVar(&c.minIdentifiers, "min-identifiers", 0, "override the context's minimum exported-identifier count")
	flag.Parse()

	if tags != "" {
		for _, t := range strings.Split(tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				c.tags = append(c.tags, t)
			}
		}
	}
	sort.Strings(c.tags)

	root := c.root
	if root == "" {
		var err error
		root, err = findModuleRoot()
		if err != nil {
			fatal("%v", err)
		}
	}

	pkgs, err := load(root, c)
	if err != nil {
		fatal("%v", err)
	}

	text, stats, err := render(c, pkgs)
	if err != nil {
		fatal("%v", err)
	}

	if c.out == "" {
		fmt.Print(text)
		return
	}
	if err := os.WriteFile(c.out, []byte(text), 0o644); err != nil {
		fatal("write %s: %v", c.out, err)
	}
	fmt.Fprintf(os.Stderr, "%s: packages=%d identifiers=%d\n", c.out, stats.packages, stats.identifiers)
}

type stats struct {
	packages    int
	identifiers int
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "apigolden: "+format+"\n", args...)
	os.Exit(1)
}

// findModuleRoot walks up from the working directory to the go.mod whose module
// path is exactly the one this tool describes. It deliberately skips the tools
// module (github.com/gogpu/gogpu/tools) that sits below it.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && modulePathOf(string(data)) == modulePath {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod with module %s above the working directory", modulePath)
		}
		dir = parent
	}
}

func modulePathOf(gomod string) string {
	for _, line := range strings.Split(gomod, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// contextEnv is os.Environ with the context forced and the workspace disabled.
// Duplicate keys are dropped so the child cannot see two GOOS values.
func contextEnv(c config) []string {
	drop := map[string]bool{
		"GOOS": true, "GOARCH": true, "GOFLAGS": true, "GOWORK": true, "CGO_ENABLED": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if drop[k] {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"GOOS="+c.goos,
		"GOARCH="+c.goarch,
		"CGO_ENABLED=0",
		"GOWORK=off",
		"GOFLAGS=-mod=readonly"+c.tagFlag(),
	)
	return env
}

func (c config) tagFlag() string {
	if len(c.tags) == 0 {
		return ""
	}
	return " -tags=" + strings.Join(c.tags, ",")
}

func (c config) describe() string {
	tags := "none"
	if len(c.tags) > 0 {
		tags = strings.Join(c.tags, ",")
	}
	return fmt.Sprintf("goos=%s goarch=%s tags=%s", c.goos, c.goarch, tags)
}

// publicPackages is the module's importable surface: internals, examples,
// tools and tmp are excluded by construction, so a new top-level package must
// be added here deliberately to enter the golden.
var publicPackages = map[string]bool{
	modulePath:                         true,
	modulePath + "/gmath":              true,
	modulePath + "/golden":             true,
	modulePath + "/gpu":                true,
	modulePath + "/gpu/backend/native": true,
	modulePath + "/gpu/types":          true,
	modulePath + "/input":              true,
	modulePath + "/sound":              true,
}

func include(path string) bool { return publicPackages[path] }

// render assembles the golden text: a deterministic header, then one block per
// package in path order.
func render(c config, pkgs []*types.Package) (string, stats, error) {
	var st stats
	blocks := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		block := FormatPackage(p)
		st.identifiers += countIdentifiers(p)
		blocks = append(blocks, block)
	}
	st.packages = len(pkgs)

	fl, err := floorFor(c)
	if err != nil {
		return "", st, err
	}
	if st.packages < fl.packages {
		return "", st, fmt.Errorf("floor: only %d public packages (min %d) for %s", st.packages, fl.packages, c.describe())
	}
	if st.identifiers < fl.identifiers {
		return "", st, fmt.Errorf("floor: only %d exported identifiers (min %d) for %s", st.identifiers, fl.identifiers, c.describe())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# apigolden — signature-level public API of %s\n", modulePath)
	fmt.Fprintf(&b, "# context: %s\n", c.describe())
	fmt.Fprintf(&b, "# packages: %d  identifiers: %d\n", st.packages, st.identifiers)
	b.WriteString("#\n# Generated by tools/apigolden. Do not edit; regenerate and review the delta.\n")
	for _, block := range blocks {
		b.WriteString("\n")
		b.WriteString(block)
	}
	return b.String(), st, nil
}

func countIdentifiers(p *types.Package) int {
	n := 0
	for _, name := range p.Scope().Names() {
		if obj := p.Scope().Lookup(name); obj != nil && obj.Exported() {
			n++
		}
	}
	return n
}
