package fiber

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type stubFileInfo struct {
	name string
}

func (fi stubFileInfo) Name() string { return fi.name }
func (fi stubFileInfo) Size() int64 {
	_ = fi
	return 0
}

func (fi stubFileInfo) Mode() fs.FileMode {
	_ = fi
	return 0
}

func (fi stubFileInfo) ModTime() time.Time {
	_ = fi
	return time.Time{}
}

func (fi stubFileInfo) IsDir() bool {
	_ = fi
	return false
}

func (fi stubFileInfo) Sys() any {
	_ = fi
	return nil
}

type noWriteFile struct {
	closeErr error
}

func (f *noWriteFile) Read(_ []byte) (int, error) {
	_ = f
	return 0, io.EOF
}

func (f *noWriteFile) Stat() (fs.FileInfo, error) {
	_ = f
	return stubFileInfo{name: "probe"}, nil
}
func (f *noWriteFile) Close() error { return f.closeErr }

type writeFile struct {
	writeErr error
	closeErr error
}

func (f *writeFile) Read(_ []byte) (int, error) {
	_ = f
	return 0, io.EOF
}

func (f *writeFile) Stat() (fs.FileInfo, error) {
	_ = f
	return stubFileInfo{name: "probe"}, nil
}
func (f *writeFile) Close() error { return f.closeErr }
func (f *writeFile) Write(_ []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len("fiber"), nil
}

type probeFS struct {
	file    fs.File
	openErr error
}

func (fsys probeFS) Open(_ string) (fs.File, error) {
	if fsys.openErr != nil {
		return nil, fsys.openErr
	}
	if fsys.file == nil {
		return nil, fs.ErrNotExist
	}
	return fsys.file, nil
}

func (fsys probeFS) OpenFile(_ string, _ int, _ fs.FileMode) (fs.File, error) {
	if fsys.openErr != nil {
		return nil, fsys.openErr
	}
	return fsys.file, nil
}

type removeErrFS struct {
	probeFS
	removeErr error
}

func (fsys removeErrFS) Remove(_ string) error {
	return fsys.removeErr
}

type readDirErrFS struct {
	err error
}

type rootFSMissingOpenFile struct{}

func (rootFSMissingOpenFile) Open(_ string) (fs.File, error) { return nil, fs.ErrNotExist }
func (rootFSMissingOpenFile) MkdirAll(_ string, _ fs.FileMode) error {
	return nil
}

type rootFSMissingMkdirAll struct{}

func (rootFSMissingMkdirAll) Open(_ string) (fs.File, error) { return nil, fs.ErrNotExist }
func (rootFSMissingMkdirAll) OpenFile(_ string, _ int, _ fs.FileMode) (fs.File, error) {
	return &writeFile{}, nil
}

type rootFSMkdirErr struct {
	err error
}

func (fsys rootFSMkdirErr) Open(_ string) (fs.File, error) {
	_ = fsys
	return nil, fs.ErrNotExist
}

func (fsys rootFSMkdirErr) OpenFile(_ string, _ int, _ fs.FileMode) (fs.File, error) {
	_ = fsys
	return &writeFile{}, nil
}
func (fsys rootFSMkdirErr) MkdirAll(_ string, _ fs.FileMode) error { return fsys.err }

type rootFSNoWriter struct{}

func (rootFSNoWriter) Open(_ string) (fs.File, error) { return nil, fs.ErrNotExist }
func (rootFSNoWriter) OpenFile(_ string, _ int, _ fs.FileMode) (fs.File, error) {
	return &noWriteFile{}, nil
}
func (rootFSNoWriter) MkdirAll(_ string, _ fs.FileMode) error { return nil }

type probeTempFile struct {
	name     *string
	writeErr error
	closeErr error
}

func (f *probeTempFile) WriteString(_ string) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len("fiber"), nil
}

func (f *probeTempFile) Close() error {
	return f.closeErr
}

func (f *probeTempFile) Name() string {
	if f.name == nil {
		return ""
	}
	return *f.name
}

func stringPtr(value string) *string {
	return &value
}

func (fsys readDirErrFS) Open(_ string) (fs.File, error) {
	_ = fsys
	return nil, fs.ErrNotExist
}

func (fsys readDirErrFS) ReadDir(_ string) ([]fs.DirEntry, error) {
	return nil, fsys.err
}

