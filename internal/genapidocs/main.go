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
	blockPattern         = regexp.MustCompile(`(?s)(<!-- go:doc ([^>\r\n]+) -->\r?\n).*?(\r?\n<!-- /go:doc -->)`)
)

type packageDocs struct {
	symbols map[string]symbolDoc
	fset    *token.FileSet
}

type symbolDoc struct {
	node   ast.Node
	text   string
	output string
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
	if err := os.WriteFile(output, rendered, 0o600); err != nil { //nolint:gosec // This CLI intentionally updates the selected documentation file.
		return fmt.Errorf("write documentation: %w", err)
	}
	return nil
}

// loadPackage uses the Go tool's file selection so build constraints match go generate.
func loadPackage(dir string) (packageDocs, error) {
	cmd := exec.Command("go", "list", "-json", ".")
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
	names := slices.Concat(info.GoFiles, info.TestGoFiles, info.XTestGoFiles)
	files := make([]*ast.File, 0, len(names))
	var tests []*ast.File
	for _, name := range names {
		file, parseErr := parser.ParseFile(fset, filepath.Join(info.Dir, name), nil, parser.ParseComments)
		if parseErr != nil {
			return packageDocs{}, fmt.Errorf("parse %s: %w", name, parseErr)
		}
		files = append(files, file)
		if strings.HasSuffix(name, "_test.go") {
			tests = append(tests, file)
		}
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
		for _, name := range value.Names {
			symbols[name] = symbolDoc{node: value.Decl, text: value.Doc}
		}
	}
	for _, function := range functions {
		symbols[function.Name] = symbolDoc{node: function.Decl, text: function.Doc}
	}
	for _, example := range examples {
		name := "Example" + example.Name
		symbols[name] = symbolDoc{text: example.Doc, output: example.Output}
		if example.Play != nil {
			symbols[name] = symbolDoc{node: example.Play, text: example.Doc, output: example.Output}
		}
	}
	return packageDocs{symbols: symbols, fset: fset}, nil
}

// render preserves handwritten prose and fails on stale or incomplete block markers.
func render(markdown []byte, docs packageDocs) ([]byte, error) {
	matches := blockPattern.FindAllSubmatch(markdown, -1)
	openCount := bytes.Count(markdown, []byte("<!-- go:doc "))
	if openCount == 0 {
		return nil, errNoBlocks
	}
	if len(matches) != openCount || bytes.Count(markdown, []byte("<!-- /go:doc -->")) != openCount {
		return nil, errMalformedBlocks
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
		code.Write(parts[1])
		fmt.Fprintln(&code)
		for i, name := range names {
			symbol, ok := docs.symbols[name]
			if !ok {
				renderErr = fmt.Errorf("%w: %s", errUnknownSymbol, name)
				return block
			}
			if symbol.node == nil {
				renderErr = fmt.Errorf("%w: %s", errUnplayableExample, name)
				return block
			}
			if i > 0 {
				fmt.Fprintln(&code)
			}
			fmt.Fprint(&code, "```go\n")
			for line := range strings.SplitSeq(strings.TrimSpace(symbol.text), "\n") {
				if symbol.text != "" {
					fmt.Fprint(&code, strings.TrimRight("// "+line, " ")+"\n")
				}
			}
			var formatted bytes.Buffer
			config := printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}
			if err := config.Fprint(&formatted, docs.fset, symbol.node); err != nil {
				renderErr = fmt.Errorf("format %s: %w", name, err)
				return block
			}
			code.Write(bytes.TrimRight(formatted.Bytes(), "\n"))
			fmt.Fprint(&code, "\n```\n")
			if symbol.output != "" {
				fmt.Fprint(&code, "\nOutput:\n\n```text\n")
				fmt.Fprint(&code, symbol.output)
				fmt.Fprint(&code, "```\n")
			}
		}
		code.Write(parts[3])
		return code.Bytes()
	})
	if renderErr != nil {
		return nil, renderErr
	}
	return result, nil
}
