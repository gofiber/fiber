package openapi

import (
	"testing"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func listUsers(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

func passThrough(c fiber.Ctx) error { return c.Next() }

type userService struct{}

func (*userService) GetUserByID(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

// rfInferCase registers routes on a fresh app and names the GET operation whose field a test inspects.
type rfInferCase struct {
	register   func(app *fiber.App)
	want       any
	name, path string
	cfg        Config
}

// rfRunInferCases runs each case on its own app and compares the field key of its GET operation with want; a nil
// want expects the field to be absent.
func rfRunInferCases(t *testing.T, key string, cases []rfInferCase) {
	t.Helper()
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			tc.register(app)
			app.Use(New(tc.cfg))
			op := modelOperation(t, fetchJSON(t, app), tc.path, "get")
			if tc.want == nil {
				require.NotContains(t, op, key)
				return
			}
			require.Equal(t, tc.want, op[key])
		})
	}
}

func Test_OpenAPI_HandlerSummaries(t *testing.T) {
	t.Parallel()
	rfRunInferCases(t, "summary", []rfInferCase{
		{name: "named function", register: func(app *fiber.App) { app.Get("/users", listUsers) }, path: "/users", want: "List users"},
		{name: "method value", register: func(app *fiber.App) { app.Get("/users/:id", (&userService{}).GetUserByID) }, path: "/users/{id}", want: "Get user by ID"},
		{name: "last handler names the operation", register: func(app *fiber.App) { app.Get("/users", passThrough, listUsers) }, path: "/users", want: "List users"},
		{name: "closure falls back to method and path", path: "/users", want: "GET /users", register: func(app *fiber.App) {
			app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		}},
		{name: "explicit summary wins", register: func(app *fiber.App) { app.Get("/users", listUsers).Summary("Everyone") }, path: "/users", want: "Everyone"},
		{name: "disabled", register: func(app *fiber.App) { app.Get("/users", listUsers) }, path: "/users", want: "GET /users", cfg: Config{DisableHandlerSummaries: true}},
	})
}

func Test_OpenAPI_GroupTags(t *testing.T) {
	t.Parallel()
	rfRunInferCases(t, "tags", []rfInferCase{
		{name: "innermost group", register: func(app *fiber.App) { app.Group("/api").Group("/v1").Group("/users").Get("/", listUsers) }, path: "/api/v1/users", want: []any{"users"}},
		{name: "version segments are skipped", register: func(app *fiber.App) { app.Group("/api/v1").Get("/users", listUsers) }, path: "/api/v1/users", want: []any{"api"}},
		{name: "parameter segments are skipped", register: func(app *fiber.App) { app.Group("/users/:id").Get("/posts", listUsers) }, path: "/users/{id}/posts", want: []any{"users"}},
		{name: "named group uses its name", path: "/api/u", want: []any{"users"}, register: func(app *fiber.App) {
			app.Group("/api").Name("api.").Group("/u").Name("users.").Get("/", listUsers)
		}},
		{name: "explicit tags win", register: func(app *fiber.App) { app.Group("/users").Get("/", listUsers).Tags("people") }, path: "/users", want: []any{"people"}},
		{name: "route on the app has no tags", register: func(app *fiber.App) { app.Get("/users", listUsers) }, path: "/users"},
		{name: "only version segments yield no tag", register: func(app *fiber.App) { app.Group("/v2").Get("/users", listUsers) }, path: "/v2/users"},
		{name: "disabled", register: func(app *fiber.App) { app.Group("/users").Get("/", listUsers) }, path: "/users", cfg: Config{DisableGroupTags: true}},
	})
}

