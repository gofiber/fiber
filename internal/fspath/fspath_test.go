package fspath

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func Benchmark_Valid(b *testing.B) {
	bench := func(name string, filesystem fs.FS, path string) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			// the handler copies the routed path into a buffer that is checked in place
			buf := make([]byte, 0, 64)
			for b.Loop() {
				buf = append(buf[:0], path...)
				if !Valid(buf, filesystem, true) {
					b.Fatal("invalid")
				}
			}
		})
	}

	bench("nilFS - decoded chars", nil, "/foo/bar/baz qux/index.html")
	bench("dirFS - decoded chars", os.DirFS("."), "/foo/bar/baz qux/index.html")
	bench("nilFS - escapes", nil, "/foo/bar/baz%20qux/photo%402x.png")
}

func Test_Valid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		filesystem fs.FS
		name       string
		input      string
	}

	testCases := []testCase{
		{name: "simple path", input: "/foo/bar.txt"},
		{name: "root", input: "/"},
		{name: "empty", input: ""},
		{name: "decoded space", input: "/foo bar/baz.txt"},
		{name: "plus literal", input: "/foo+bar/baz.txt"},
		{name: "dots in names", input: "/.well-known/..x/y..z/a.b"},
		{name: "colon past the first segment", input: "/a/C:/b"},
		{name: "utf-8", input: "/caf\u00e9/\u65e5\u672c.txt"},
		{name: "invalid utf-8 on the OS root", input: "/a\xffb.txt"},
		// escapes the router left encoded stay encoded for consistent authorization
		{name: "encoded space", input: "/foo%20bar/baz.txt"},
		{name: "encoded reserved character", input: "/photo%402x.png"},
		{name: "lowercase hex digits", input: "/a%7bb%7d.txt"},
		{name: "encoded percent sign", input: "/100%25.txt"},
		{name: "encoded utf-8", input: "/caf%C3%A9.txt"},
		{name: "encoded byte past DEL", input: "/a%80b.txt"},
		{name: "percent sign stays encoded", input: "/%2570rivate/secret.txt"},
		{name: "double encoded traversal stays a name", input: "/%252e%252e/bar.txt"},
		{name: "double encoded slash stays a name", input: "/..%252F..%252Fetc/passwd"},
		{name: "double encoded backslash stays a name", input: "/..%255C..%255Cwin.ini"},
		{name: "double encoded null stays a name", input: "/a%2500.txt"},
		{name: "trailing slash preserved", input: "/foo/bar/"},
		{name: "trailing slash after an escape", input: "/foo%20bar/"},
		{filesystem: os.DirFS("."), name: "filesystem root", input: "/"},
		{filesystem: os.DirFS("."), name: "filesystem empty", input: ""},
		{filesystem: os.DirFS("."), name: "filesystem encoded space", input: "/foo%20bar/baz.txt"},
		{filesystem: os.DirFS("."), name: "filesystem trailing slash", input: "/foo/"},
		{filesystem: fstest.MapFS{}, name: "filesystem drive letter is a name", input: "/C:/x"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.True(t, Valid([]byte(tc.input), tc.filesystem, true), "input=%q", tc.input)
		})
	}
}

// Test_Valid_NoDecode covers UnescapePath, where a "%" left in the routed
// path is literal and must not be decoded again.
func Test_Valid_NoDecode(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"/%70rivate/secret.txt",
		"/100%.txt",
		"/100%",
		"/a%zzb.txt",
		"/foo%2Fbar.txt",
		"/foo%5Cbar.txt",
		"/foo%00bar.txt",
		"/%2e%2e/bar.txt",
		"/foo bar/baz.txt",
	} {
		require.True(t, Valid([]byte(input), nil, false), "input=%q", input)
	}

	for _, input := range []string{
		"/foo\\bar.txt", "/foo/bar.txt\x00", "/foo\x1fbar.txt", "/foo\x7fbar.txt",
		"/foo//bar.txt", "/foo/../bar.txt", "/./bar.txt", "/C:/x", "//server/share/x",
	} {
		require.False(t, Valid([]byte(input), nil, false), "input=%q", input)
	}
}

