// Command genapidocs updates marked Markdown blocks from Go declarations and examples.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag" //nolint:depguard // Only the standalone generator entry point parses flags.
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	errOutputRequired    = errors.New("an existing Markdown output file is required")
	errNoBlocks          = errors.New("no go:doc blocks found")
	errMalformedBlocks   = errors.New("unbalanced or malformed go:doc blocks")
	errUnknownSymbol     = errors.New("unknown exported declaration or example")
	errUnplayableExample = errors.New("example cannot be rendered as a standalone program")
	// The lazy optional body lets the closing marker follow the opening line directly.
	blockPattern = regexp.MustCompile(`(?s)(<!-- go:doc ([^>\r\n]+) -->\r?\n)(?:.*?\r?\n)??(<!-- /go:doc -->)`)
)

type packageDocs struct {
	symbols map[string]symbolDoc
	fset    *token.FileSet
}

type symbolDoc struct {
	node      ast.Node
	text      string
	output    string
	comments  []*ast.CommentGroup
	unordered bool
}

// main regenerates the current package's marked documentation blocks.
func main() {
	output := flag.String("out", "", "existing Markdown file containing go:doc blocks")
	flag.Parse()
	if err := generate(".", *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// generate validates all blocks before replacing a documentation file.
func generate(dir, output string) error {
	if output == "" {
		return errOutputRequired
	}
	markdown, err := os.ReadFile(output) //nolint:gosec // The maintainer explicitly selects this existing Markdown file.
	if err != nil {
		return fmt.Errorf("read documentation: %w", err)
	}
	docs, err := loadPackage(dir)
	if err != nil {
		return err
	}
	rendered, err := render(markdown, docs)
	if err != nil {
		return err
	}
	if bytes.Equal(markdown, rendered) {
		return nil
	}
	if err := writeAtomic(output, rendered); err != nil {
		return fmt.Errorf("write documentation: %w", err)
	}
	return nil
}

// writeAtomic replaces path via a sibling temp file so an interrupted write never truncates the page.
func writeAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".genapidocs-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // Already renamed on success; best-effort cleanup otherwise.
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), info.Mode().Perm())
	}
	if err != nil {
		return fmt.Errorf("prepare temp file: %w", err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace: %w", err)
	}
	return nil
}

// loadPackage uses the Go tool's file selection so build constraints match go generate.
func loadPackage(dir string) (packageDocs, error) {
	// -find skips dependency resolution; only the package's own file lists are needed.
	cmd := exec.Command("go", "list", "-find", "-json=Dir,ImportPath,GoFiles,CgoFiles,TestGoFiles,XTestGoFiles", ".")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return packageDocs{}, fmt.Errorf("go list: %s: %w", stderr.String(), err)
	}
	var info build.Package
	if err = json.Unmarshal(output, &info); err != nil {
		return packageDocs{}, fmt.Errorf("decode go list: %w", err)
	}
	fset := token.NewFileSet()
	names := slices.Concat(info.GoFiles, info.CgoFiles, info.TestGoFiles, info.XTestGoFiles)
	files := make([]*ast.File, 0, len(names))
	var tests []*ast.File
	// Built before go/doc filters the AST, so comments of removed unexported nodes stay unreachable.
	commentMap := ast.CommentMap{}
	for _, name := range names {
		file, parseErr := parser.ParseFile(fset, filepath.Join(info.Dir, name), nil, parser.ParseComments)
		if parseErr != nil {
			return packageDocs{}, fmt.Errorf("parse %s: %w", name, parseErr)
		}
		files = append(files, file)
		if strings.HasSuffix(name, "_test.go") {
			tests = append(tests, file)
			continue
		}
		maps.Copy(commentMap, ast.NewCommentMap(fset, file, file.Comments))
	}
	examples := doc.Examples(tests...)
	pkg, err := doc.NewFromFiles(fset, files, info.ImportPath)
	if err != nil {
		return packageDocs{}, fmt.Errorf("extract package documentation: %w", err)
	}
	symbols := make(map[string]symbolDoc)
	values := slices.Concat(pkg.Consts, pkg.Vars)
	functions := slices.Clone(pkg.Funcs)
	for _, typ := range pkg.Types {
		symbols[typ.Name] = symbolDoc{node: typ.Decl, text: typ.Doc}
		values = append(values, typ.Consts...)
		values = append(values, typ.Vars...)
		functions = append(functions, typ.Funcs...)
		for _, method := range typ.Methods {
			symbols[typ.Name+"."+method.Name] = symbolDoc{node: method.Decl, text: method.Doc}
		}
	}
	for _, value := range values {
		// Value literals are shown as copyable code, so drop go/doc's "filtered fields" placeholder.
		ast.Inspect(value.Decl, func(n ast.Node) bool {
			if lit, ok := n.(*ast.CompositeLit); ok {
				lit.Incomplete = false
			}
			return true
		})
		// Only value literals need free comments; type and func comments sit in Doc/Comment fields.
		comments := innerComments(value.Decl, commentMap)
		for _, name := range value.Names {
			symbols[name] = symbolDoc{node: value.Decl, text: value.Doc, comments: comments}
		}
	}
	for _, function := range functions {
		symbols[function.Name] = symbolDoc{node: function.Decl, text: function.Doc}
	}
	for _, example := range examples {
		name := "Example" + example.Name
		play := playable(example)
		if _, exists := symbols[name]; exists && play == nil {
			continue // An internal and an external example share the name; keep the playable one.
		}
		// The example's doc comment names a test function the program does not contain, so it is not published.
		symbol := symbolDoc{output: example.Output, unordered: example.Unordered}
		if play != nil {
			symbol.node = play
		}
		symbols[name] = symbol
	}
	return packageDocs{symbols: symbols, fset: fset}, nil
}

