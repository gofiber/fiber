package urlnorm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_RootedPath covers the two sources of a leading slash: the route path the
// author registered, whose slashes name the route and must survive, and a
// parameter spliced at the front, whose slashes would compose a host.
func Test_RootedPath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		location  string
		routePath string
		want      string
	}{
		{"ordinary path", "/user/john", "/user/:name", "/user/john"},

		// The value opened a second slash on a route that starts one.
		{"value adds a slash", "//evil.com", "/*", "/evil.com"},
		{"value adds a backslash", `/\evil.com`, "/*", "/evil.com"},
		{"value adds several", "///evil.com", "/*", "/evil.com"},
		{"value is only slashes", "//", "/*", "/"},

		// The author registered the slashes, so they name the route: collapsing
		// them composed a path that answers 404.
		{"route starts two", "//internal", "//internal", "//internal"},
		{"route starts two with a value", "//internal/john", "//internal/:name", "//internal/john"},
		{"route starts two, value adds one", "///evil.com", "//*", "//evil.com"},

		// A backslash the author wrote is folded, the parser reading it as one.
		{"route backslash is folded", `\internal`, `\internal`, "/internal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, RootedPath(tc.location, tc.routePath))
		})
	}
}