func Test_Valid_Error(t *testing.T) {
	t.Parallel()

	type testCase struct {
		filesystem fs.FS
		name       string
		input      string
	}

	testCases := []testCase{
		// the router removes dot segments and keeps empty ones, so neither names a matched file
		{name: "parent segment", input: "/foo/../bar.txt"},
		{name: "leading parent segment", input: "/../bar.txt"},
		{name: "bare parent segment", input: ".."},
		{name: "current segment", input: "/foo/./bar.txt"},
		{name: "trailing parent segment", input: "/foo/.."},
		{name: "trailing current segment", input: "/foo/."},
		{name: "encoded parent segment", input: "/foo/%2e%2e/bar.txt"},
		{name: "mixed encoded parent segment", input: "/foo/.%2E/bar.txt"},
		{name: "empty segment", input: "/foo//bar.txt"},
		{name: "empty segment before the trailing slash", input: "/foo//"},
		{filesystem: os.DirFS("."), name: "filesystem parent segment", input: "/foo/../bar.txt"},
		{filesystem: os.DirFS("."), name: "filesystem empty segment", input: "/foo//bar.txt"},
		// a malformed escape is not a valid spelling of any name
		{name: "malformed escape", input: "/a%zzb.txt"},
		{name: "half malformed escape", input: "/a%2zb.txt"},
		{name: "truncated escape", input: "/foo%2"},
		{name: "trailing percent", input: "/foo%"},
		{name: "stray percent", input: "/100%.txt"},
		{name: "stray percent before an escape", input: "/%%2570rivate"},
		// the router decodes every escape of an unreserved character, so one that
		// reaches here was forged from a stray '%' and must not be decoded again
		{name: "forged unreserved escape", input: "/%70rivate/secret.txt"},
		{name: "forged unreserved tilde", input: "/%7Efile.txt"},
		{name: "forged dot escape", input: "/%2egit/config"},
		{name: "forged digit escape", input: "/a%30.txt"},
		// a decoded slash, a backslash or a control character cannot be part of a name
		{name: "null byte", input: "/foo/bar.txt\x00"},
		{name: "null byte mid name", input: "/foo\x00.txt/bar"},
		{name: "encoded null byte", input: "/foo/bar.txt%00"},
		{name: "control character", input: "/foo\x1fbar.txt"},
		{name: "tab", input: "/foo\tbar.txt"},
		{name: "DEL", input: "/foo\x7fbar.txt"},
		{name: "encoded control character", input: "/foo%0Abar.txt"},
		{name: "encoded DEL", input: "/foo%7Fbar.txt"},
		{name: "encoded slash", input: "/foo%2Fbar.txt"},
		{name: "encoded lowercase slash", input: "/foo%2fbar.txt"},
		{name: "encoded trailing slash", input: "/foo/bar%2F"},
		{name: "encoded slash traversal", input: "/..%2F..%2Fetc/passwd"},
		{name: "encoded backslash", input: "/foo%5C..%5Cbar.txt"},
		{name: "encoded lowercase backslash", input: "/foo%5c..%5cbar.txt"},
		{name: "backslash beside an escape", input: "/foo%20bar\\baz.txt"},
		{name: "control character beside an escape", input: "/foo%20bar\x00"},
		{name: "backslash", input: "/foo\\bar.txt"},
		{name: "backslash traversal", input: "/..\\private/secret.txt"},
		{name: "backslash path", input: "\\foo\\bar.txt"},
		{name: "backslash absolute", input: "/\\Windows\\System32\\drivers\\etc\\hosts"},
		{name: "backslash unc path", input: "\\\\server\\share\\secret.txt"},
		{name: "backslash drive letter", input: "/C:\\Windows\\System32\\cmd.exe"},
		{name: "double slash path", input: "//foo//bar.txt"},
		{name: "triple slash path", input: "///server/share/secret.txt"},
		{name: "drive letter", input: "C:/Windows/System32/cmd.exe"},
		{name: "lowercase drive letter", input: "/c:/Windows/win.ini"},
		{name: "drive letter with leading slash", input: "/C:/Windows/System32/cmd.exe"},
		{name: "bare drive letter", input: "/C:"},
		{name: "drive relative", input: "/C:secret.txt"},
		{name: "unc path", input: "//server/share/secret.txt"},
		{name: "device path", input: "//?/C:/secret.txt"},
		{filesystem: os.DirFS("."), name: "filesystem backslash", input: "/foo\\bar.txt"},
		{filesystem: os.DirFS("."), name: "filesystem encoded slash", input: "/foo%2Fbar.txt"},
		{filesystem: fstest.MapFS{}, name: "filesystem invalid utf-8", input: "/a\xffb.txt"},
		{filesystem: fstest.MapFS{}, name: "filesystem unc path", input: "//server/share"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.False(t, Valid([]byte(tc.input), tc.filesystem, true), "input=%q", tc.input)
		})
	}
}

