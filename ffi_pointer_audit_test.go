package gogpu_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ffiPointerAllowed lists address-to-uintptr conversions that are safe without a
// runtime.Pinner, keyed by "file:function:expression", with the reason.
var ffiPointerAllowed = map[string]string{}

// TestFFIPointersArePinned guards P1-FFI-001 (the class wgpu found as P1-VK-012).
// A uintptr holding the address of a Go object is invisible to the GC and to the
// stack copier. Only a conversion written directly in the argument list of
// syscall.Syscall*, syscall.SyscallN or a Proc.Call / LazyProc.Call of the
// standard library's syscall package or of golang.org/x/sys/windows keeps the
// object alive and in place (unsafe.Pointer rule 4; both Call methods carry
// //go:uintptrescapes). That holds only for a direct call on the concrete type:
// the same Call reached through an interface or a func value loses the
// directive. goffi wrappers, objc msgSend, Wayland marshal helpers and struct
// fields read by C are ordinary Go code. Any other conversion of an address must
// pin the object with runtime.Pinner in the same function, point into a
// package-level variable, or be listed in ffiPointerAllowed with a reason.
//
// The check is syntactic. It recognizes the address-of form (&x, &x[i], &x.f);
// a pointer variable converted to uintptr is not checked, except when the
// uintptr is stored in a struct field (composite literal or field assignment):
// C reads such a field outside any argument list, so no Call keeps the referent
// alive. A field pointing into a package-level variable passes. A Call receiver counts
// as a proc only when the same package declares it with type *syscall.Proc,
// *syscall.LazyProc, *windows.Proc or *windows.LazyProc, or initializes it with
// NewProc, FindProc or MustFindProc on a DLL from NewLazyDLL, NewLazySystemDLL,
// LoadDLL or MustLoadDLL of either package.
func TestFFIPointersArePinned(t *testing.T) {
	var violations []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if path != "." {
			name := d.Name()
			// Private and scratch dirs, test data and nested modules are not
			// part of this module's code.
			if strings.HasPrefix(name, ".") || name == "tmp" || name == "testdata" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		found, err := unpinnedFFIAddressesInDir(path)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Errorf("Go address passed as uintptr without runtime.Pinner (see P1-FFI-001): %s", v)
	}
}

// unpinnedFFIAddressesInDir checks the non-test Go files of one package
// directory, whatever their build tags.
func unpinnedFFIAddressesInDir(dir string) ([]string, error) {
	fset, files, paths, err := parsePackageDir(dir)
	if err != nil {
		return nil, err
	}
	procs := syscallProcNames(fset, files)
	globals := packageVars(files)
	var found []string
	for i, f := range files {
		found = append(found, unpinnedFFIAddresses(fset, f, paths[i], procs, globals)...)
	}
	return found, nil
}

// parsePackageDir parses the non-test Go files of one directory.
func parsePackageDir(dir string) (*token.FileSet, []*ast.File, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	var paths []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		files = append(files, f)
		paths = append(paths, filepath.ToSlash(path))
	}
	return fset, files, paths, nil
}

// syscallProcNames returns the names of variables and struct fields in files
// that hold a *Proc or *LazyProc of the syscall or golang.org/x/sys/windows
// package, whose Call methods carry //go:uintptrescapes.
func syscallProcNames(fset *token.FileSet, files []*ast.File) map[string]bool {
	isType := func(e ast.Expr, names ...string) bool {
		s := exprString(fset, e)
		for _, n := range names {
			if s == n {
				return true
			}
		}
		return false
	}
	dlls := map[string]bool{}
	procs := map[string]bool{}
	// Typed declarations: var x *syscall.Proc, struct fields, var dll *syscall.DLL.
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			var names []*ast.Ident
			var typ ast.Expr
			switch v := n.(type) {
			case *ast.ValueSpec:
				names, typ = v.Names, v.Type
			case *ast.Field:
				names, typ = v.Names, v.Type
			default:
				return true
			}
			if typ == nil {
				return true
			}
			for _, id := range names {
				switch {
				case isType(typ, "*syscall.Proc", "*syscall.LazyProc", "*windows.Proc", "*windows.LazyProc"):
					procs[id.Name] = true
				case isType(typ, "*syscall.DLL", "*syscall.LazyDLL", "*windows.DLL", "*windows.LazyDLL"):
					dlls[id.Name] = true
				}
			}
			return true
		})
	}
	// Initialized declarations and assignments: dll = syscall.NewLazyDLL(...),
	// dll = windows.NewLazySystemDLL(...), proc = dll.NewProc(...),
	// proc := dll.NewProc(...).
	record := func(name string, value ast.Expr) {
		call, ok := value.(*ast.CallExpr)
		if !ok {
			return
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok {
			return
		}
		loader := sel.Sel.Name == "NewLazyDLL" || sel.Sel.Name == "NewLazySystemDLL" ||
			sel.Sel.Name == "LoadDLL" || sel.Sel.Name == "MustLoadDLL"
		switch {
		case (x.Name == "syscall" || x.Name == "windows") && loader:
			dlls[name] = true
		case dlls[x.Name] && (sel.Sel.Name == "NewProc" || sel.Sel.Name == "FindProc" || sel.Sel.Name == "MustFindProc"):
			procs[name] = true
		}
	}
	// Two passes: a proc may be declared before the DLL it comes from.
	for pass := 0; pass < 2; pass++ {
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.ValueSpec:
					for i := range v.Values {
						if i < len(v.Names) {
							record(v.Names[i].Name, v.Values[i])
						}
					}
				case *ast.AssignStmt:
					// Multi-value forms (proc, err = dll.FindProc(...)) bind the
					// first result to the proc.
					if len(v.Rhs) == 1 && len(v.Lhs) >= 1 {
						if id, ok := v.Lhs[0].(*ast.Ident); ok {
							record(id.Name, v.Rhs[0])
						}
					}
				}
				return true
			})
		}
	}
	return procs
}

