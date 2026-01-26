package fiber

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
)

type uploadPath struct {
	osPath    string
	slashPath string
}

func (app *App) configureUploads() {
	if app.config.RootFs != nil {
		writer, ok := app.config.RootFs.(interface {
			fs.FS
			OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error)
		})
		if !ok {
			panic("fiber: RootFs must implement OpenFile for uploads")
		}
		mkdirer, ok := app.config.RootFs.(interface {
			MkdirAll(path string, perm fs.FileMode) error
		})
		if !ok {
			panic("fiber: RootFs must implement MkdirAll for uploads")
		}

		prefix, err := cleanUploadRootPrefix(app.config.RootDir)
		if err != nil {
			panic(fmt.Sprintf("fiber: invalid RootDir for RootFs: %v", err))
		}

		if prefix != "" {
			if err := mkdirer.MkdirAll(prefix, app.config.RootPerms); err != nil {
				panic(fmt.Sprintf("fiber: failed to create RootFs prefix %q: %v", prefix, err))
			}
		}

		if err := probeUploadFSWritable(writer, prefix); err != nil {
			panic(fmt.Sprintf("fiber: RootFs not writable: %v", err))
		}

		app.config.uploadRootFSPrefix = prefix
		app.config.uploadRootFSWriter = writer
		app.config.uploadRootPath = prefix
		return
	}

	if app.config.RootDir == "" {
		return
	}

	rootAbs, err := filepath.Abs(app.config.RootDir)
	if err != nil {
		panic(fmt.Sprintf("fiber: failed to resolve RootDir: %v", err))
	}
	rootAbs = filepath.Clean(rootAbs)
	if err = os.MkdirAll(rootAbs, app.config.RootPerms); err != nil {
		panic(fmt.Sprintf("fiber: failed to create RootDir %q: %v", rootAbs, err))
	}
	rootEval, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		panic(fmt.Sprintf("fiber: failed to resolve RootDir symlinks %q: %v", rootAbs, err))
	}
	rootEval = filepath.Clean(rootEval)
	if err := probeUploadDirWritable(rootAbs); err != nil {
		panic(fmt.Sprintf("fiber: RootDir not writable %q: %v", rootAbs, err))
	}

	app.config.uploadRootDir = rootAbs
	app.config.uploadRootEval = rootEval
	app.config.uploadRootPath = storageRootPrefix(app.config.RootDir)
}

func validateUploadPath(path string) (uploadPath, error) {
	if path == "" {
		return uploadPath{}, ErrInvalidUploadPath
	}
	if isAbsUploadPath(path) {
		return uploadPath{}, ErrInvalidUploadPath
	}
	if containsDotDot(path) {
		return uploadPath{}, ErrInvalidUploadPath
	}

	cleanOS := filepath.Clean(path)
	cleanOS = utils.TrimLeft(cleanOS, '.')
	cleanOS = utils.TrimLeft(cleanOS, byte(filepath.Separator))

	cleanSlash := pathpkg.Clean("/" + filepath.ToSlash(path))
	cleanSlash = utils.TrimLeft(cleanSlash, '/')

	if cleanOS == "." || cleanOS == "" || cleanSlash == "." || cleanSlash == "" {
		return uploadPath{}, ErrInvalidUploadPath
	}
	if !fs.ValidPath(cleanSlash) {
		return uploadPath{}, ErrInvalidUploadPath
	}

	return uploadPath{
		osPath:    cleanOS,
		slashPath: cleanSlash,
	}, nil
}

func isAbsUploadPath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	if filepath.VolumeName(path) != "" {
		return true
	}
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\")
}

func containsDotDot(path string) bool {
	return slices.Contains(strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	}), "..")
}

func storageRootPrefix(root string) string {
	if root == "" {
		return ""
	}
	root = filepath.Clean(root)
	if volume := filepath.VolumeName(root); volume != "" {
		root = root[len(volume):]
	}
	root = filepath.ToSlash(root)
	root = utils.TrimLeft(root, '/')
	if root == "." {
		return ""
	}
	return root
}

