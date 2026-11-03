package main

import (
	"flag" //nolint:depguard // Subprocess tests run the real CLI with its own flag set and coverage directory.
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMain lets subprocesses exercise the real CLI, including its exit status.
func TestMain(m *testing.M) {
	if os.Getenv("GENAPIDOCS_CLI_TEST") == "1" {
		if filepath.Base(os.Args[0]) == "go" || filepath.Base(os.Args[0]) == "go.exe" {
			if path := os.Getenv("GENAPIDOCS_BLOCK_WRITE"); path != "" {
				if err := os.Remove(path); err != nil {
					panic(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					panic(err)
				}
			}
			fmt.Fprint(os.Stdout, os.Getenv("GENAPIDOCS_GO_OUTPUT"))
			os.Exit(0)
		}
		main()
		os.Exit(0)
	}
	m.Run()
}

// TestCLI exercises generation and failures through the actual command entry point.
func TestCLI(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		file   string
		json   string
		want   string
		block  bool
	}{
		{name: "success"},
		{name: "go list", want: "go list:"},
		{name: "decode", json: "not JSON", want: "decode go list:"},
		{name: "parse", file: "broken.go", source: "package sample\nvar invalid = )", want: "parse broken.go:"},
		{name: "extract", file: "source.txt", source: "package sample", want: "extract package documentation:"},
		{name: "write", file: "sample.go", source: "package sample\ntype Config struct{}", block: true, want: "write documentation:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := fixture(t)
			path := filepath.Join(dir, "docs.md")
			require.NoError(t, os.WriteFile(path, []byte("<!-- go:doc Config -->\nstale\n<!-- /go:doc -->\n"), 0o600))
			executable, err := os.Executable()
			require.NoError(t, err)
			cmd := exec.Command(executable, "-out", path)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GENAPIDOCS_CLI_TEST=1")
			if cover := flag.Lookup("test.gocoverdir"); cover != nil && cover.Value.String() != "" {
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+cover.Value.String())
			}
			if tc.name == "go list" {
				require.NoError(t, os.Remove(filepath.Join(dir, "go.mod")))
			}
			if tc.json != "" || tc.file != "" {
				bin := t.TempDir()
				name := "go"
				if runtime.GOOS == "windows" {
					name += ".exe"
				}
				binary, readErr := os.ReadFile(executable)
				require.NoError(t, readErr)
				require.NoError(t, os.WriteFile(filepath.Join(bin, name), binary, 0o700))
				output := tc.json
				if tc.file != "" {
					require.NoError(t, os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.source), 0o600))
					output = fmt.Sprintf(`{"Dir":%q,"ImportPath":"example.test/sample","GoFiles":[%q]}`, filepath.ToSlash(dir), tc.file)
				}
				cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GENAPIDOCS_GO_OUTPUT="+output)
				if tc.block {
					cmd.Env = append(cmd.Env, "GENAPIDOCS_BLOCK_WRITE="+path)
				}
			}
			output, err := cmd.CombinedOutput()
			if tc.want == "" {
				require.NoError(t, err, "%s", output)
				generated, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				require.Contains(t, string(generated), "type Config struct")
				return
			}
			var exit *exec.ExitError
			require.ErrorAs(t, err, &exit, "%s", output)
			require.Equal(t, 1, exit.ExitCode())
			require.Contains(t, string(output), tc.want)
		})
	}
}

// TestRenderErrors rejects every block after an error and reports unsupported syntax nodes.
func TestRenderErrors(t *testing.T) {
	t.Parallel()
	docs := packageDocs{fset: token.NewFileSet(), symbols: map[string]symbolDoc{"Unsupported": {node: &ast.Comment{Text: "// unsupported"}}}}
	_, err := render([]byte("<!-- go:doc Missing -->\nold\n<!-- /go:doc -->\n<!-- go:doc Unsupported -->\nold\n<!-- /go:doc -->"), docs)
	require.ErrorIs(t, err, errUnknownSymbol)
	_, err = render([]byte("<!-- go:doc Unsupported -->\nold\n<!-- /go:doc -->"), docs)
	require.ErrorContains(t, err, "format Unsupported: go/printer: unsupported node type")
}
