package testconventions

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ignoreMarker silences one rule for the declaration or statement it sits on:
//
//	// conventions:ignore parallel: AllocsPerRun panics in a parallel test
//
// The comment must end on the line of the construct or on the line above it,
// and the reason after the colon is required.
const ignoreMarker = "conventions:ignore "

const (
	ruleParallel    = "parallel"
	ruleBlankLine   = "blank-line"
	ruleHint        = "run-hint"
	ruleBodyRead    = "body-read"
	ruleStorageFake = "storage-fake"
	ruleBenchLoop   = "bench-loop"
)

// finding is one violation of a rule.
type finding struct {
	rule  string
	scope string // the function the finding is in, for the rules that keep a baseline
	msg   string
	pos   token.Position
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d: [%s] %s", f.pos.Filename, f.pos.Line, f.rule, f.msg)
}

// source is a parsed test file with what the rules need besides the syntax tree.
type source struct {
	file     *ast.File
	fset     *token.FileSet
	path     string // slash-separated, relative to the module root
	lines    []string
	ignores  map[string]map[int]bool // rule -> lines on which it is silenced
	scope    string                  // set by the rules that report per function
	findings []finding
}

func (s *source) reportf(rule string, pos token.Pos, format string, args ...any) {
	p := s.fset.Position(pos)
	if s.ignores[rule][p.Line] || s.ignores[rule][p.Line-1] {
		return
	}
	p.Filename = s.path
	s.findings = append(s.findings, finding{rule: rule, scope: s.scope, pos: p, msg: fmt.Sprintf(format, args...)})
}

// moduleRoot returns the directory holding go.mod, searching upwards from the
// working directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "go.mod not found")
		dir = parent
	}
}

// testFiles parses every _test.go file of the module.
func testFiles(t *testing.T) []*source {
	t.Helper()
	root := moduleRoot(t)
	var files []*source
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "node_modules", "vendor", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		s, err := newSource(t, filepath.ToSlash(rel), src)
		if errors.Is(err, errGenerated) {
			return nil
		}
		if err != nil {
			return err
		}
		files = append(files, s)
		return nil
	})
	require.NoError(t, err)
	return files
}

// errGenerated is returned by newSource for generated files, which are not
// written by hand and are not checked.
var errGenerated = errors.New("generated file")

// newSource parses src as the file at the module-relative path.
func newSource(t *testing.T, path string, src []byte) (*source, error) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if ast.IsGenerated(file) {
		return nil, errGenerated
	}
	s := &source{file: file, fset: fset, path: path, lines: strings.Split(string(src), "\n"), ignores: map[string]map[int]bool{}}
	s.collectIgnores(t)
	return s, nil
}

// runRules applies every rule to s.
func (s *source) runRules() {
	s.checkParallel()
	s.checkBlankLine()
	s.checkRunHint()
	s.checkBodyRead()
	s.checkStorageFake()
	s.checkBenchLoop()
}

// collectIgnores records the rules silenced by marker comments. A comment
// silences the line it ends on, which the rules extend to the line below.
func (s *source) collectIgnores(t *testing.T) {
	t.Helper()
	for _, group := range s.file.Comments {
		for _, c := range group.List {
			_, rest, ok := strings.Cut(c.Text, ignoreMarker)
			if !ok {
				continue
			}
			rule, reason, _ := strings.Cut(rest, ":")
			rule = strings.TrimSpace(rule)
			line := s.fset.Position(group.End()).Line
			require.NotEmpty(t, strings.TrimSpace(reason), "%s:%d: %q needs a reason after the colon", s.path, line, c.Text)
			if s.ignores[rule] == nil {
				s.ignores[rule] = map[int]bool{}
			}
			s.ignores[rule][line] = true
		}
	}
}

// isTestingParam reports whether field is a parameter of type *testing.<name>.
func isTestingParam(field *ast.Field, name string) bool {
	star, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "testing" && sel.Sel.Name == name
}