// innerComments returns the comments go/doc's filtered declaration still reaches, minus its doc comment.
func innerComments(decl ast.Decl, commentMap ast.CommentMap) []*ast.CommentGroup {
	var groups []*ast.CommentGroup
	ast.Inspect(decl, func(n ast.Node) bool {
		for _, group := range commentMap[n] {
			if group.Pos() >= decl.Pos() && !slices.Contains(groups, group) {
				groups = append(groups, group)
			}
		}
		return true
	})
	slices.SortFunc(groups, func(a, b *ast.CommentGroup) int { return int(a.Pos() - b.Pos()) })
	return groups
}

// playable returns the example as a standalone program, or nil when it is not one.
func playable(example *doc.Example) *ast.File {
	play := example.Play
	if play == nil {
		return nil
	}
	// go/doc checks single-function examples itself but trusts whole-file examples blindly.
	if file, ok := example.Code.(*ast.File); ok && !selfContained(file) {
		return nil
	}
	play.Decls = slices.DeleteFunc(slices.Clone(play.Decls), func(decl ast.Decl) bool {
		gen, ok := decl.(*ast.GenDecl)
		return ok && gen.Tok == token.IMPORT && len(gen.Specs) == 0
	})
	return play
}

// selfContained reports whether every identifier the file leaves unresolved is a builtin or an import.
func selfContained(file *ast.File) bool {
	unresolved := make(map[*ast.Ident]bool, len(file.Unresolved))
	for _, ident := range file.Unresolved {
		unresolved[ident] = types.Universe.Lookup(ident.Name) == nil
	}
	// shortcut: any unresolved selector base is taken as a package name; compile the program if that ever lets a stray variable through.
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				delete(unresolved, ident)
			}
		}
		if kv, ok := n.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok {
				delete(unresolved, ident) // Struct field keys never resolve at file scope.
			}
		}
		return true
	})
	for _, foreign := range unresolved {
		if foreign {
			return false
		}
	}
	return true
}

// render preserves handwritten prose and fails on stale or incomplete block markers.
func render(markdown []byte, docs packageDocs) ([]byte, error) {
	matches := blockPattern.FindAllSubmatch(markdown, -1)
	openCount := bytes.Count(markdown, []byte("<!-- go:doc "))
	if openCount == 0 {
		return nil, errNoBlocks
	}
	if closeCount := bytes.Count(markdown, []byte("<!-- /go:doc -->")); len(matches) != openCount || closeCount != openCount {
		return nil, fmt.Errorf("%w: %d openers, %d closers, %d complete blocks", errMalformedBlocks, openCount, closeCount, len(matches))
	}
	var renderErr error
	result := blockPattern.ReplaceAllFunc(markdown, func(block []byte) []byte {
		if renderErr != nil {
			return block
		}
		parts := blockPattern.FindSubmatch(block)
		names := strings.Fields(string(parts[2]))
		if len(names) == 0 {
			renderErr = errMalformedBlocks
			return block
		}
		var code bytes.Buffer
		seen := map[ast.Node]bool{}
		for _, name := range names {
			symbol, ok := docs.symbols[name]
			if !ok {
				renderErr = fmt.Errorf("%w: %s", errUnknownSymbol, name)
				return block
			}
			if symbol.node == nil {
				renderErr = fmt.Errorf("%w: %s", errUnplayableExample, name)
				return block
			}
			if seen[symbol.node] {
				continue // Names of one const or var group share a declaration.
			}
			seen[symbol.node] = true
			fmt.Fprintln(&code)
			fmt.Fprint(&code, "```go\n")
			if text := strings.TrimSpace(symbol.text); text != "" {
				for line := range strings.SplitSeq(text, "\n") {
					fmt.Fprint(&code, strings.TrimRight("// "+line, " \t")+"\n")
				}
			}
			var node any = symbol.node
			if symbol.comments != nil {
				node = &printer.CommentedNode{Node: symbol.node, Comments: symbol.comments}
			}
			var formatted bytes.Buffer
			config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8} // gofmt's settings
			if err := config.Fprint(&formatted, docs.fset, node); err != nil {
				renderErr = fmt.Errorf("format %s: %w", name, err)
				return block
			}
			code.Write(bytes.TrimRight(formatted.Bytes(), "\n"))
			fmt.Fprint(&code, "\n```\n")
			if symbol.output != "" {
				label := "Output:"
				if symbol.unordered {
					label = "Unordered output:"
				}
				fmt.Fprint(&code, "\n", label, "\n\n```text\n", symbol.output, "```\n")
			}
		}
		// markdownlint MD010 rejects hard tabs, which gofmt indentation and doc code blocks carry.
		expanded := bytes.ReplaceAll(code.Bytes(), []byte("\t"), []byte("    "))
		return slices.Concat(parts[1], expanded, []byte("\n"), parts[3])
	})
	if renderErr != nil {
		return nil, renderErr
	}
	return result, nil
}