func cleanUploadRootPrefix(root string) (string, error) {
	if root == "" || root == "." {
		return "", nil
	}
	if isAbsUploadPath(root) || containsDotDot(root) {
		return "", ErrInvalidUploadPath
	}
	cleanSlash := pathpkg.Clean("/" + filepath.ToSlash(root))
	cleanSlash = utils.TrimLeft(cleanSlash, '/')
	if cleanSlash == "." || cleanSlash == "" {
		return "", ErrInvalidUploadPath
	}
	if !fs.ValidPath(cleanSlash) {
		return "", ErrInvalidUploadPath
	}
	return cleanSlash, nil
}

func ensureUploadPathWithinRoot(rootEval, fullPath string) error {
	parent := filepath.Dir(fullPath)
	parentEval, err := evalExistingPath(parent)
	if err != nil {
		return err
	}
	if !hasPathPrefix(parentEval, rootEval) {
		return ErrUploadPathEscapesRoot
	}
	return nil
}

func evalExistingPath(path string) (string, error) {
	current := path
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return "", fmt.Errorf("failed to resolve symlinks for %q: %w", current, resolveErr)
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to stat %q: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("failed to resolve upload path %q: %w", current, err)
		}
		current = parent
	}
}

func hasPathPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	if strings.HasSuffix(prefix, string(os.PathSeparator)) {
		return true
	}
	if len(path) == len(prefix) {
		return true
	}
	return len(path) > len(prefix) && path[len(prefix)] == os.PathSeparator
}

func ensureNoSymlinkFS(fsys fs.FS, fullPath string) error {
	parts := strings.Split(fullPath, "/")
	if len(parts) <= 1 {
		return nil
	}
	for i := 0; i < len(parts)-1; i++ {
		name := parts[i]
		if name == "" {
			continue
		}
		parent := strings.Join(parts[:i], "/")
		if parent == "" {
			parent = "."
		}
		entries, err := fs.ReadDir(fsys, parent)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("failed to read upload directory %q: %w", parent, err)
		}
		for _, entry := range entries {
			if entry.Name() == name && entry.Type()&fs.ModeSymlink != 0 {
				return ErrUploadPathEscapesRoot
			}
		}
	}
	return nil
}

func probeUploadDirWritable(root string) error {
	tempFile, err := os.CreateTemp(root, ".fiber-upload-check-*")
	if err != nil {
		return fmt.Errorf("failed to create probe file: %w", err)
	}
	if _, err := tempFile.WriteString("fiber"); err != nil {
		closeErr := tempFile.Close()
		if closeErr != nil {
			return fmt.Errorf("failed to close probe file: %w", closeErr)
		}
		removeErr := os.Remove(tempFile.Name())
		if removeErr != nil {
			return fmt.Errorf("failed to remove probe file: %w", removeErr)
		}
		return fmt.Errorf("failed to write probe file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		removeErr := os.Remove(tempFile.Name())
		if removeErr != nil {
			return fmt.Errorf("failed to remove probe file: %w", removeErr)
		}
		return fmt.Errorf("failed to close probe file: %w", err)
	}
	if err := os.Remove(tempFile.Name()); err != nil {
		return fmt.Errorf("failed to remove probe file: %w", err)
	}
	return nil
}

func probeUploadFSWritable(fsys interface {
	fs.FS
	OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error)
}, prefix string,
) error {
	name := pathpkg.Join(prefix, fmt.Sprintf(".fiber-upload-check-%d", time.Now().UnixNano()))
	file, err := fsys.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create probe file: %w", err)
	}
	writer, ok := file.(io.Writer)
	if !ok {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("failed to close probe file: %w", closeErr)
		}
		return errors.New("upload file is not writable")
	}
	if _, err := writer.Write([]byte("fiber")); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return fmt.Errorf("failed to close probe file: %w", closeErr)
		}
		return fmt.Errorf("failed to write probe file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close probe file: %w", err)
	}
	if remover, ok := fsys.(interface {
		Remove(name string) error
	}); ok {
		if err := remover.Remove(name); err != nil {
			return fmt.Errorf("failed to remove probe file: %w", err)
		}
	}
	return nil
}

func storageUploadPath(prefix, cleanSlash string) string {
	if prefix == "" {
		return cleanSlash
	}
	return pathpkg.Join(prefix, cleanSlash)
}