// packageVars returns the package-level variable declarations of files, keyed
// by name. The parser resolves an identifier to its declaration only within one
// file, so a package-level variable from another file arrives unresolved.
func packageVars(files []*ast.File) map[string]*ast.ValueSpec {
	vars := map[string]*ast.ValueSpec{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for _, id := range vs.Names {
					vars[id.Name] = vs
				}
			}
		}
	}
	return vars
}

// addressOfPackageVar reports whether addr (&x, &x[i], &x.f, ...) points into
// a package-level variable. Its storage is static, or heap memory reachable
// through the variable: the stack copier never moves it and the GC keeps it,
// so the address is safe to hand to C as a uintptr (interface descriptors and
// listener tables that libwayland keeps for the life of a proxy, too).
func addressOfPackageVar(addr ast.Expr, globals map[string]*ast.ValueSpec) bool {
	e := addr.(*ast.UnaryExpr).X
	for {
		switch v := e.(type) {
		case *ast.IndexExpr:
			e = v.X
		case *ast.SelectorExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		case *ast.Ident:
			if v.Obj != nil {
				// Resolved in this file: a package-level var only if the
				// declaration is the top-level one, not a local of the same name.
				spec, ok := v.Obj.Decl.(*ast.ValueSpec)
				return ok && globals[v.Name] == spec
			}
			return globals[v.Name] != nil
		default:
			return false
		}
	}
}

func unpinnedFFIAddresses(fset *token.FileSet, file *ast.File, rel string, procs map[string]bool, globals map[string]*ast.ValueSpec) []string {
	var found []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		pinned := pinnedExpressions(fset, fn.Body)
		var stack []ast.Node
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if operand, ok := pointerToUintptr(n); ok && storedInStructField(stack) {
				if addr, ok := operand.(*ast.UnaryExpr); ok && addr.Op == token.AND && addressOfPackageVar(addr, globals) {
					return true
				}
				text := exprString(fset, operand)
				key := rel + ":" + fn.Name.Name + ":field " + text
				if _, ok := ffiPointerAllowed[key]; !ok && !pinned[text] {
					found = append(found, fset.Position(n.Pos()).String()+" "+key)
				}
				return true
			}
			addr, ok := addressToUintptr(n)
			if !ok {
				return true
			}
			if call, ok := stack[len(stack)-2].(*ast.CallExpr); ok && keepsUintptrReferent(call, procs) {
				return true
			}
			if addressOfPackageVar(addr, globals) {
				return true
			}
			text := exprString(fset, addr)
			if pinned[text] {
				return true
			}
			key := rel + ":" + fn.Name.Name + ":" + text
			if _, ok := ffiPointerAllowed[key]; ok {
				return true
			}
			found = append(found, fset.Position(n.Pos()).String()+" "+key)
			return true
		})
	}
	return found
}

// pointerToUintptr matches uintptr(unsafe.Pointer(x)) and returns x.
func pointerToUintptr(n ast.Node) (ast.Expr, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "uintptr" {
		return nil, false
	}
	inner, ok := call.Args[0].(*ast.CallExpr)
	if !ok || len(inner.Args) != 1 {
		return nil, false
	}
	sel, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Pointer" {
		return nil, false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "unsafe" {
		return nil, false
	}
	return inner.Args[0], true
}

// addressToUintptr matches uintptr(unsafe.Pointer(&x...)) and returns &x....
func addressToUintptr(n ast.Node) (ast.Expr, bool) {
	operand, ok := pointerToUintptr(n)
	if !ok {
		return nil, false
	}
	addr, ok := operand.(*ast.UnaryExpr)
	if !ok || addr.Op != token.AND {
		return nil, false
	}
	return addr, true
}

