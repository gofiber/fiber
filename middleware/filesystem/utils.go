package filesystem

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

func getFileExtension(path string) string {
	n := strings.LastIndexByte(path, '.')
	if n < 0 {
		return ""
	}
	return path[n:]
}

func dirList(c *fiber.Ctx, f http.File) error {
	fileinfos, err := f.Readdir(-1)
	if err != nil {
		return err
	}

	fm := make(map[string]os.FileInfo, len(fileinfos))
	filenames := make([]string, 0, len(fileinfos))
	for _, fi := range fileinfos {
		name := fi.Name()
		fm[name] = fi
		filenames = append(filenames, name)
	}

	basePathEscaped := html.EscapeString(c.Path())
	fmt.Fprintf(c, "<html><head><title>%s</title><style>.dir { font-weight: bold }</style></head><body>", basePathEscaped)
	fmt.Fprintf(c, "<h1>%s</h1>", basePathEscaped)
	fmt.Fprint(c, "<ul>")

	if len(basePathEscaped) > 1 {
		parentPathEscaped := html.EscapeString(utils.TrimRight(c.Path(), '/') + "/..")
		fmt.Fprintf(c, `<li><a href="%s" class="dir">..</a></li>`, parentPathEscaped)
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
		fmt.Fprintf(c, `<li><a href="%s" class="%s">%s</a>, %s, last modified %s</li>`,
			pathEscaped, className, html.EscapeString(name), auxStr, fi.ModTime())
	}
	fmt.Fprint(c, "</ul></body></html>")

	c.Type("html")

	return nil
}
