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
		//nolint:testifylint // MIMEApplicationJSON is a plain string, JSONEq not required
		require.Equal(t, MIMEApplicationJSON, route.Consumes)
	})

	t.Run("Produces", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Produces(MIMEApplicationXML)
		route := app.stack[app.methodInt(MethodGet)][0]
		//nolint:testifylint // MIMEApplicationXML is a plain string, JSONEq not required
		require.Equal(t, MIMEApplicationXML, route.Produces)
	})

	t.Run("RequestBody", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Post("/users", testEmptyHandler).RequestBody("User", true, MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodPost)][0]
		require.NotNil(t, route.RequestBody)
		require.Equal(t, []string{MIMEApplicationJSON}, route.RequestBody.MediaTypes)
	})

	t.Run("RequestBodyWithExample", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Post("/users", testEmptyHandler).
			RequestBodyWithExample("User", true, map[string]any{"type": "object"}, "#/components/schemas/User", map[string]any{"name": "doe"}, map[string]any{"sample": map[string]any{"name": "john"}}, MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodPost)][0]
		require.NotNil(t, route.RequestBody)
		require.Equal(t, "#/components/schemas/User", route.RequestBody.SchemaRef)
		require.Equal(t, map[string]any{"$ref": "#/components/schemas/User"}, route.RequestBody.Schema)
		require.Equal(t, map[string]any{"name": "doe"}, route.RequestBody.Example)
	})

	t.Run("Parameter", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users/:id", testEmptyHandler).Parameter("id", "path", false, map[string]any{"type": "integer"}, "identifier")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Len(t, route.Parameters, 1)
		require.Equal(t, "id", route.Parameters[0].Name)
		require.True(t, route.Parameters[0].Required)
		require.Equal(t, "integer", route.Parameters[0].Schema["type"])
	})

	t.Run("ParameterWithExample", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users/:id", testEmptyHandler).
			ParameterWithExample("id", "path", false, nil, "#/components/schemas/ID", "identifier", "123", map[string]any{"sample": "value"})
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Len(t, route.Parameters, 1)
		require.Equal(t, "#/components/schemas/ID", route.Parameters[0].SchemaRef)
		require.Equal(t, "123", route.Parameters[0].Example)
		require.Equal(t, map[string]any{"sample": "value"}, route.Parameters[0].Examples)
	})

	t.Run("Response", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).Response(StatusCreated, "Created", MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Contains(t, route.Responses, "201")
		require.Equal(t, []string{MIMEApplicationJSON}, route.Responses["201"].MediaTypes)
	})

	t.Run("ResponseWithExample", func(t *testing.T) {
		t.Parallel()
		app := New()
		grp := app.Group("/api")
		grp.Get("/users", testEmptyHandler).
			ResponseWithExample(StatusCreated, "Created", nil, "#/components/schemas/User", map[string]any{"id": 1}, map[string]any{"sample": map[string]any{"id": 2}}, MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodGet)][0]
		resp := route.Responses["201"]
		require.Equal(t, "#/components/schemas/User", resp.SchemaRef)
		require.Equal(t, map[string]any{"$ref": "#/components/schemas/User"}, resp.Schema)
		require.Equal(t, map[string]any{"id": 1}, resp.Example)
		require.Equal(t, map[string]any{"sample": map[string]any{"id": 2}}, resp.Examples)
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
