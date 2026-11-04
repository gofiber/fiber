package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDocumentationCompleteness catches additions to packages that opt into complete API coverage.
func TestDocumentationCompleteness(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"func Added() {}", "func (Config) Added() {}", "type Added int"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			dir := fixture(t)
			docs, err := loadPackage(dir)
			require.NoError(t, err)
			input := "<!-- go:doc-all -->\n<!-- go:doc " + strings.Join(docs.required, " ") + " -->\n<!-- /go:doc -->\n"
			rendered, err := render([]byte(input), docs)
			require.NoError(t, err)
			require.Equal(t, 1, strings.Count(string(rendered), "KindA Kind = iota"))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "added.go"), []byte("package sample\n"+source+"\n"), 0o600))
			docs, err = loadPackage(dir)
			require.NoError(t, err)
			_, err = render(rendered, docs)
			require.ErrorIs(t, err, errUndocumentedAPI)
			require.ErrorContains(t, err, "Added")
		})
	}
}

// TestInnerComments retains and sorts comments inside a value without duplicating its doc comment.
func TestInnerComments(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", "package sample\n// Settings documents the variable.\nvar Settings = struct{ X, Y int }{\n// first\nX: 1,\n// second\nY: 2,\n}\n", parser.ParseComments)
	require.NoError(t, err)
	groups := innerComments(file.Decls[0], ast.NewCommentMap(fset, file, file.Comments))
	require.Len(t, groups, 2)
	require.Equal(t, "first\n", groups[0].Text())
	require.Equal(t, "second\n", groups[1].Text())
}

// TestPackageMembers exposes interface methods and struct fields without indexing nested fields on the parent.
func TestPackageMembers(t *testing.T) {
	t.Parallel()
	dir := fixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "members.go"), []byte("package sample\ntype API interface { Fetch() string; hidden() }\ntype Nested struct { Value struct { Leaf int } }\ntype Paired struct { First, Second int }\n"), 0o600))
	docs, err := loadPackage(dir)
	require.NoError(t, err)
	out, err := render([]byte("<!-- go:doc API.Fetch Config.Value Paired.First Paired.Second -->\n<!-- /go:doc -->"), docs)
	require.NoError(t, err)
	require.Contains(t, string(out), "Fetch() string")
	require.Contains(t, string(out), "Value int")
	require.NotContains(t, docs.symbols, "API.hidden")
	require.NotContains(t, docs.symbols, "Nested.Leaf")
	require.Equal(t, 1, strings.Count(string(out), "First, Second int"))
}

// TestPackageNameCollisions makes example selection deterministic and rejects ambiguous public names.
func TestPackageNameCollisions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		want     error
		name     string
		source   string
		external string
	}{
		{name: "declaration", source: "package sample\nvar ExampleDouble = 1\n", want: errAmbiguousSymbol},
		{name: "two playable", source: "package sample\nimport \"fmt\"\nfunc ExampleDouble() { fmt.Println(4) }\nfunc helperExample() {}\n", want: errAmbiguousSymbol},
		{name: "two unplayable", source: "package sample\nimport \"fmt\"\nfunc ExampleDouble_unplayable() { fmt.Println(hiddenVar) }\n"},
		{name: "internal unplayable", source: "package sample\nimport \"fmt\"\nfunc ExampleDouble() { fmt.Println(hiddenVar) }\n"},
		{name: "external unplayable", source: "package sample\nimport \"fmt\"\nfunc ExampleDouble() { fmt.Println(4) }\nfunc helperExample() {}\n", external: "package sample_test\nimport \"fmt\"\nfunc ExampleDouble() { fmt.Println(value) }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := fixture(t)
			name := "internal_test.go"
			if tc.name == "declaration" {
				name = "declaration.go"
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(tc.source), 0o600))
			if tc.external != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "example_test.go"), []byte(tc.external), 0o600))
			}
			docs, err := loadPackage(dir)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, docs.symbols["ExampleDouble"].node)
		})
	}
}

// TestRenderGeneratedMarkers prevents output from making its own markers ambiguous on the next run.
func TestRenderGeneratedMarkers(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{"<!-- go:doc Other -->", "<!-- /go:doc -->"} {
		t.Run(marker, func(t *testing.T) {
			t.Parallel()
			docs := packageDocs{fset: token.NewFileSet(), symbols: map[string]symbolDoc{"Value": {node: ast.NewIdent("Value"), output: marker + "\n"}}}
			_, err := render([]byte("<!-- go:doc Value -->\n<!-- /go:doc -->"), docs)
			require.ErrorIs(t, err, errGeneratedMarker)
		})
	}
}

// TestRenderMarkerLayout preserves examples of marker syntax and both supported line endings.
func TestRenderMarkerLayout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		prefix string
	}{
		{name: "inline", prefix: "Use <!-- go:doc Missing --> inline.\n"},
		{name: "backticks", prefix: "```markdown\n<!-- go:doc-all -->\n<!-- go:doc Missing -->\n<!-- /go:doc -->\n```\n"},
		{name: "long fence", prefix: "````markdown\n```\n<!-- go:doc Missing -->\n<!-- /go:doc -->\n````\n"},
		{name: "tilde fence", prefix: "~~~markdown\n```\n<!-- go:doc Missing -->\n<!-- /go:doc -->\n~~~\n"},
		{name: "spaces", prefix: "    <!-- go:doc Missing -->\n    <!-- /go:doc -->\n"},
		{name: "tabs", prefix: "\t<!-- go:doc Missing -->\n\t<!-- /go:doc -->\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			docs, err := loadPackage(fixture(t))
			require.NoError(t, err)
			for _, newline := range []string{"\n", "\r\n"} {
				prefix := strings.ReplaceAll(tc.prefix, "\n", newline)
				input := prefix + strings.ReplaceAll("<!-- go:doc Config -->\nold\n<!-- /go:doc -->\n", "\n", newline)
				out, renderErr := render([]byte(input), docs)
				require.NoError(t, renderErr)
				require.True(t, strings.HasPrefix(string(out), prefix))
				if newline == "\r\n" {
					require.NotContains(t, strings.ReplaceAll(string(out), "\r\n", ""), "\n")
				}
				again, renderErr := render(out, docs)
				require.NoError(t, renderErr)
				require.Equal(t, out, again)
			}
		})
	}
}

// TestRenderNestedMarkers rejects incomplete nested blocks before any write.
func TestRenderNestedMarkers(t *testing.T) {
	t.Parallel()
	_, err := render([]byte("<!-- go:doc Config -->\n<!-- go:doc Config -->\n<!-- /go:doc -->"), packageDocs{})
	require.ErrorIs(t, err, errMalformedBlocks)
}

// TestWriteAtomicErrors leaves the destination and directory intact when replacement fails.
func TestWriteAtomicErrors(t *testing.T) {
	t.Parallel()
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, writeAtomic(filepath.Join(t.TempDir(), "missing.md"), []byte("new")), os.ErrNotExist)
	})
	t.Run("readonly directory", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" || os.Getuid() == 0 {
			t.Skip("requires Unix directory permissions")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "docs.md")
		require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
		require.NoError(t, os.Chmod(dir, 0o500))
		t.Cleanup(func() { require.NoError(t, os.Chmod(dir, 0o700)) })
		require.ErrorContains(t, writeAtomic(path, []byte("new")), "create temp file:")
		original, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "original", string(original))
	})
}
