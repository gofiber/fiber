package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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
var DefaultConfig = Config{
	// Value is three.
	Value: 3,
	secret: "x", // hidden note
}

// Kind enumerates kinds.
type Kind int

const (
	KindA Kind = iota
	KindB
)

var hiddenVar = 1

type hiddenType struct{}

func hidden() {}

func (Config) hiddenMethod() {}

// NewConfig returns a configuration.
func NewConfig(value int) Config { return Config{Value: value} }

// Box holds a value.
type Box[T any] struct { Item T }

// Value returns the item.
func (b Box[T]) Value() T { return b.Item }

// Double doubles a number:
//
//	Double(2)
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

func ExampleKind() {
	fmt.Println(sample.KindB)
	fmt.Println(sample.KindA)
	// Unordered output:
	// 0
	// 1
}
`,
		"support_test.go": "package sample_test\n\nvar value = 4\n",
		// One example plus another declaration makes go/doc publish the whole file.
		"whole_test.go": `package sample_test

import (
	"fmt"
	"example.test/sample"
)

func show(b sample.Box[int]) { fmt.Println(len("ab") + b.Value()) }

func ExampleBox() {
	show(sample.Box[int]{Item: 1})
	// Output: 3
}
`,
		"foreign_test.go": `package sample_test

import (
	"fmt"
	"example.test/sample"
)

type helper struct{}

func ExampleNewConfig() {
	fmt.Println(sample.NewConfig(value).Value)
	// Output: 4
}
`,
		"excluded.go": "//go:build ignore\n\npackage sample\n\nvar invalid = )\n",
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
		"Example.\n\n<!-- go:doc ExampleDouble -->\nstale example\n<!-- /go:doc -->\n\n" +
		"<!-- go:doc Double KindA KindB -->\n<!-- /go:doc -->\n\n<!-- go:doc ExampleKind ExampleBox -->\n<!-- /go:doc -->\n\nAfter.\n"
	require.NoError(t, os.WriteFile(path, []byte(input), 0o600))
	require.NoError(t, generate(dir, path))
	result, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(result), "Before.")
	require.Contains(t, string(result), "After.")
	require.Contains(t, string(result), "// Config contains public settings.")
	require.Contains(t, string(result), "Value int")
	require.Contains(t, string(result), "var DefaultConfig = Config{\n    // Value is three.\n    Value: 3,\n}")
	require.NotContains(t, string(result), "hidden")
	require.NotContains(t, string(result), "\t")
	require.Contains(t, string(result), "// Double doubles a number:\n//\n//     Double(2)\nfunc Double(n int) int")
	require.Equal(t, 1, strings.Count(string(result), "KindA Kind = iota"))
	require.Contains(t, string(result), "Unordered output:\n\n```text\n0\n1\n```")
	require.Contains(t, string(result), "func show(b sample.Box[int])")
	require.NotContains(t, string(result), "func Example")
	require.NotContains(t, string(result), "import ()")
	require.Contains(t, string(result), "func NewConfig(value int) Config")
	require.Contains(t, string(result), "func (b Box[T]) Value() T")
	require.NotContains(t, string(result), "secret")
	require.NotContains(t, string(result), "return Config")
	require.NotContains(t, string(result), "stale")
	examples := regexp.MustCompile("(?s)<!-- go:doc ExampleDouble -->\\n\\n```go\\n(.*?)\\n```").FindSubmatch(result)
	require.Len(t, examples, 2)
	require.Contains(t, string(result), "```text\n4\n```")
	whole := regexp.MustCompile("(?s)<!-- go:doc ExampleKind ExampleBox -->.*?```go\n.*?```.*?```go\n(.*?)\n```").FindSubmatch(result)
	require.Len(t, whole, 2)
	for want, source := range map[string][]byte{"4\n": examples[1], "3\n": whole[1]} {
		program := filepath.Join(dir, "cmd", "example"+want[:1], "main.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(program), 0o700))
		require.NoError(t, os.WriteFile(program, source, 0o600))
		cmd := exec.Command("go", "run", "./cmd/example"+want[:1])
		cmd.Dir = dir
		output, runErr := cmd.CombinedOutput()
		require.NoError(t, runErr, "%s", output)
		require.Equal(t, want, string(output))
	}
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
		{name: "private field", markdown: "<!-- go:doc secret -->\nold\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "private func", markdown: "<!-- go:doc hidden -->\nold\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "private var", markdown: "<!-- go:doc hiddenVar -->\nold\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "private type", markdown: "<!-- go:doc hiddenType -->\nold\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "private method", markdown: "<!-- go:doc Config.hiddenMethod -->\nold\n<!-- /go:doc -->", want: errUnknownSymbol},
		{name: "nonplayable example", markdown: "<!-- go:doc ExampleDouble_unplayable -->\nold\n<!-- /go:doc -->", want: errUnplayableExample},
		{name: "nonplayable whole file", markdown: "<!-- go:doc ExampleNewConfig -->\nold\n<!-- /go:doc -->", want: errUnplayableExample},
		{name: "adjacent then missing end", markdown: "<!-- go:doc Config -->\n<!-- /go:doc -->\n<!-- go:doc Config -->\n", want: errMalformedBlocks},
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

// TestGenerateCgo includes declarations from cgo files selected by the Go build context.
func TestGenerateCgo(t *testing.T) {
	t.Parallel()
	dir := fixture(t)
	cmd := exec.Command("go", "env", "CGO_ENABLED")
	cmd.Dir = dir
	enabled, err := cmd.Output()
	require.NoError(t, err)
	if strings.TrimSpace(string(enabled)) != "1" {
		t.Skip("cgo files are excluded in this build context")
	}
	source := "package sample\n\nimport \"C\"\n\n// CgoConfig configures a cgo-backed API.\ntype CgoConfig struct { Enabled bool }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cgo.go"), []byte(source), 0o600))
	path := filepath.Join(dir, "docs.md")
	require.NoError(t, os.WriteFile(path, []byte("<!-- go:doc CgoConfig -->\nstale\n<!-- /go:doc -->\n"), 0o600))
	require.NoError(t, generate(dir, path))
	result, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(result), "type CgoConfig struct")
	require.NotContains(t, string(result), "stale")
}