func Test_OpenAPI_DefaultProduces(t *testing.T) {
	t.Parallel()
	statuses := func(app *fiber.App) {
		app.Put("/users/:id", listUsers).Response(fiber.StatusNoContent, "Updated").Response(fiber.StatusNotModified, "Unchanged").Response(fiber.StatusContinue, "Go on")
	}
	for _, tc := range []struct {
		register           func(app *fiber.App)
		name, path, method string
		status             string
		content            []string
		cfg                Config
	}{
		{name: "undocumented route documents JSON", register: func(app *fiber.App) { app.Get("/users", listUsers) }, path: "/users", method: "get", status: "200", content: []string{fiber.MIMEApplicationJSON}},
		{name: "configured default", register: func(app *fiber.App) { app.Get("/users", listUsers) }, path: "/users", method: "get", status: "200", content: []string{fiber.MIMETextPlain}, cfg: Config{DefaultProduces: fiber.MIMETextPlain}},
		{name: "explicit produces wins", register: func(app *fiber.App) { app.Get("/users", listUsers).Produces(fiber.MIMEApplicationXML) }, path: "/users", method: "get", status: "200", content: []string{fiber.MIMEApplicationXML}},
		{name: "declared response without a media type uses the default", path: "/users", method: "post", status: "201", content: []string{fiber.MIMEApplicationJSON}, register: func(app *fiber.App) {
			app.Post("/users", listUsers).Response(fiber.StatusCreated, "Created")
		}},
		{name: "204 stays without content", register: statuses, path: "/users/{id}", method: "put", status: "204"},
		{name: "304 stays without content", register: statuses, path: "/users/{id}", method: "put", status: "304"},
		{name: "100 stays without content", register: statuses, path: "/users/{id}", method: "put", status: "100"},
		{name: "delete default stays without content", register: func(app *fiber.App) { app.Delete("/users/:id", listUsers) }, path: "/users/{id}", method: "delete", status: "204"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			tc.register(app)
			app.Use(New(tc.cfg))
			resp := requireMap(t, requireMap(t, modelOperation(t, fetchJSON(t, app), tc.path, tc.method)["responses"])[tc.status])
			if tc.content == nil {
				require.NotContains(t, resp, "content")
				return
			}
			require.Equal(t, tc.content, rfKeys(requireMap(t, resp["content"])))
		})
	}
}

func Test_summaryFromFuncName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"main.listUsers":                         "List users",
		"github.com/acme/api/handlers.ListUsers": "List users",
		"main.(*Server).GetUserByID-fm":          "Get user by ID",
		"main.HTTPServer":                        "HTTP server",
		"main.get_user_v2":                       "Get user v2",
		"main.listIDs":                           "List IDs",
		"main.getURLs":                           "Get URLs",
		"main.Handler[...]":                      "Handler",
		"main.S[...].List-fm":                    "List",
		"main.(*S[...]).Get-fm":                  "Get",
		"main.(*Repo[go.shape.struct { X []int }]).FindByID-fm": "Find by ID",
		"main.Handler[go.shape.int]":                            "Handler",
		"main.x":                                                "X",
		"main.main.func1":                                       "",
		"main.main.func1.2":                                     "",
		"main.glob..func1":                                      "",
		"main.init.deferwrap1":                                  "",
		"main.run.gowrap1":                                      "",
		"":                                                      "",
		"main.":                                                 "",
		"main.(*Server).handle-fm":                              "Handle",
		"github.com/acme/api/v2/handlers.Create":                "Create",
		"main._":                                                "",
		"pkg.日本語Handler":                                        "日本語 handler",
		"pkg._2fa":                                              "2fa",
		"pkg.élanCheck":                                         "Élan check",
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := summaryFromFuncName(name)
			require.True(t, utf8.ValidString(got))
			require.Equal(t, want, got)
		})
	}
}

func Test_statusHasNoBody(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]bool{
		"100": true, "101": true, "204": true, "205": true, "304": true,
		"200": false, "201": false, "404": false, "default": false, "1": false,
	} {
		require.Equal(t, want, statusHasNoBody(code), code)
	}
}

func Test_handlerSummary_Guards(t *testing.T) {
	t.Parallel()
	require.Empty(t, handlerSummary(nil))
	require.Empty(t, handlerSummary([]fiber.Handler{}))
	require.Empty(t, handlerSummary([]fiber.Handler{listUsers, nil}))
	require.Equal(t, "List users", handlerSummary([]fiber.Handler{passThrough, listUsers}))
}

func Test_isVersionSegment(t *testing.T) {
	t.Parallel()
	for segment, want := range map[string]bool{
		"v1": true, "V2": true, "v2.1": true, "v10": true,
		"v": false, "": false, "va1": false, "1": false, "version": false, "v1beta": false,
	} {
		require.Equal(t, want, isVersionSegment(segment), segment)
	}
}
