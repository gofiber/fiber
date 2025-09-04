package fiber

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Group_OpenAPI_Helpers(t *testing.T) {
	t.Parallel()

	t.Run("Summary", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Summary("sum")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, "sum", route.Summary)
	})

	t.Run("Description", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Description("desc")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, "desc", route.Description)
	})

	t.Run("Consumes", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Consumes(MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, MIMEApplicationJSON, route.Consumes)
	})

	t.Run("Produces", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Produces(MIMEApplicationXML)
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, MIMEApplicationXML, route.Produces)
	})

	t.Run("Tags", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Tags("foo", "bar")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, []string{"foo", "bar"}, route.Tags)
	})

	t.Run("Deprecated", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Deprecated()
		route := app.stack[app.methodInt(MethodGet)][0]
		require.True(t, route.Deprecated)
	})
}