func TestValidateUploadPath(t *testing.T) {
	t.Parallel()

	absPath := "/var/uploads/file.txt"
	if runtime.GOOS == "windows" {
		absPath = `C:\uploads\file.txt`
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "valid", path: "uploads/file.txt"},
		{name: "valid_dot_prefix", path: ".hidden/file.txt"},
		{name: "dot", path: ".", wantErr: true},
		{name: "empty", path: "", wantErr: true},
		{name: "absolute", path: absPath, wantErr: true},
		{name: "dotdot", path: "../file.txt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := validateUploadPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.path, err)
			}
			if normalized.osPath == "" || normalized.slashPath == "" {
				t.Fatalf("expected normalized paths for %q", tt.path)
			}
			if strings.Contains(normalized.slashPath, `\`) {
				t.Fatalf("expected slash path to use forward slashes, got %q", normalized.slashPath)
			}
		})
	}
}

func TestIsAbsUploadPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "relative", path: "uploads/file.txt", want: false},
		{name: "slash_abs", path: "/uploads/file.txt", want: true},
		{name: "backslash_abs", path: `\uploads\file.txt`, want: true},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			path string
			want bool
		}{name: "volume_abs", path: `C:\uploads\file.txt`, want: true})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isAbsUploadPath(tt.path); got != tt.want {
				t.Fatalf("expected %v for %q, got %v", tt.want, tt.path, got)
			}
		})
	}
}

func TestContainsDotDot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "dotdot_segment", path: "uploads/../file.txt", want: true},
		{name: "dotdot_prefix", path: "../file.txt", want: true},
		{name: "no_dotdot", path: "uploads/.../file.txt", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := containsDotDot(tt.path); got != tt.want {
				t.Fatalf("expected %v for %q, got %v", tt.want, tt.path, got)
			}
		})
	}
}

func TestStorageRootPrefix(t *testing.T) {
	t.Parallel()

	root := "/var/uploads"
	if runtime.GOOS == "windows" {
		root = `C:\uploads`
	}

	tests := []struct {
		name string
		root string
		want string
	}{
		{name: "empty", root: "", want: ""},
		{name: "dot", root: ".", want: ""},
		{name: "rooted", root: root, want: "var/uploads"},
		{name: "relative", root: "uploads", want: "uploads"},
	}

	if runtime.GOOS == "windows" {
		tests[2].want = "uploads"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := storageRootPrefix(tt.root); got != tt.want {
				t.Fatalf("expected %q for %q, got %q", tt.want, tt.root, got)
			}
		})
	}
}

func TestCleanUploadRootPrefix(t *testing.T) {
	t.Parallel()

	abs := "/uploads"
	if runtime.GOOS == "windows" {
		abs = `C:\uploads`
	}

	tests := []struct {
		name    string
		root    string
		want    string
		wantErr bool
	}{
		{name: "empty", root: "", want: ""},
		{name: "dot", root: ".", want: ""},
		{name: "valid", root: "uploads", want: "uploads"},
		{name: "abs", root: abs, wantErr: true},
		{name: "dotdot", root: "../uploads", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := cleanUploadRootPrefix(tt.root)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.root)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.root, err)
			}
			if got != tt.want {
				t.Fatalf("expected %q for %q, got %q", tt.want, tt.root, got)
			}
		})
	}
}

func TestEnsureUploadPathWithinRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rootEval, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("failed to eval root: %v", err)
	}

	inside := filepath.Join(root, "uploads", "file.txt")
	if err := ensureUploadPathWithinRoot(rootEval, inside); err != nil {
		t.Fatalf("expected inside path to be allowed, got %v", err)
	}

	outside := filepath.Join(t.TempDir(), "file.txt")
	if err := ensureUploadPathWithinRoot(rootEval, outside); !errors.Is(err, ErrUploadPathEscapesRoot) {
		t.Fatalf("expected ErrUploadPathEscapesRoot, got %v", err)
	}
}

func TestEvalExistingPath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("path resolution differs on this platform")
	}

	root := t.TempDir()
	target := filepath.Join(root, "missing", "file.txt")
	got, err := evalExistingPath(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("expected %q, got %q", filepath.Clean(root), got)
	}
}

func TestHasPathPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		prefix string
		want   bool
	}{
		{name: "same", path: filepath.Join("a", "b"), prefix: filepath.Join("a", "b"), want: true},
		{name: "child", path: filepath.Join("a", "b", "c"), prefix: filepath.Join("a", "b"), want: true},
		{name: "sibling", path: filepath.Join("a", "bc"), prefix: filepath.Join("a", "b"), want: false},
		{name: "prefix_with_separator", path: filepath.Join("a", "b", "c"), prefix: filepath.Join("a", "b") + string(os.PathSeparator), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hasPathPrefix(tt.path, tt.prefix); got != tt.want {
				t.Fatalf("expected %v for %q/%q, got %v", tt.want, tt.path, tt.prefix, got)
			}
		})
	}
}

func TestEnsureNoSymlinkFS(t *testing.T) {
	t.Parallel()

	t.Run("missing_parent", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{}
		if err := ensureNoSymlinkFS(fsys, "missing/file.txt"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("symlink_detected", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"dir":      &fstest.MapFile{Mode: fs.ModeDir},
			"dir/link": &fstest.MapFile{Mode: fs.ModeSymlink},
		}
		if err := ensureNoSymlinkFS(fsys, "dir/link/file.txt"); !errors.Is(err, ErrUploadPathEscapesRoot) {
			t.Fatalf("expected ErrUploadPathEscapesRoot, got %v", err)
		}
	})

	t.Run("read_dir_error", func(t *testing.T) {
		t.Parallel()

		fsys := readDirErrFS{err: errors.New("read failure")}
		if err := ensureNoSymlinkFS(fsys, "dir/file.txt"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no_symlink", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"dir":       &fstest.MapFile{Mode: fs.ModeDir},
			"dir/child": &fstest.MapFile{Mode: fs.ModeDir},
		}
		if err := ensureNoSymlinkFS(fsys, "dir/child/file.txt"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("single_part_path", func(t *testing.T) {
		t.Parallel()

		if err := ensureNoSymlinkFS(fstest.MapFS{}, "file.txt"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("leading_slash", func(t *testing.T) {
		t.Parallel()

		if err := ensureNoSymlinkFS(fstest.MapFS{}, "/file.txt"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestProbeUploadDirWritable(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("probe behavior differs on Windows")
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		if err := probeUploadDirWritable(t.TempDir()); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("not_directory", func(t *testing.T) {
		t.Parallel()

		file, err := os.CreateTemp(t.TempDir(), "probe")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		t.Cleanup(func() {
			if removeErr := os.Remove(file.Name()); removeErr != nil {
				t.Fatalf("failed to remove temp file: %v", removeErr)
			}
		})
		if err := probeUploadDirWritable(file.Name()); err == nil {
			t.Fatal("expected error for file path")
		}
	})

	t.Run("create_error", func(t *testing.T) {
		t.Parallel()

		createErr := errors.New("create failure")
		err := probeUploadDirWritableWith(t.TempDir(), func(_, _ string) (tempProbeFile, error) {
			return nil, createErr
		}, func(string) error { return nil })
		if !errors.Is(err, createErr) {
			t.Fatalf("expected create error, got %v", err)
		}
	})

	t.Run("write_error", func(t *testing.T) {
		t.Parallel()

		writeErr := errors.New("write failure")
		err := probeUploadDirWritableWith(t.TempDir(), func(_, _ string) (tempProbeFile, error) {
			return &probeTempFile{name: stringPtr("probe"), writeErr: writeErr}, nil
		}, func(string) error { return nil })
		if !errors.Is(err, writeErr) {
			t.Fatalf("expected write error, got %v", err)
		}
	})

	t.Run("close_error_after_write", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("close failure")
		err := probeUploadDirWritableWith(t.TempDir(), func(_, _ string) (tempProbeFile, error) {
			return &probeTempFile{name: stringPtr("probe"), closeErr: closeErr}, nil
		}, func(string) error { return nil })
		if !errors.Is(err, closeErr) {
			t.Fatalf("expected close error, got %v", err)
		}
	})

	t.Run("remove_error_after_write_failure", func(t *testing.T) {
		t.Parallel()

		removeErr := errors.New("remove failure")
		err := probeUploadDirWritableWith(t.TempDir(), func(_, _ string) (tempProbeFile, error) {
			return &probeTempFile{name: stringPtr("probe"), writeErr: errors.New("write failure")}, nil
		}, func(string) error { return removeErr })
		if !errors.Is(err, removeErr) {
			t.Fatalf("expected remove error, got %v", err)
		}
	})

	t.Run("remove_error_after_close_failure", func(t *testing.T) {
		t.Parallel()

		removeErr := errors.New("remove failure")
		err := probeUploadDirWritableWith(t.TempDir(), func(_, _ string) (tempProbeFile, error) {
			return &probeTempFile{name: stringPtr("probe"), closeErr: errors.New("close failure")}, nil
		}, func(string) error { return removeErr })
		if !errors.Is(err, removeErr) {
			t.Fatalf("expected remove error, got %v", err)
		}
	})

	t.Run("remove_error_after_success", func(t *testing.T) {
		t.Parallel()

		removeErr := errors.New("remove failure")
		err := probeUploadDirWritableWith(t.TempDir(), func(_, _ string) (tempProbeFile, error) {
			return &probeTempFile{name: stringPtr("probe")}, nil
		}, func(string) error { return removeErr })
		if !errors.Is(err, removeErr) {
			t.Fatalf("expected remove error, got %v", err)
		}
	})
}

func TestConfigureUploads(t *testing.T) {
	t.Parallel()

	t.Run("root_dir", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		app := New(Config{RootDir: root})
		if app.config.uploadRootDir == "" {
			t.Fatal("expected uploadRootDir to be set")
		}
		if app.config.uploadRootPath == "" {
			t.Fatal("expected uploadRootPath to be set")
		}
	})

	t.Run("missing_openfile", func(t *testing.T) {
		t.Parallel()

		assertPanics(t, func() {
			New(Config{RootDir: "uploads", RootFs: rootFSMissingOpenFile{}})
		})
	})

	t.Run("missing_mkdirall", func(t *testing.T) {
		t.Parallel()

		assertPanics(t, func() {
			New(Config{RootDir: "uploads", RootFs: rootFSMissingMkdirAll{}})
		})
	})

	t.Run("invalid_rootdir_for_rootfs", func(t *testing.T) {
		t.Parallel()

		assertPanics(t, func() {
			New(Config{RootDir: filepath.Join(t.TempDir(), "uploads"), RootFs: rootFSNoWriter{}})
		})
	})

	t.Run("mkdir_error", func(t *testing.T) {
		t.Parallel()

		assertPanics(t, func() {
			New(Config{
				RootDir: "uploads",
				RootFs:  rootFSMkdirErr{err: errors.New("mkdir failure")},
			})
		})
	})

	t.Run("not_writable", func(t *testing.T) {
		t.Parallel()

		assertPanics(t, func() {
			New(Config{RootDir: "uploads", RootFs: rootFSNoWriter{}})
		})
	})
}

func TestEvalExistingPathError(t *testing.T) {
	t.Parallel()

	_, err := evalExistingPath("invalid\x00path")
	if err == nil {
		t.Fatal("expected error")
	}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestProbeUploadFSWritable(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		fsys := probeFS{file: &writeFile{}}
		if err := probeUploadFSWritable(fsys, ""); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("open_error", func(t *testing.T) {
		t.Parallel()

		fsys := probeFS{openErr: errors.New("open failure")}
		if err := probeUploadFSWritable(fsys, ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not_writable", func(t *testing.T) {
		t.Parallel()

		fsys := probeFS{file: &noWriteFile{}}
		if err := probeUploadFSWritable(fsys, ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("write_error", func(t *testing.T) {
		t.Parallel()

		fsys := probeFS{file: &writeFile{writeErr: errors.New("write failure")}}
		if err := probeUploadFSWritable(fsys, ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("close_error", func(t *testing.T) {
		t.Parallel()

		fsys := probeFS{file: &writeFile{closeErr: errors.New("close failure")}}
		if err := probeUploadFSWritable(fsys, ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("remove_error", func(t *testing.T) {
		t.Parallel()

		fsys := removeErrFS{
			probeFS:   probeFS{file: &writeFile{}},
			removeErr: errors.New("remove failure"),
		}
		if err := probeUploadFSWritable(fsys, ""); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestStorageUploadPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		path   string
		want   string
	}{
		{name: "empty_prefix", prefix: "", path: "uploads/file.txt", want: "uploads/file.txt"},
		{name: "with_prefix", prefix: "root", path: "uploads/file.txt", want: "root/uploads/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := storageUploadPath(tt.prefix, tt.path); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