// paramName returns the name of the single *testing.<kind> parameter of a
// function type, or "" when it has another shape.
func paramName(ft *ast.FuncType, kind string) string {
	if ft.Params == nil || len(ft.Params.List) != 1 || len(ft.Params.List[0].Names) != 1 {
		return ""
	}
	if !isTestingParam(ft.Params.List[0], kind) {
		return ""
	}
	return ft.Params.List[0].Names[0].Name
}

// callName returns "x.Sel" or "Sel" for a call's function, and "" otherwise.
func callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			return x.Name + "." + fn.Sel.Name
		}
		return fn.Sel.Name
	case *ast.Ident:
		return fn.Name
	}
	return ""
}

// cannotBeParallel reports whether the body calls something Go refuses in a
// parallel test or in a test with a parallel ancestor.
func cannotBeParallel(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			name := callName(call)
			if name == "testing.AllocsPerRun" || strings.HasSuffix(name, ".Setenv") || strings.HasSuffix(name, ".Chdir") {
				found = true
			}
		}
		return !found
	})
	return found
}

// checkParallel requires every test and subtest to start with t.Parallel().
// Tests that were sequential before the rule are listed in the baseline.
func (s *source) checkParallel() {
	check := func(name string, ft *ast.FuncType, body *ast.BlockStmt, pos token.Pos) {
		param := paramName(ft, "T")
		if param == "" || body == nil || cannotBeParallel(body) {
			return
		}
		isParallel := func(call *ast.CallExpr) bool { return callName(call) == param+".Parallel" }
		if len(body.List) > 0 {
			if stmt, ok := body.List[0].(*ast.ExprStmt); ok {
				if call, ok := stmt.X.(*ast.CallExpr); ok && isParallel(call) {
					return
				}
			}
		}
		later := false
		for _, stmt := range body.List {
			if es, ok := stmt.(*ast.ExprStmt); ok {
				if call, ok := es.X.(*ast.CallExpr); ok && isParallel(call) {
					later = true
				}
			}
		}
		if later {
			s.reportf(ruleParallel, pos, "%s calls %s.Parallel() after other statements; make it the first one", name, param)
			return
		}
		s.reportf(ruleParallel, pos, "%s does not call %s.Parallel(); add it, or mark the reason with // %s%s: <why>", name, param, ignoreMarker, ruleParallel)
	}
	for _, decl := range s.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		s.scope = fn.Name.Name
		if strings.HasPrefix(fn.Name.Name, "Test") && fn.Recv == nil {
			check(fn.Name.Name, fn.Type, fn.Body, fn.Pos())
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && strings.HasSuffix(callName(call), ".Run") && len(call.Args) == 2 {
				if lit, ok := call.Args[1].(*ast.FuncLit); ok {
					check("a subtest of "+fn.Name.Name, lit.Type, lit.Body, call.Pos())
				}
			}
			return true
		})
	}
	s.scope = ""
}

// checkBlankLine forbids a blank line right after t.Parallel().
func (s *source) checkBlankLine() {
	ast.Inspect(s.file, func(n ast.Node) bool {
		stmt, ok := n.(*ast.ExprStmt)
		if !ok {
			return true
		}
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok || !strings.HasSuffix(callName(call), ".Parallel") {
			return true
		}
		line := s.fset.Position(stmt.End()).Line // 1-based line of the call; s.lines[line] is the next one
		if line < len(s.lines) && strings.TrimSpace(s.lines[line]) == "" {
			s.reportf(ruleBlankLine, stmt.Pos(), "blank line right after t.Parallel()")
		}
		return true
	})
}

// checkRunHint forbids "// go test -run ..." hint comments.
func (s *source) checkRunHint() {
	for _, group := range s.file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(c.Text, "//")), "go test -run") {
				s.reportf(ruleHint, c.Pos(), "remove the `go test -run` hint comment")
			}
		}
	}
}

// checkBodyRead flags io.ReadAll(resp.Body), which apptest.Body does and closes.
func (s *source) checkBodyRead() {
	if strings.HasPrefix(s.path, "internal/apptest/") {
		return
	}
	ast.Inspect(s.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || callName(call) != "io.ReadAll" || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Args[0].(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Body" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "resp" || id.Name == "res" || id.Name == "response") {
			s.reportf(ruleBodyRead, call.Pos(), "read response bodies with apptest.Body or apptest.BodyBytes; mark a read that must see the error")
		}
		return true
	})
}

