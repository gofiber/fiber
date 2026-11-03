package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// fixture provides exported APIs, an executable example and an excluded source file.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/sample\n\ngo 1.26.0\n",
		"sample.go": `package sample

// Config contains public settings.
type Config struct {
	Value int
	secret string
}

// DefaultConfig defines the default settings.
var DefaultConfig = Config{Value: 3}

// NewConfig returns a configuration.
func NewConfig(value int) Config { return Config{Value: value} }

// Box holds a value.
type Box[T any] struct { Item T }

// Value returns the item.
func (b Box[T]) Value() T { return b.Item }

// Double doubles a number.
func Double(n int) int { return n * 2 }
`,
		"example_test.go": `package sample_test

import (
	"fmt"
	"example.test/sample"
)

func ExampleDouble() {
	fmt.Println(sample.Double(2))
	// Output: 4
}

func ExampleDouble_unplayable() {
	fmt.Println(sample.Double(value))
	// Output: 8
}
`,
		"support_test.go": "package sample_test\n\nvar value = 4\n",
		"excluded.go":     "//go:build ignore\n\npackage sample\n\nvar invalid = )\n",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}

// TestGenerate verifies real declarations, runnable examples, preserved prose and repeatability.
func TestGenerate(t *testing.T) {
	t.Parallel()
	dir := fixture(t)
	path := filepath.Join(dir, "docs.md")
	input := "Before.\n\n<!-- go:doc Config DefaultConfig NewConfig Box.Value -->\nstale API\n<!-- /go:doc -->\n\n" +
		"Example.\n\n<!-- go:doc ExampleDouble -->\nstale example\n<!-- /go:doc -->\n\nAfter.\n"
	require.NoError(t, os.WriteFile(path, []byte(input), 0o600))
	require.NoError(t, generate(dir, path))
	result, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(result), "Before.")
	require.Contains(t, string(result), "After.")
	require.Contains(t, string(result), "// Config contains public settings.")
	require.Contains(t, string(result), "Value int")
	require.Contains(t, string(result), "var DefaultConfig = Config{Value: 3}")
	require.Contains(t, string(result), "func NewConfig(value int) Config")
	require.Contains(t, string(result), "func (b Box[T]) Value() T")
	require.NotContains(t, string(result), "secret")
	require.NotContains(t, string(result), "return Config")
	require.NotContains(t, string(result), "stale")
	examples := regexp.MustCompile("(?s)<!-- go:doc ExampleDouble -->\\n\\n```go\\n(.*?)\\n```").FindSubmatch(result)
	require.Len(t, examples, 2)
	require.Contains(t, string(result), "```text\n4\n```")
	program := filepath.Join(dir, "cmd", "example", "main.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(program), 0o700))
	require.NoError(t, os.WriteFile(program, examples[1], 0o600))
	cmd := exec.Command("go", "run", "./cmd/example")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.Equal(t, "4\n", string(output))
	require.NoError(t, generate(dir, path))
	again, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, result, again)
}

// TestGenerateErrors ensures invalid markers never replace the user's documentation.
func TestGenerateErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		want     error
		name     string
		markdown string
	}{
		{name: "no markers", markdown: "Handwritten documentation.", want: errNoBlocks},
		{name: "missing end", markdown: "<!-- go:doc Config -->\nold\n", want: errMalformedBlocks},
		{name: "empty names", markdown: "<!-- go:doc   -->\nold\n<!-- /go:doc -->", want: errMalformedBlocks},
		{name: "extra end", markdown: "<!-- go:doc Config -->\nold\n<!-- /go:doc -->\n<!-- /go:doc -->", want: errMalformedBlocks},
		{name: "unknown symbol", markdown: "<!-- go:doc Missing -->\nold\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "valid then invalid", markdown: "<!-- go:doc Config -->\nfirst\n<!-- /go:doc -->\n<!-- go:doc Missing -->\nsecond\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "private symbol", markdown: "<!-- go:doc secret -->\nold\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "nonplayable example", markdown: "<!-- go:doc ExampleDouble_unplayable -->\nold\n<!-- /go:doc -->", want: errUnplayableExample},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := fixture(t)
			path := filepath.Join(dir, "docs.md")
			require.NoError(t, os.WriteFile(path, []byte(tc.markdown), 0o600))
			require.ErrorIs(t, generate(dir, path), tc.want)
			unchanged, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, tc.markdown, string(unchanged))
		})
	}
}

// TestGenerateMissingOutput checks required and nonexistent output paths.
func TestGenerateMissingOutput(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, generate(t.TempDir(), ""), errOutputRequired)
	require.ErrorIs(t, generate(t.TempDir(), filepath.Join(t.TempDir(), "missing.md")), os.ErrNotExist)
}

// TestRetryDocumentationCurrent prevents published API blocks from drifting from Go sources.
func TestRetryDocumentationCurrent(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	docs, err := loadPackage(filepath.Join(root, "addon", "retry"))
	require.NoError(t, err)
	markdown, err := os.ReadFile(filepath.Join(root, "docs", "addon", "retry.md"))
	require.NoError(t, err)
	rendered, err := render(markdown, docs)
	require.NoError(t, err)
	require.Equal(t, string(markdown), string(rendered), "run make generate to update the retry documentation")
}
