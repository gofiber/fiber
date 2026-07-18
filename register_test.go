// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_RouteChain_OpenAPI_Helpers exercises the documentation helpers on the
// Registering (RouteChain) fluent API, which delegate to the app's
// route-metadata machinery for the chain's own registration.
func Test_RouteChain_OpenAPI_Helpers(t *testing.T) {
	t.Parallel()
	app := New()

	app.RouteChain("/users").Post(testEmptyHandler).
		Name("createUser").
		Summary("Create a user").
		Description("Creates a new user").
		Consumes(MIMEApplicationJSON).
		Produces(MIMEApplicationXML).
		Parameter("trace", "header", false, nil, "trace id").
		ParameterWithExample("lang", "query", false, nil, "", "language", "en", nil).
		AddParameter(RouteParameter{Name: "verbose", In: "query", Schema: map[string]any{"type": "boolean"}}).
		Response(StatusCreated, "Created", MIMEApplicationJSON).
		ResponseWithExample(StatusAccepted, "Accepted", nil, "#/components/schemas/User", map[string]any{"id": 1}, nil, MIMEApplicationJSON).
		ResponseHeader(StatusCreated, "Location", "resource url", nil).
		ResponseContent(StatusOK, "OK", map[string]RouteMediaType{MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}}).
		ResponseLink(StatusCreated, "self", map[string]any{"operationId": "createUser"}).
		Tags("users").
		Deprecated().
		Security(map[string][]string{"bearerAuth": {}}).
		Hidden().
		OperationExternalDocs("docs", "https://example.com/docs").
		OperationExtension(map[string]any{"x-team": "core"})

	route := app.stack[app.methodInt(MethodPost)][0]
	require.Equal(t, "createUser", route.Name)
	require.Equal(t, "Create a user", route.Summary)
	require.Equal(t, "Creates a new user", route.Description)
	//nolint:testifylint // MIME type string, not a JSON payload
	require.Equal(t, MIMEApplicationJSON, route.Consumes)
	require.Equal(t, MIMEApplicationXML, route.Produces)
	require.Len(t, route.Parameters, 3)
	require.Contains(t, route.Responses, "201")
	require.Contains(t, route.Responses, "202")
	require.Contains(t, route.Responses["201"].Headers, "Location")
	require.Contains(t, route.Responses["200"].Content, MIMEApplicationJSON)
	require.Contains(t, route.Responses["201"].Links, "self")
	require.Equal(t, []string{"users"}, route.Tags)
	require.True(t, route.Deprecated)
	require.Len(t, route.Security, 1)
	require.True(t, route.IsHidden())
	require.Equal(t, "https://example.com/docs", route.ExternalDocs["url"])
	require.Equal(t, "core", route.OperationExtensions["x-team"])

	// Request-body variants overwrite one another, so exercise each separately.
	app.RouteChain("/rb-plain").Put(testEmptyHandler).RequestBody("Body", true, MIMEApplicationJSON)
	require.Equal(t, []string{MIMEApplicationJSON}, app.stack[app.methodInt(MethodPut)][0].RequestBody.MediaTypes)

	app.RouteChain("/rb-example").Patch(testEmptyHandler).
		RequestBodyWithExample("Body", true, nil, "#/components/schemas/User", nil, nil, MIMEApplicationJSON)
	require.Equal(t, "#/components/schemas/User", app.stack[app.methodInt(MethodPatch)][0].RequestBody.SchemaRef)

	app.RouteChain("/rb-content").Delete(testEmptyHandler).
		RequestBodyContent("Body", true, map[string]RouteMediaType{MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}})
	require.Contains(t, app.stack[app.methodInt(MethodDelete)][0].RequestBody.Content, MIMEApplicationJSON)
}

// Test_RouteChain_Nested verifies a nested RouteChain inherits the parent
// path as a prefix and documents its own route.
func Test_RouteChain_Nested(t *testing.T) {
	t.Parallel()
	app := New()

	app.RouteChain("/api").RouteChain("/users").Get(testEmptyHandler).Summary("List users")

	route := app.stack[app.methodInt(MethodGet)][0]
	require.Equal(t, "/api/users", route.Path)
	require.Equal(t, "List users", route.Summary)
}
