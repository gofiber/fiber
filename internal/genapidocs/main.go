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
	errAmbiguousSymbol   = errors.New("ambiguous declaration or example name")
	errGeneratedMarker   = errors.New("generated text contains a go:doc marker")
	errUndocumentedAPI   = errors.New("exported declarations are not documented")
	fencePattern         = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
)

type packageDocs struct {
	symbols  map[string]symbolDoc
	fset     *token.FileSet
	required []string
}

type symbolDoc struct {
	node      ast.Node
	text      string
	output    string
	comments  []*ast.CommentGroup
	unordered bool
	fragment  bool
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
	info, err := os.Stat(path) //nolint:gosec // The maintainer explicitly selects this existing output path.
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
		err = os.Chmod(tmp.Name(), info.Mode().Perm()) //nolint:gosec // This command creates the sibling temp file.
	}
	if err != nil {
		return fmt.Errorf("prepare temp file: %w", err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil { //nolint:gosec // This CLI intentionally replaces the selected output path.
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
		// Value literals retain source expressions without go/doc's "filtered fields" placeholder.
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
	required := slices.Sorted(maps.Keys(symbols))
	for _, typ := range pkg.Types {
		ast.Inspect(typ.Decl, func(node ast.Node) bool {
			var fields []*ast.Field
			switch node := node.(type) {
			case *ast.StructType:
				fields = node.Fields.List
			case *ast.InterfaceType:
				fields = node.Methods.List
			}
			for _, field := range fields {
				list := &ast.FieldList{List: []*ast.Field{field}}
				var member ast.Expr = &ast.StructType{Fields: list}
				if _, isInterface := node.(*ast.InterfaceType); isInterface {
					member = &ast.InterfaceType{Methods: list}
				}
				for _, name := range field.Names {
					if ast.IsExported(name.Name) {
						symbols[typ.Name+"."+name.Name] = symbolDoc{node: member, fragment: true}
					}
				}
			}

			return fields == nil
		})
	}
	exampleNames := make(map[string]bool)
	for _, example := range examples {
		name := "Example" + example.Name
		play := playable(example)
		if previous, exists := symbols[name]; exists {
			if !exampleNames[name] || previous.node != nil && play != nil {
				return packageDocs{}, fmt.Errorf("%w: %s", errAmbiguousSymbol, name)
			}
			if play == nil {
				continue // Keep a playable example over an unplayable one with the same name.
			}
		}
		exampleNames[name] = true
		// The example's doc comment names a test function the program does not contain, so it is not published.
		symbol := symbolDoc{output: example.Output, unordered: example.Unordered}
		if play != nil {
			symbol.node = play
		}
		symbols[name] = symbol
	}
	return packageDocs{symbols: symbols, fset: fset, required: required}, nil
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

// render replaces whole-line markers outside code fences and preserves the file's line endings.
func render(markdown []byte, docs packageDocs) ([]byte, error) {
	newline := "\n"
	if first := bytes.IndexByte(markdown, '\n'); first > 0 && markdown[first-1] == '\r' {
		newline = "\r\n"
	}
	var result bytes.Buffer
	var names []string
	var fence string
	blocks := 0
	checkAll := false
	selected := map[ast.Node]bool{}
	for index, line := range strings.SplitAfter(string(markdown), "\n") {
		content := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		match := fencePattern.FindStringSubmatch(content)
		if match != nil {
			if fence == "" {
				fence = match[1]
			} else if match[1][0] == fence[0] && len(match[1]) >= len(fence) && strings.TrimSpace(match[2]) == "" {
				fence = ""
			}
		}
		if fence != "" || match != nil || strings.HasPrefix(content, "    ") || strings.HasPrefix(content, "\t") {
			if names == nil {
				fmt.Fprint(&result, line)
			}
			continue
		}
		content = strings.TrimSpace(content)
		if rest, opener := strings.CutPrefix(content, "<!-- go:doc "); opener && strings.HasSuffix(rest, " -->") {
			if names != nil {
				return nil, fmt.Errorf("%w at line %d", errMalformedBlocks, index+1)
			}
			names = strings.Fields(strings.TrimSuffix(rest, " -->"))
			if len(names) == 0 {
				return nil, fmt.Errorf("%w at line %d", errMalformedBlocks, index+1)
			}
			fmt.Fprint(&result, line)
			continue
		}
		if content == "<!-- /go:doc -->" {
			if names == nil {
				return nil, fmt.Errorf("%w at line %d", errMalformedBlocks, index+1)
			}
			body, err := renderSymbols(names, docs, selected)
			if err != nil {
				return nil, err
			}
			result.Write(bytes.ReplaceAll(body, []byte("\n"), []byte(newline)))
			fmt.Fprint(&result, newline, line)
			names = nil
			blocks++
			continue
		}
		if names == nil {
			checkAll = checkAll || content == "<!-- go:doc-all -->"
			fmt.Fprint(&result, line)
		}
	}
	if names != nil {
		return nil, fmt.Errorf("%w: missing closing marker", errMalformedBlocks)
	}
	if blocks == 0 {
		return nil, errNoBlocks
	}
	if checkAll {
		var missing []string
		for _, name := range docs.required {
			if !selected[docs.symbols[name].node] {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("%w: %s", errUndocumentedAPI, strings.Join(missing, ", "))
		}
	}
	return result.Bytes(), nil
}

// renderSymbols prints declarations once per block and rejects marker text in generated content.
func renderSymbols(names []string, docs packageDocs, selected map[ast.Node]bool) ([]byte, error) {
	var code bytes.Buffer
	seen := map[ast.Node]bool{}
	for _, name := range names {
		symbol, ok := docs.symbols[name]
		if !ok {
			return nil, fmt.Errorf("%w: %s", errUnknownSymbol, name)
		}
		if symbol.node == nil {
			return nil, fmt.Errorf("%w: %s", errUnplayableExample, name)
		}
		if seen[symbol.node] {
			continue
		}
		seen[symbol.node] = true
		selected[symbol.node] = true
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
		config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
		if err := config.Fprint(&formatted, docs.fset, node); err != nil {
			return nil, fmt.Errorf("format %s: %w", name, err)
		}
		printed := bytes.TrimRight(formatted.Bytes(), "\n")
		if symbol.fragment {
			printed = bytes.TrimSpace(printed[bytes.IndexByte(printed, '{')+1 : bytes.LastIndexByte(printed, '}')])
		}
		code.Write(printed)
		fmt.Fprint(&code, "\n```\n")
		if symbol.output != "" {
			label := "Output:"
			if symbol.unordered {
				label = "Unordered output:"
			}
			fmt.Fprint(&code, "\n", label, "\n\n```text\n", symbol.output, "```\n")
		}
	}
	if bytes.Contains(code.Bytes(), []byte("<!-- go:doc")) || bytes.Contains(code.Bytes(), []byte("<!-- /go:doc")) {
		return nil, errGeneratedMarker
	}
	return bytes.ReplaceAll(code.Bytes(), []byte("\t"), []byte("    ")), nil
}
