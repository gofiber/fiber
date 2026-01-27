package fiber

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type testUploadFS struct {
	mkdirPath string
	mkdirPerm fs.FileMode
}

func (tfs *testUploadFS) Open(_ string) (fs.File, error) {
	_ = tfs
	return nil, fs.ErrNotExist
}

func (tfs *testUploadFS) OpenFile(_ string, _ int, _ fs.FileMode) (fs.File, error) {
	_ = tfs
	return &testUploadFile{buf: &bytes.Buffer{}}, nil
}

func (tfs *testUploadFS) MkdirAll(path string, perm fs.FileMode) error {
	tfs.mkdirPath = path
	tfs.mkdirPerm = perm
	return nil
}

func (tfs *testUploadFS) Remove(_ string) error {
	_ = tfs
	return nil
}

type testUploadFile struct {
	buf *bytes.Buffer
}

func (tf *testUploadFile) Read(p []byte) (int, error) {
	//nolint:wrapcheck // test helper passthrough
	return tf.buf.Read(p)
}

func (tf *testUploadFile) Write(p []byte) (int, error) {
	//nolint:wrapcheck // test helper passthrough
	return tf.buf.Write(p)
}

func (tf *testUploadFile) Close() error {
	_ = tf
	return nil
}

func (tf *testUploadFile) Stat() (fs.FileInfo, error) {
	_ = tf
	return testUploadFileInfo{name: "upload"}, nil
}

type testUploadFileInfo struct {
	name string
}

func (fi testUploadFileInfo) Name() string { return fi.name }
func (fi testUploadFileInfo) Size() int64 {
	_ = fi
	return 0
}

func (fi testUploadFileInfo) Mode() fs.FileMode {
	_ = fi
	return 0
}

func (fi testUploadFileInfo) ModTime() time.Time {
	_ = fi
	return time.Time{}
}

func (fi testUploadFileInfo) IsDir() bool {
	_ = fi
	return false
}

func (fi testUploadFileInfo) Sys() any {
	_ = fi
	return nil
}

func TestRootPermsRootFs(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("root perms are not validated on Windows in this test")
	}

	tests := []struct {
		name     string
		rootPerm fs.FileMode
		wantPerm fs.FileMode
	}{
		{
			name:     "default",
			rootPerm: 0,
			wantPerm: 0o750,
		},
		{
			name:     "custom",
			rootPerm: 0o700,
			wantPerm: 0o700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tfs := &testUploadFS{}
			New(Config{
				RootDir:   "uploads",
				RootFs:    tfs,
				RootPerms: tt.rootPerm,
			})

			if tfs.mkdirPath != "uploads" {
				t.Fatalf("expected RootFs prefix %q, got %q", "uploads", tfs.mkdirPath)
			}
			if tfs.mkdirPerm != tt.wantPerm {
				t.Fatalf("expected RootPerms %o, got %o", tt.wantPerm, tfs.mkdirPerm)
			}
		})
	}
}

func TestValidateUploadPathPreservesLeadingDot(t *testing.T) {
	t.Parallel()

	path := filepath.Join(".hidden", "file.txt")

	normalized, err := validateUploadPath(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.HasPrefix(normalized.osPath, ".") {
		t.Fatalf("expected os path %q to preserve leading dot", normalized.osPath)
	}
	if normalized.slashPath != ".hidden/file.txt" {
		t.Fatalf("expected slash path %q, got %q", ".hidden/file.txt", normalized.slashPath)
	}
}