var storageMethods = []string{"Get", "Set", "Delete", "Reset", "Close"}

// checkStorageFake flags types that implement the whole fiber.Storage method
// set; internal/storagetest holds the fakes.
func (s *source) checkStorageFake() {
	if strings.HasPrefix(s.path, "internal/storagetest/") {
		return
	}
	methods := map[string][]string{}
	pos := map[string]token.Pos{}
	for _, decl := range s.file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					pos[ts.Name.Name] = gen.Pos() // the marker goes above the type declaration
				}
			}
			continue
		}
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		recv := fn.Recv.List[0].Type
		if star, isPtr := recv.(*ast.StarExpr); isPtr {
			recv = star.X
		}
		id, ok := recv.(*ast.Ident)
		if !ok {
			continue
		}
		methods[id.Name] = append(methods[id.Name], fn.Name.Name)
		if _, seen := pos[id.Name]; !seen {
			pos[id.Name] = fn.Pos()
		}
	}
	for typ, names := range methods {
		if !slices.ContainsFunc(storageMethods, func(m string) bool { return !slices.Contains(names, m) }) {
			s.reportf(ruleStorageFake, pos[typ], "%s looks like a fake fiber.Storage; use storagetest.Memory or storagetest.Failing, or embed them", typ)
		}
	}
}

// checkBenchLoop requires the timed loop to stay in the named benchmark: a
// function that is not a Benchmark must not run b.Loop(), b.RunParallel or pb.Next().
func (s *source) checkBenchLoop() {
	for _, decl := range s.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || strings.HasPrefix(fn.Name.Name, "Benchmark") {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			// A closure in a Benchmark is fine; here we are in a helper.
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch name := callName(call); {
			case strings.HasSuffix(name, ".Loop"), strings.HasSuffix(name, ".RunParallel"), strings.HasSuffix(name, ".Next") && strings.HasPrefix(name, "pb."):
				s.reportf(ruleBenchLoop, fn.Pos(), "%s runs a timed loop; keep the loop in the named Benchmark and share only the setup", fn.Name.Name)
				return false
			}
			return true
		})
	}
}

// baselinePath lists the tests that were sequential before the parallel rule,
// as "<file> <function> <count>" lines, one per function with violations. The
// list may only shrink: a new violation fails the test, and so does an entry
// that is out of date. UPDATE_CONVENTIONS_BASELINE=1 rewrites it.
const baselinePath = "testdata/parallel_baseline.txt"

func readBaseline(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(baselinePath)
	require.NoError(t, err)
	base := map[string]int{}
	for line := range strings.SplitSeq(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		require.Len(t, fields, 3, "baseline line %q", line)
		var n int
		_, err := fmt.Sscan(fields[2], &n)
		require.NoError(t, err)
		base[fields[0]+" "+fields[1]] = n
	}
	return base
}

func writeBaseline(t *testing.T, counts map[string]int) {
	t.Helper()
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var b strings.Builder
	fmt.Fprintln(&b, "# Tests that were sequential before the parallel rule: <file> <function> <violations>.")
	fmt.Fprintln(&b, "# Make them parallel and delete the line; never add one. See conventions_test.go.")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s %d\n", k, counts[k])
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(baselinePath), 0o750))
	require.NoError(t, os.WriteFile(baselinePath, []byte(b.String()), 0o600))
}