// storedInStructField reports whether the innermost node of stack is the value
// of a keyed composite-literal element (T{f: v}) or the right-hand side of an
// assignment to a selector (s.f = v).
func storedInStructField(stack []ast.Node) bool {
	if len(stack) < 3 {
		return false
	}
	n := stack[len(stack)-1]
	switch parent := stack[len(stack)-2].(type) {
	case *ast.KeyValueExpr:
		_, inLiteral := stack[len(stack)-3].(*ast.CompositeLit)
		return inLiteral && parent.Value == n
	case *ast.AssignStmt:
		for i, rhs := range parent.Rhs {
			if rhs == n && i < len(parent.Lhs) {
				_, isField := parent.Lhs[i].(*ast.SelectorExpr)
				return isField
			}
		}
	}
	return false
}

// keepsUintptrReferent reports whether call is one whose uintptr arguments the
// compiler keeps alive and unmoved: syscall.Syscall*, or Call on a receiver that
// procs records as a standard library *syscall.Proc / *syscall.LazyProc.
func keepsUintptrReferent(call *ast.CallExpr, procs map[string]bool) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "syscall" && strings.HasPrefix(sel.Sel.Name, "Syscall") {
		return true
	}
	if sel.Sel.Name != "Call" {
		return false
	}
	switch recv := sel.X.(type) {
	case *ast.Ident:
		return procs[recv.Name]
	case *ast.SelectorExpr:
		return procs[recv.Sel.Name]
	}
	return false
}

// pinnedExpressions collects the arguments of every X.Pin(arg) call in body.
func pinnedExpressions(fset *token.FileSet, body *ast.BlockStmt) map[string]bool {
	pinned := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Pin" {
			pinned[exprString(fset, call.Args[0])] = true
		}
		return true
	})
	return pinned
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var b strings.Builder
	_ = printer.Fprint(&b, fset, e)
	return b.String()
}

// TestFFIPointerGuardRejectsIndirectCall checks the guard itself: a direct Call
// on a syscall or golang.org/x/sys/windows proc passes, the same Call through an
// interface or a Go wrapper does not; through a wrapper, an address in a
// package-level variable passes and a local that shadows it does not; a pointer
// stored as uintptr in a struct field fails unless it is pinned or points into a
// package-level variable.
func TestFFIPointerGuardRejectsIndirectCall(t *testing.T) {
	const src = `package p

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type caller interface{ Call(a ...uintptr) (uintptr, uintptr, error) }

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	procOK   = user32.NewProc("GetDC")
	xsys     = windows.NewLazySystemDLL("user32.dll")
	procXSys = xsys.NewProc("GetDC")
)

func direct(x int) { procOK.Call(uintptr(unsafe.Pointer(&x))) }

func viaInterface(c caller, x int) { c.Call(uintptr(unsafe.Pointer(&x))) }

func viaXSys(x int) { procXSys.Call(uintptr(unsafe.Pointer(&x))) }

func send(a uintptr) { procXSys.Call(a) }

func viaWrapper(x int) { send(uintptr(unsafe.Pointer(&x))) }

var table [4]uintptr

func global() { send(uintptr(unsafe.Pointer(&table[0]))) }

func shadow() {
	var table [4]uintptr
	send(uintptr(unsafe.Pointer(&table[0])))
}

type item struct{ data uintptr }

func fieldLiteral(s *uint16) {
	procXSys.Call(uintptr(unsafe.Pointer(&item{data: uintptr(unsafe.Pointer(s))})))
}

func fieldAssign(s *uint16) {
	var it item
	it.data = uintptr(unsafe.Pointer(s))
	procXSys.Call(uintptr(unsafe.Pointer(&it)))
}

func fieldPinned(s *uint16) {
	var p runtime.Pinner
	p.Pin(s)
	defer p.Unpin()
	procXSys.Call(uintptr(unsafe.Pointer(&item{data: uintptr(unsafe.Pointer(s))})))
}

func fieldGlobal() {
	procXSys.Call(uintptr(unsafe.Pointer(&item{data: uintptr(unsafe.Pointer(&table[0]))})))
}
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := unpinnedFFIAddressesInDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	funcs := make([]string, 0, len(found))
	for _, f := range found {
		parts := strings.Split(f, ":")
		funcs = append(funcs, parts[len(parts)-2])
	}
	sort.Strings(funcs)
	if got, want := strings.Join(funcs, ","), "fieldAssign,fieldLiteral,shadow,viaInterface,viaWrapper"; got != want {
		t.Fatalf("flagged functions = %q, want %q (direct proc Calls, package-level vars and pinned fields must pass)", got, want)
	}
}
