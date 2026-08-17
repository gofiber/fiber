package fiber

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
)

// middlewareDocsDir holds one page per built-in middleware.
const middlewareDocsDir = "docs/middleware"

// Sections every middleware page carries, so a reader can jump to the
// signature or the config of any middleware without first reading the page.
var middlewareDocSections = []string{
	"## Signatures",
	"## Config",
	"## Examples",
}

type middlewareDocException struct {
	file   string
	reason string
	skip   []string
}

// Pages that are not shaped like a middleware page. Listing them here rather
// than loosening the check keeps the deviation visible and reviewable.
var middlewareDocExceptions = []middlewareDocException{
	{
		file:   "adaptor.md",
		skip:   []string{"## Signatures", "## Config"},
		reason: "a converter between net/http and Fiber, documented one function at a time and with no config",
	},
	{
		file:   "csrf.md",
		skip:   []string{"## Signatures", "## Config", "## Examples"},
		reason: "guide-shaped page, config lives under Advanced Configuration and the examples under Recipes",
	},
	{
		file:   "session.md",
		skip:   []string{"## Signatures", "## Config"},
		reason: "guide-shaped page, the config is reached through Configuration",
	},
	{
		file:   "skip.md",
		skip:   []string{"## Config"},
		reason: "wraps a handler behind a predicate, there is no config struct",
	},
}

// Test_Docs_MiddlewarePageSections keeps the middleware pages to one layout.
// The docs are synced to gofiber.io as they are, so a page that invents its
// own section names is only noticed once it is published.
func Test_Docs_MiddlewarePageSections(t *testing.T) {
	t.Parallel()

	skips := make(map[string]map[string]bool, len(middlewareDocExceptions))
	for _, exception := range middlewareDocExceptions {
		set := make(map[string]bool, len(exception.skip))
		for _, section := range exception.skip {
			set[section] = true
		}
		skips[exception.file] = set
	}

	entries, err := os.ReadDir(middlewareDocsDir)
	require.NoError(t, err)

	pages := make(map[string]bool, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		pages[name] = true

		body, readErr := os.ReadFile(filepath.Join(middlewareDocsDir, name))
		require.NoError(t, readErr)

		headings := make(map[string]bool)
		for line := range strings.SplitSeq(string(body), "\n") {
			line = strings.TrimRight(line, " \r")
			if strings.HasPrefix(line, "## ") {
				headings[line] = true
			}
		}

		for _, section := range middlewareDocSections {
			if skips[name][section] {
				// A page that has grown the section back keeps its exception alive
				// and with it a hole in the check, so the skip has to go.
				require.False(t, headings[section],
					"%s/%s carries %q again - drop that section from its middlewareDocExceptions entry",
					middlewareDocsDir, name, section)
				continue
			}
			require.True(t, headings[section],
				"%s/%s is missing %q - add the section, or add the page to middlewareDocExceptions with a reason",
				middlewareDocsDir, name, section)
		}
	}

	require.NotEmpty(t, pages, "no middleware pages found under %s", middlewareDocsDir)

	// An exception for a page that no longer exists would silently excuse that
	// page the day someone adds it back.
	for _, exception := range middlewareDocExceptions {
		require.NotEmpty(t, utils.TrimSpace(exception.reason),
			"middlewareDocExceptions covers %s without a reason", exception.file)
		require.True(t, pages[exception.file],
			"middlewareDocExceptions covers %s (%s) but that page does not exist",
			exception.file, exception.reason)
	}
}