func Test_TestConventions(t *testing.T) {
	t.Parallel()
	var problems []string
	counts := map[string]int{}
	parallel := map[string][]finding{}
	for _, s := range testFiles(t) {
		s.runRules()
		for _, f := range s.findings {
			if f.rule != ruleParallel {
				problems = append(problems, f.String())
				continue
			}
			key := f.pos.Filename + " " + f.scope
			counts[key]++
			parallel[key] = append(parallel[key], f)
		}
	}

	if os.Getenv("UPDATE_CONVENTIONS_BASELINE") != "" {
		writeBaseline(t, counts)
	}
	base := readBaseline(t)
	for key, n := range counts {
		if n > base[key] {
			for _, f := range parallel[key] {
				problems = append(problems, f.String())
			}
		}
	}
	for key, n := range base {
		if counts[key] < n {
			problems = append(problems, fmt.Sprintf("%s: the baseline lists %d sequential test(s) there, found %d; lower or delete the line in %s", key, n, counts[key], baselinePath))
		}
	}

	slices.Sort(problems)
	require.Empty(t, problems, "test convention violations (see AGENTS.md); a justified exception takes a // %s<rule>: <reason> comment", ignoreMarker)
}

// Test_TestConventions_Rules pins what each rule flags and what it lets through.
func Test_TestConventions_Rules(t *testing.T) {
	t.Parallel()
	const head = "package x\n\nimport (\n\t\"io\"\n\t\"testing\"\n)\n\nvar _ = io.EOF\n\n"
	tests := []struct {
		name, code string
		want       []string // rules, in report order
	}{
		{"parallel test", "func TestA(t *testing.T) {\n\tt.Parallel()\n}", nil},
		{"missing parallel", "func TestA(t *testing.T) {\n}", []string{ruleParallel}},
		{"late parallel", "func TestA(t *testing.T) {\n\t_ = 1\n\tt.Parallel()\n}", []string{ruleParallel}},
		{"subtest without parallel", "func TestA(t *testing.T) {\n\tt.Parallel()\n\tt.Run(\"a\", func(t *testing.T) {})\n}", []string{ruleParallel}},
		{"subtest with parallel", "func TestA(t *testing.T) {\n\tt.Parallel()\n\tt.Run(\"a\", func(t *testing.T) {\n\t\tt.Parallel()\n\t})\n}", nil},
		{"Setenv cannot be parallel", "func TestA(t *testing.T) {\n\tt.Setenv(\"A\", \"b\")\n}", nil},
		{"AllocsPerRun cannot be parallel", "func TestA(t *testing.T) {\n\ttesting.AllocsPerRun(1, func() {})\n}", nil},
		{"marked exception", "// conventions:ignore parallel: global state\nfunc TestA(t *testing.T) {\n}", nil},
		{"benchmark needs no parallel", "func BenchmarkA(b *testing.B) {\n}", nil},
		{"blank line after parallel", "func TestA(t *testing.T) {\n\tt.Parallel()\n\n\t_ = 1\n}", []string{ruleBlankLine}},
		{"run hint", "// go test -run TestA\nfunc TestA(t *testing.T) {\n\tt.Parallel()\n}", []string{ruleHint}},
		{"response body read", "func TestA(t *testing.T) {\n\tt.Parallel()\n\tvar resp struct{ Body io.Reader }\n\t_, _ = io.ReadAll(resp.Body)\n}", []string{ruleBodyRead}},
		{"request body read", "func TestA(t *testing.T) {\n\tt.Parallel()\n\tvar r struct{ Body io.Reader }\n\t_, _ = io.ReadAll(r.Body)\n}", nil},
		{"storage fake", "type fake struct{}\nfunc (fake) Get(string) {}\nfunc (fake) Set() {}\nfunc (fake) Delete() {}\nfunc (fake) Reset() {}\nfunc (fake) Close() {}", []string{ruleStorageFake}},
		{"partial storage", "type fake struct{}\nfunc (fake) Get(string) {}\nfunc (fake) Set() {}", nil},
		{"timed loop in a helper", "func helper(b *testing.B) {\n\tfor b.Loop() {\n\t}\n}", []string{ruleBenchLoop}},
		{"timed loop in a benchmark", "func BenchmarkA(b *testing.B) {\n\tfor b.Loop() {\n\t}\n}", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, err := newSource(t, "x_test.go", []byte(head+tc.code+"\n"))
			require.NoError(t, err)
			s.runRules()
			var got []string
			for _, f := range s.findings {
				got = append(got, f.rule)
			}
			require.Equal(t, tc.want, got)
		})
	}
}
