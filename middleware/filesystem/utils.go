package filesystem

import (
	"fmt"
	"html"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func getFileExtension(p string) string {
	n := strings.LastIndexByte(p, '.')
	if n < 0 {
		return ""
	}
	return p[n:]
}

func dirList(c fiber.Ctx, f fs.File) error {
	ff := f.(fs.ReadDirFile)
	fileinfos, err := ff.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("failed to read dir: %w", err)
	}

	fm := make(map[string]fs.FileInfo, len(fileinfos))
	filenames := make([]string, 0, len(fileinfos))
	for _, fi := range fileinfos {
		name := fi.Name()
		info, err := fi.Info()
		if err != nil {
			return err
		}

		fm[name] = info
		filenames = append(filenames, name)
	}

	basePathEscaped := html.EscapeString(c.Path())
	_, _ = fmt.Fprintf(c, "<html><head><title>%s</title><style>.dir { font-weight: bold }</style></head><body>", basePathEscaped)
	_, _ = fmt.Fprintf(c, "<h1>%s</h1>", basePathEscaped)
	_, _ = fmt.Fprint(c, "<ul>")

	if len(basePathEscaped) > 1 {
		parentPathEscaped := html.EscapeString(strings.TrimRight(c.Path(), "/") + "/..")
		_, _ = fmt.Fprintf(c, `<li><a href="%s" class="dir">..</a></li>`, parentPathEscaped)
	}

	sort.Strings(filenames)
	for _, name := range filenames {
		pathEscaped := html.EscapeString(path.Join(c.Path() + "/" + name))
		fi := fm[name]
		auxStr := "dir"
		className := "dir"
		if !fi.IsDir() {
			auxStr = fmt.Sprintf("file, %d bytes", fi.Size())
			className = "file"
		}
		_, _ = fmt.Fprintf(c, `<li><a href="%s" class="%s">%s</a>, %s, last modified %s</li>`,
			pathEscaped, className, html.EscapeString(name), auxStr, fi.ModTime())
	}
	_, _ = fmt.Fprint(c, "</ul></body></html>")

	c.Type("html")

	return nil
}

func openFile(filesystem fs.FS, name string) (fs.File, error) {
	name = filepath.ToSlash(name)

	return filesystem.Open(name)
}