func Test_HasUnsafeSegment(t *testing.T) {
	t.Parallel()

	for _, p := range []string{"/../x", "/a/../x", "/a/..", "/./x", "/a/.", "/a//b", "//", "/a//", "//a", "..", "."} {
		require.True(t, hasUnsafeSegment([]byte(p)), "path=%q", p)
	}
	for _, p := range []string{"", "/", "/a", "/a/", "/a/b", "/.a/..b/c./d..", "/.../a", "/%2e%2e/a", "a/b"} {
		require.False(t, hasUnsafeSegment([]byte(p)), "path=%q", p)
	}
}

func Test_HasDriveLetter(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]bool{
		"":      false,
		"C":     false,
		"C:":    true,
		"c:/x":  true,
		"Z:x":   true,
		"1:/x":  false,
		"@:/x":  false,
		"CC:/x": false,
		"/C:/x": false,
	} {
		require.Equal(t, want, hasDriveLetter(input), "input=%q", input)
	}
}

func Test_HasTrailingDotOrSpaceSegment(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]bool{
		"/":              false,
		"/foo/bar.txt":   false,
		"/.well-known/x": false,
		"/..x/y":         false,
		"/a.b/c":         false,
		"/foo/":          false,
		"/foo./bar":      true,
		"/foo /bar":      true,
		"/foo.":          true,
		"/foo ":          true,
		"/foo./":         true,
		"/foo/bar. ":     true,
	} {
		require.Equal(t, want, hasTrailingDotOrSpaceSegment([]byte(input)), "input=%q", input)
	}
}

// Test_Valid_TrailingDotOrSpace pins that a segment ending in a dot or a
// space names an ordinary file everywhere except Windows, which strips both
// when it opens a file and would turn "private." into the guarded "private".
func Test_Valid_TrailingDotOrSpace(t *testing.T) {
	t.Parallel()

	for _, filesystem := range []fs.FS{nil, fstest.MapFS{}} {
		for _, input := range []string{"/private./secret.txt", "/private /secret.txt", "/y.../z.", "/foo. /"} {
			require.Equal(t, runtime.GOOS != windowsOS, Valid([]byte(input), filesystem, true), "input=%q", input)
		}
	}
}

// Test_Valid_InPlace pins that Valid neither changes p nor keeps it.
func Test_Valid_InPlace(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"/foo%20bar/baz.txt", "/foo/../bar.txt", "/%70rivate"} {
		p := []byte(input)
		Valid(p, nil, true)
		require.Equal(t, input, string(p))
	}
}

func Test_Unhex(t *testing.T) {
	t.Parallel()

	for c := range 256 {
		want := -1
		switch {
		case c >= '0' && c <= '9':
			want = c - '0'
		case c >= 'a' && c <= 'f':
			want = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			want = c - 'A' + 10
		}
		require.Equal(t, want, unhex(byte(c)), "c=%q", c)
	}
}

func Test_FSName(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"":            "",
		"/":           "",
		".":           "",
		"index.html":  "index.html",
		"/index.html": "index.html",
		"//a//b.txt":  "a/b.txt",
		"a/./b.txt":   "a/b.txt",
		"a/../b.txt":  "b.txt",
		"dir/":        "dir",
		// a request path never climbs above the root, so neither does the name
		"../b.txt":      "b.txt",
		"a/../../b.txt": "b.txt",
		"/../../etc":    "etc",
		// escapes are names here: SendFile escapes them before fasthttp decodes once
		"100%25.txt": "100%25.txt",
		"a%2F..":     "a%2F..",
	} {
		got := FSName(input)
		require.Equal(t, want, got, "input=%q", input)
		require.True(t, got == "" || fs.ValidPath(got), "input=%q", input)
	}
}

func Test_OSName(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"/srv/www/index.html":    "/srv/www/index.html",
		"/srv/www/../index.html": "/srv/index.html",
		"/srv//www/./a.txt":      "/srv/www/a.txt",
		"/srv/www/dir/":          "/srv/www/dir",
		"/../../etc/passwd":      "/etc/passwd",
	} {
		require.Equal(t, filepath.FromSlash(want), OSName(input), "input=%q", input)
	}
}

// Test_OSName_Windows covers the volume a Windows SendFile path keeps.
func Test_OSName_Windows(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != windowsOS {
		t.Skip("volume names exist only on Windows")
	}

	for input, want := range map[string]string{
		"C:/srv/../index.html":    `C:\index.html`,
		"C:/../../index.html":     `C:\index.html`,
		"//server/share/../a.txt": `\\server\share\a.txt`,
	} {
		require.Equal(t, want, OSName(input), "input=%q", input)
	}
}
