// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var errTestDomainHook = errors.New("domain hook failure")

func Test_Domain_Basic(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get("/hello", func(c Ctx) error {
		return c.SendString("api hello")
	})

	// Matching domain
	req := httptest.NewRequest(MethodGet, "/hello", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "api hello", string(body))

	// Non-matching domain → 404
	req = httptest.NewRequest(MethodGet, "/hello", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_Params(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain(":user.blog.example.com").Get("/", func(c Ctx) error {
		user := DomainParam(c, "user")
		return c.SendString("blog of " + user)
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "john.blog.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "blog of john", string(body))
}

func Test_Domain_MultipleParams(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain(":sub.:region.example.com").Get("/", func(c Ctx) error {
		sub := DomainParam(c, "sub")
		region := DomainParam(c, "region")
		return c.SendString(sub + "-" + region)
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "api.us-east.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "api-us-east", string(body))
}

func Test_Domain_CaseInsensitive(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("API.Example.COM").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))
}

func Test_Domain_ParamNameCasePreserved(t *testing.T) {
	t.Parallel()

	app := New()

	// Use mixed-case param name ":User" — DomainParam should find it by exact name
	app.Domain(":User.example.com").Get("/", func(c Ctx) error {
		return c.SendString(DomainParam(c, "User"))
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "alice.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "alice", string(body))
}

func Test_Domain_TrailingDot(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Fully-qualified domain name with trailing dot should match
	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "api.example.com."
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))
}

func Test_Domain_MultipleDomains(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get("/", func(c Ctx) error {
		return c.SendString("api")
	})

	app.Domain("www.example.com").Get("/", func(c Ctx) error {
		return c.SendString("www")
	})

	// First domain
	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "api", string(body))

	// Second domain
	req = httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "www", string(body))

	// Unknown domain
	req = httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "other.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_WithGroup(t *testing.T) {
	t.Parallel()

	app := New()

	domain := app.Domain("api.example.com")
	api := domain.Group("/api")
	api.Get("/users", func(c Ctx) error {
		return c.SendString("users list")
	})

	// Matching domain + path
	req := httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "users list", string(body))

	// Wrong domain
	req = httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_WithMiddleware(t *testing.T) {
	t.Parallel()

	app := New()

	domain := app.Domain("api.example.com")
	domain.Use(func(c Ctx) error {
		c.Set("X-Domain", "api")
		return c.Next()
	})
	domain.Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Matching domain - middleware should set header
	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "api", resp.Header.Get("X-Domain"))

	// Non-matching domain - middleware should not set header
	req = httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.Empty(t, resp.Header.Get("X-Domain"))
}

func Test_Domain_OpenAPI_Helpers(t *testing.T) {
	t.Parallel()

	t.Run("Summary", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).Summary("Get all users")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, "Get all users", route.Summary)
	})

	t.Run("Description", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).Description("Retrieves all users")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, "Retrieves all users", route.Description)
	})

	t.Run("Consumes", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).Consumes(MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodGet)][0]
		//nolint:testifylint // MIMEApplicationJSON is a MIME type string, not JSON payload
		require.Equal(t, MIMEApplicationJSON, route.Consumes)
	})

	t.Run("Produces", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).Produces(MIMEApplicationXML)
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, MIMEApplicationXML, route.Produces)
	})

	t.Run("RequestBody", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Post("/users", testEmptyHandler).RequestBody("User", true, MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodPost)][0]
		require.NotNil(t, route.RequestBody)
		require.Equal(t, []string{MIMEApplicationJSON}, route.RequestBody.MediaTypes)
	})

	t.Run("RequestBodyWithExample", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Post("/users", testEmptyHandler).
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
		domain := app.Domain("api.example.com")
		domain.Get("/users/:id", testEmptyHandler).Parameter("id", "path", false, map[string]any{"type": "integer"}, "identifier")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Len(t, route.Parameters, 1)
		require.Equal(t, "id", route.Parameters[0].Name)
		require.True(t, route.Parameters[0].Required)
		require.Equal(t, "integer", route.Parameters[0].Schema["type"])
	})

	t.Run("ParameterWithExample", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Get("/users/:id", testEmptyHandler).
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
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).Response(StatusCreated, "Created", MIMEApplicationJSON)
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Contains(t, route.Responses, "201")
		require.Equal(t, []string{MIMEApplicationJSON}, route.Responses["201"].MediaTypes)
	})

	t.Run("ResponseWithExample", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).
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
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).Tags("users", "api")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, []string{"users", "api"}, route.Tags)
	})

	t.Run("Deprecated", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Get("/users", testEmptyHandler).Deprecated()
		route := app.stack[app.methodInt(MethodGet)][0]
		require.True(t, route.Deprecated)
	})
}

func Test_Domain_HTTPMethods(t *testing.T) {
	t.Parallel()

	methods := []struct {
		reg    func(Router, string, any, ...any) Router
		method string
	}{
		{method: MethodGet, reg: func(r Router, p string, h any, hs ...any) Router { return r.Get(p, h, hs...) }},
		{method: MethodPost, reg: func(r Router, p string, h any, hs ...any) Router { return r.Post(p, h, hs...) }},
		{method: MethodPut, reg: func(r Router, p string, h any, hs ...any) Router { return r.Put(p, h, hs...) }},
		{method: MethodDelete, reg: func(r Router, p string, h any, hs ...any) Router { return r.Delete(p, h, hs...) }},
		{method: MethodPatch, reg: func(r Router, p string, h any, hs ...any) Router { return r.Patch(p, h, hs...) }},
		{method: MethodQuery, reg: func(r Router, p string, h any, hs ...any) Router { return r.Query(p, h, hs...) }},
		{method: MethodOptions, reg: func(r Router, p string, h any, hs ...any) Router { return r.Options(p, h, hs...) }},
		{method: MethodConnect, reg: func(r Router, p string, h any, hs ...any) Router { return r.Connect(p, h, hs...) }},
		{method: MethodTrace, reg: func(r Router, p string, h any, hs ...any) Router { return r.Trace(p, h, hs...) }},
	}

	for _, m := range methods {
		t.Run(m.method, func(t *testing.T) {
			t.Parallel()
			app := New()

			domain := app.Domain("api.example.com")
			m.reg(domain, "/test", func(c Ctx) error {
				return c.SendString(m.method)
			})

			req := httptest.NewRequest(m.method, "/test", http.NoBody)
			req.Host = "api.example.com"
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, StatusOK, resp.StatusCode)
		})
	}
}

func Test_Domain_Head(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Head("/test", func(c Ctx) error {
		c.Set("X-Custom", "head")
		return c.SendStatus(StatusOK)
	})

	req := httptest.NewRequest(MethodHead, "/test", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Domain_All(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").All("/test", func(c Ctx) error {
		return c.SendString("all methods")
	})

	for _, method := range []string{MethodGet, MethodPost, MethodPut, MethodDelete} {
		req := httptest.NewRequest(method, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	}
}

func Test_Domain_Add(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Add([]string{MethodGet, MethodPost}, "/test", func(c Ctx) error {
		return c.SendString("ok")
	})

	// GET should work
	req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	// POST should work
	req = httptest.NewRequest(MethodPost, "/test", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Domain_DomainParam_DefaultValue(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("example.com").Get("/", func(c Ctx) error {
		// No domain params set, should return default
		user := DomainParam(c, "user", "default")
		return c.SendString(user)
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "default", string(body))
}

func Test_Domain_DomainParam_NoDefault(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("example.com").Get("/", func(c Ctx) error {
		user := DomainParam(c, "user")
		return c.SendString("user=" + user)
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "user=", string(body))
}

func Test_Domain_WithHostPort(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Host with port - Hostname() strips the port
	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "api.example.com:8080"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Domain_NoMatch_WrongPartCount(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Different number of domain parts
	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	req = httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "sub.api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_Route(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Route("/api", func(router Router) {
		router.Get("/users", func(c Ctx) error {
			return c.SendString("users")
		})
		router.Get("/posts", func(c Ctx) error {
			return c.SendString("posts")
		})
	})

	req := httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "users", string(body))

	req = httptest.NewRequest(MethodGet, "/api/posts", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "posts", string(body))

	// Wrong domain
	req = httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_Name(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get("/test", func(c Ctx) error {
		return c.SendString("ok")
	}).Name("api-test")

	// Verify route was named - by checking routes
	var found bool
	for _, routes := range app.Stack() {
		for _, route := range routes {
			if route.Name == "api-test" {
				found = true
				break
			}
		}
	}
	require.True(t, found, "route should be named 'api-test'")
}

func Test_Domain_NameWithGroup(t *testing.T) {
	t.Parallel()

	app := New()

	// When Domain is used with Route(prefix, fn, name), the group Name()
	// should apply properly via delegation to the underlying group.
	api := app.Domain("api.example.com")
	api.Route("/v1", func(r Router) {
		r.Get("/items", func(c Ctx) error {
			return c.SendString("items")
		}).Name("items-list")
	}, "v1.")

	var found bool
	for _, routes := range app.Stack() {
		for _, route := range routes {
			if route.Name == "v1.items-list" {
				found = true
				break
			}
		}
	}
	require.True(t, found, "route should be named 'v1.items-list'")
}

func Test_Domain_RouteChain(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").RouteChain("/api/users").
		Get(func(c Ctx) error {
			return c.SendString("get users")
		}).
		Post(func(c Ctx) error {
			return c.SendString("create user")
		})

	// GET
	req := httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "get users", string(body))

	// POST
	req = httptest.NewRequest(MethodPost, "/api/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "create user", string(body))

	// Wrong domain
	req = httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_GroupFromGroup(t *testing.T) {
	t.Parallel()

	app := New()

	api := app.Group("/api")
	domain := api.Domain("api.example.com")
	domain.Get("/users", func(c Ctx) error {
		return c.SendString("users from group domain")
	})

	req := httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "users from group domain", string(body))

	// Wrong domain
	req = httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_UseWithPrefix(t *testing.T) {
	t.Parallel()

	app := New()

	domain := app.Domain("api.example.com")
	domain.Use("/api", func(c Ctx) error {
		c.Set("X-API", "true")
		return c.Next()
	})
	domain.Get("/api/data", func(c Ctx) error {
		return c.SendString("data")
	})

	req := httptest.NewRequest(MethodGet, "/api/data", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "true", resp.Header.Get("X-API"))
}

func Test_Domain_FallbackToNonDomain(t *testing.T) {
	t.Parallel()

	app := New()

	// Domain-specific route registered first
	app.Domain("api.example.com").Get("/", func(c Ctx) error {
		return c.SendString("api")
	})

	// Fallback non-domain route
	app.Get("/", func(c Ctx) error {
		return c.SendString("fallback")
	})

	// Domain route matches
	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "api", string(body))

	// Fallback route matches for other domains
	req = httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "fallback", string(body))
}

func Test_Domain_WithPathParams(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain(":tenant.example.com").Get("/users/:id", func(c Ctx) error {
		tenant := DomainParam(c, "tenant")
		id := c.Params("id")
		return c.SendString(tenant + ":" + id)
	})

	req := httptest.NewRequest(MethodGet, "/users/42", http.NoBody)
	req.Host = "acme.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "acme:42", string(body))
}

func Test_Domain_NestedGroups(t *testing.T) {
	t.Parallel()

	app := New()

	domain := app.Domain("api.example.com")
	v1 := domain.Group("/v1")
	users := v1.Group("/users")
	users.Get("/", func(c Ctx) error {
		return c.SendString("v1 users")
	})

	req := httptest.NewRequest(MethodGet, "/v1/users/", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "v1 users", string(body))
}

func Test_Domain_Chaining(t *testing.T) {
	t.Parallel()

	app := New()

	// Methods should return the domain router for chaining
	domain := app.Domain("api.example.com")
	domain.
		Get("/a", func(c Ctx) error { return c.SendString("a") }).
		Post("/b", func(c Ctx) error { return c.SendString("b") })

	req := httptest.NewRequest(MethodGet, "/a", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	req = httptest.NewRequest(MethodPost, "/b", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Domain_MultipleHandlers(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get(
		"/test",
		func(c Ctx) error {
			c.Set("X-First", "true")
			return c.Next()
		},
		func(c Ctx) error {
			return c.SendString("final")
		},
	)

	// Matching host — both handlers should run
	req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "true", resp.Header.Get("X-First"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "final", string(body))

	// Non-matching host — none of the handlers should run
	req = httptest.NewRequest(MethodGet, "/test", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.Empty(t, resp.Header.Get("X-First"))
}

func Test_Domain_NetHTTPHandler(t *testing.T) {
	t.Parallel()

	app := New()

	// Register a net/http handler through domain routing
	app.Domain("api.example.com").Get("/http", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("net/http handler"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))

	req := httptest.NewRequest(MethodGet, "/http", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Domain_EmptyHostname(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Empty host should not match
	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = ""
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_DomainOnDomain(t *testing.T) {
	t.Parallel()

	app := New()

	// Domain created from a domain router (should replace the pattern)
	base := app.Domain("api.example.com")
	other := base.Domain("www.example.com")

	other.Get("/", func(c Ctx) error {
		return c.SendString("www")
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "www.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "www", string(body))
}

func Test_Domain_GroupMiddleware(t *testing.T) {
	t.Parallel()

	app := New()

	domain := app.Domain("api.example.com")
	api := domain.Group("/api", func(c Ctx) error {
		c.Set("X-Group-MW", "yes")
		return c.Next()
	})
	api.Get("/data", func(c Ctx) error {
		return c.SendString("data")
	})

	// Matching domain
	req := httptest.NewRequest(MethodGet, "/api/data", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "yes", resp.Header.Get("X-Group-MW"))

	// Non-matching domain
	req = httptest.NewRequest(MethodGet, "/api/data", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.Empty(t, resp.Header.Get("X-Group-MW"))
}

func Test_Domain_UseMultiplePrefixes(t *testing.T) {
	t.Parallel()

	app := New()

	domain := app.Domain("api.example.com")
	domain.Use([]string{"/a", "/b"}, func(c Ctx) error {
		c.Set("X-Domain-MW", "true")
		return c.Next()
	})
	domain.Get("/a/test", func(c Ctx) error {
		return c.SendString("a")
	})
	domain.Get("/b/test", func(c Ctx) error {
		return c.SendString("b")
	})

	req := httptest.NewRequest(MethodGet, "/a/test", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "true", resp.Header.Get("X-Domain-MW"))

	req = httptest.NewRequest(MethodGet, "/b/test", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "true", resp.Header.Get("X-Domain-MW"))
}

func Test_Domain_RoutePanic(t *testing.T) {
	t.Parallel()

	app := New()

	require.Panics(t, func() {
		app.Domain("api.example.com").Route("/test", nil)
	})
}

func Test_Domain_UseMount(t *testing.T) {
	t.Parallel()

	app := New()
	subApp := New()

	// Create routes in the sub-app
	subApp.Get("/users", func(c Ctx) error {
		return c.SendString("users list")
	})
	subApp.Get("/posts", func(c Ctx) error {
		return c.SendString("posts list")
	})

	// Mount the sub-app on the domain router
	app.Domain("api.example.com").Use("/api", subApp)

	// Test that sub-app routes work on the correct domain
	req := httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "users list", string(body))

	// Test second route
	req = httptest.NewRequest(MethodGet, "/api/posts", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "posts list", string(body))

	// Test that sub-app routes don't work on wrong domain
	req = httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_UseMountNoPrefix(t *testing.T) {
	t.Parallel()

	app := New()
	subApp := New()

	// Create a route in the sub-app
	subApp.Get("/users", func(c Ctx) error {
		return c.SendString("users list")
	})

	// Mount the sub-app at root on the domain router
	app.Domain("api.example.com").Use(subApp)

	// Test that sub-app routes work
	req := httptest.NewRequest(MethodGet, "/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "users list", string(body))

	// Wrong domain should 404
	req = httptest.NewRequest(MethodGet, "/users", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_UseMountFromGroup(t *testing.T) {
	t.Parallel()

	app := New()
	subApp := New()

	// Create a route in the sub-app
	subApp.Get("/data", func(c Ctx) error {
		return c.SendString("data response")
	})

	// Mount via a group's domain router
	api := app.Group("/api")
	api.Domain("api.example.com").Use("/v1", subApp)

	// Test that sub-app routes work with group prefix + mount prefix
	req := httptest.NewRequest(MethodGet, "/api/v1/data", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "data response", string(body))

	// Wrong domain should 404
	req = httptest.NewRequest(MethodGet, "/api/v1/data", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Domain_StaleParamsCleared(t *testing.T) {
	t.Parallel()

	app := New()

	// First: a domain with a parameter
	app.Domain(":tenant.example.com").Use(func(c Ctx) error {
		c.Set("X-Tenant", DomainParam(c, "tenant"))
		return c.Next()
	})

	// Second: a static domain (no params) — should clear any stale params
	app.Domain("static.example.com").Get("/check", func(c Ctx) error {
		// DomainParam should return "" since the static domain has no params
		val := DomainParam(c, "tenant")
		return c.SendString("tenant=" + val)
	})

	req := httptest.NewRequest(MethodGet, "/check", http.NoBody)
	req.Host = "static.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "tenant=", string(body))
}

func Test_Domain_RouteChainNested(t *testing.T) {
	t.Parallel()

	app := New()

	app.Domain("api.example.com").RouteChain("/api").RouteChain("/v1").
		Get(func(c Ctx) error {
			return c.SendString("v1")
		})

	req := httptest.NewRequest(MethodGet, "/api/v1", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "v1", string(body))
}

func Test_Domain_RouteChainAllMethods(t *testing.T) {
	t.Parallel()

	app := New()

	rc := app.Domain("api.example.com").RouteChain("/test")
	rc.All(func(c Ctx) error {
		c.Set("X-All", "yes")
		return c.Next()
	})
	rc.Head(func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})
	rc.Put(func(c Ctx) error {
		return c.SendString("put")
	})
	rc.Delete(func(c Ctx) error {
		return c.SendString("delete")
	})
	rc.Connect(func(c Ctx) error {
		return c.SendString("connect")
	})
	rc.Options(func(c Ctx) error {
		return c.SendString("options")
	})
	rc.Trace(func(c Ctx) error {
		return c.SendString("trace")
	})
	rc.Patch(func(c Ctx) error {
		return c.SendString("patch")
	})
	rc.Query(func(c Ctx) error {
		return c.SendString("query")
	})

	for _, method := range []string{MethodPut, MethodDelete, MethodPatch, MethodOptions, MethodQuery} {
		req := httptest.NewRequest(method, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	}
}

func Test_parseDomainPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pattern    string
		paramNames []string
		numParts   int
	}{
		{name: "simple", pattern: "example.com", numParts: 2},
		{name: "with subdomain", pattern: "api.example.com", numParts: 3},
		{name: "single param", pattern: ":sub.example.com", numParts: 3, paramNames: []string{"sub"}},
		{name: "multiple params", pattern: ":sub.:region.example.com", numParts: 4, paramNames: []string{"sub", "region"}},
		{name: "case insensitive", pattern: "API.Example.COM", numParts: 3},
		{name: "single part", pattern: "localhost", numParts: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := parseDomainPattern(tt.pattern)
			require.Equal(t, tt.numParts, m.numParts)
			if tt.paramNames == nil {
				require.Empty(t, m.paramNames)
			} else {
				require.Equal(t, tt.paramNames, m.paramNames)
			}
		})
	}
}

func Test_domainMatcher_match(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		hostname string
		values   []string
		matched  bool
	}{
		{name: "exact match", pattern: "api.example.com", hostname: "api.example.com", matched: true},
		{name: "case mismatch", pattern: "api.example.com", hostname: "API.EXAMPLE.COM", matched: true},
		{name: "wrong subdomain", pattern: "api.example.com", hostname: "www.example.com"},
		{name: "wrong part count", pattern: "api.example.com", hostname: "example.com"},
		{name: "with param", pattern: ":sub.example.com", hostname: "api.example.com", matched: true, values: []string{"api"}},
		{name: "multi param", pattern: ":a.:b.com", hostname: "x.y.com", matched: true, values: []string{"x", "y"}},
		{name: "param no match const", pattern: ":a.example.com", hostname: "x.other.com"},
		{name: "empty hostname", pattern: "example.com", hostname: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := parseDomainPattern(tt.pattern)
			matched, values := m.match(tt.hostname)
			require.Equal(t, tt.matched, matched)
			if tt.values == nil {
				if matched && len(m.paramIdx) == 0 {
					require.Empty(t, values)
				}
			} else {
				require.Equal(t, tt.values, values)
			}
		})
	}
}

// Test_Domain_HandlerTypes verifies that the domain router is compatible with
// all handler types defined in adapter.go.
func Test_Domain_HandlerTypes(t *testing.T) {
	t.Parallel()

	t.Run("fiber handler", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", func(c Ctx) error {
			return c.SendString("fiber")
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "fiber", string(body))
	})

	t.Run("fiber handler no error", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", func(c Ctx) {
			c.Set("X-Handler", "no-error")
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "no-error", resp.Header.Get("X-Handler"))
	})

	t.Run("express req res error", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", func(_ Req, res Res) error {
			return res.SendString("express-err")
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "express-err", string(body))
	})

	t.Run("express req res no error", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", func(_ Req, res Res) {
			res.Set("X-Express", "ok")
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "ok", resp.Header.Get("X-Express"))
	})

	t.Run("express next-err returns-err", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func() error) error {
				res.Set("X-MW", "yes")
				return next()
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "chained", string(body))
	})

	t.Run("express with next error", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func() error) {
				res.Set("X-MW", "yes")
				_ = next() //nolint:errcheck // test
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
	})

	t.Run("express with noarg next error", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func()) error {
				res.Set("X-MW", "yes")
				next()
				return nil
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
	})

	t.Run("express with noarg next", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func()) {
				res.Set("X-MW", "yes")
				next()
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
	})

	t.Run("express with error next", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func(error)) {
				res.Set("X-MW", "yes")
				next(nil)
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
	})

	t.Run("express with error next error", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func(error)) error {
				res.Set("X-MW", "yes")
				next(nil)
				return nil
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
	})

	t.Run("express errnext-err returns-err", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func(error) error) {
				res.Set("X-MW", "yes")
				_ = next(nil) //nolint:errcheck // test
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
	})

	t.Run("express errnext-err returns-err err", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(_ Req, res Res, next func(error) error) error {
				res.Set("X-MW", "yes")
				return next(nil)
			},
			func(c Ctx) error {
				return c.SendString("chained")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "yes", resp.Header.Get("X-MW"))
	})

	t.Run("net/http HandlerFunc", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("net/http func handler", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("fasthttp RequestHandler", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("fasthttp")
		}))
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("fasthttp handler with error", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/test", func(ctx *fasthttp.RequestCtx) error {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("fasthttp-err")
			return nil
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	// Verify non-matching domain doesn't execute any handler type
	t.Run("non-matching domain skips all handler types", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get(
			"/test",
			func(c Ctx) error {
				c.Set("X-Handler", "ran")
				return c.SendString("should-not-run")
			},
		)
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "wrong.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.Empty(t, resp.Header.Get("X-Handler"))
	})
}

// Test_Domain_UseHandlerTypes verifies that Use() is compatible with all handler types.
func Test_Domain_UseHandlerTypes(t *testing.T) {
	t.Parallel()

	t.Run("fiber handler middleware", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Use(func(c Ctx) error {
			c.Set("X-MW", "fiber")
			return c.Next()
		})
		domain.Get("/test", func(c Ctx) error {
			return c.SendString("ok")
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "fiber", resp.Header.Get("X-MW"))
	})

	t.Run("express middleware", func(t *testing.T) {
		t.Parallel()
		app := New()
		domain := app.Domain("api.example.com")
		domain.Use(func(_ Req, res Res, next func() error) error {
			res.Set("X-MW", "express")
			return next()
		})
		domain.Get("/test", func(c Ctx) error {
			return c.SendString("ok")
		})
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, "express", resp.Header.Get("X-MW"))
	})
}

func Benchmark_Domain_Match(b *testing.B) {
	m := parseDomainPattern(":tenant.api.example.com")
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		m.match("acme.api.example.com")
	}
}

func Benchmark_Domain_Route(b *testing.B) {
	app := New()

	app.Domain("api.example.com").Get("/test", func(c Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
	req.Host = "api.example.com"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // benchmark
	}
}

func Benchmark_Domain_NoImpact(b *testing.B) {
	// Benchmark regular routes to ensure domain feature has zero impact
	app := New()

	app.Get("/test", func(c Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(MethodGet, "/test", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // benchmark
	}
}

// Test_Domain_Security_EmptyPattern tests that empty domain patterns are rejected
func Test_Domain_Security_EmptyPattern(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		parseDomainPattern("")
	})

	require.Panics(t, func() {
		parseDomainPattern("   ")
	})
}

// Test_Domain_Security_EmptyLabel tests that domain patterns with empty labels are rejected
func Test_Domain_Security_EmptyLabel(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		parseDomainPattern("example..com")
	})

	require.Panics(t, func() {
		parseDomainPattern(".example.com")
	})
}

// Test_Domain_Security_EmptyParamName tests that empty parameter names are rejected
func Test_Domain_Security_EmptyParamName(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		parseDomainPattern(":.example.com")
	})
}

// Test_Domain_Security_InvalidParamName tests that invalid parameter names are rejected
func Test_Domain_Security_InvalidParamName(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		parseDomainPattern(":user@host.example.com")
	})

	require.Panics(t, func() {
		parseDomainPattern(":user$.example.com")
	})

	require.Panics(t, func() {
		parseDomainPattern(":user name.example.com")
	})
}

// Test_Domain_Security_InvalidDomainChars tests that invalid domain characters are rejected
func Test_Domain_Security_InvalidDomainChars(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		parseDomainPattern("example$.com")
	})

	require.Panics(t, func() {
		parseDomainPattern("example@domain.com")
	})

	require.Panics(t, func() {
		parseDomainPattern("example domain.com")
	})
}

// Test_Domain_Security_TooManyParts tests DoS protection against excessive domain labels
func Test_Domain_Security_TooManyParts(t *testing.T) {
	t.Parallel()

	// Pattern with too many parts should panic
	require.Panics(t, func() {
		parts := make([]string, 20)
		for i := range parts {
			parts[i] = "sub"
		}
		parseDomainPattern(strings.Join(parts, "."))
	})
}

// Test_Domain_Security_TooManyPartsRuntime tests DoS protection against excessive hostname labels at runtime
func Test_Domain_Security_TooManyPartsRuntime(t *testing.T) {
	t.Parallel()

	app := New()
	app.Domain("example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Hostname with too many labels should not match (DoS protection)
	parts := make([]string, 20)
	for i := range parts {
		parts[i] = "sub"
	}
	maliciousHost := strings.Join(parts, ".")

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = maliciousHost
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_Security_ExcessiveHostnameLength tests DoS protection against very long hostnames
func Test_Domain_Security_ExcessiveHostnameLength(t *testing.T) {
	t.Parallel()

	app := New()
	app.Domain("example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Hostname exceeding 253 characters should not match (DoS protection)
	maliciousHost := strings.Repeat("a", 254) + ".com"

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = maliciousHost
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_Security_ExcessiveLabelLength tests DoS protection against very long labels
func Test_Domain_Security_ExcessiveLabelLength(t *testing.T) {
	t.Parallel()

	app := New()
	app.Domain("example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Label exceeding 63 characters should not match (DoS protection)
	maliciousHost := strings.Repeat("a", 64) + ".example.com"

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = maliciousHost
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_Security_InvalidHostnameChars tests that hostnames with invalid characters are rejected
func Test_Domain_Security_InvalidHostnameChars(t *testing.T) {
	t.Parallel()

	app := New()
	app.Domain("example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	// Test hostnames with invalid characters that can be tested
	// Note: Some invalid chars (like spaces) are rejected by httptest.NewRequest itself
	tests := []struct {
		host      string
		canCreate bool
	}{
		{"example$.com", true},
		{"example@domain.com", true},
		{"example\x00.com", true},
		{"example\n.com", true},
		{"example;.com", true},
		{"example/.com", true},
	}

	for _, tt := range tests {
		if tt.canCreate {
			req := httptest.NewRequest(MethodGet, "/", http.NoBody)
			req.Host = tt.host
			resp, err := app.Test(req)
			if err == nil {
				require.Equal(t, StatusNotFound, resp.StatusCode, "Should reject hostname: %s", tt.host)
			}
			// If there's an error, the validation happened at an earlier layer which is also acceptable
		}
	}
}

// Test_Domain_Security_EmptyHostnameLabel tests that hostnames with empty labels are rejected
func Test_Domain_Security_EmptyHostnameLabel(t *testing.T) {
	t.Parallel()

	app := New()
	app.Domain("example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "example..com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_Security_ValidParamNames tests that valid parameter names are accepted
func Test_Domain_Security_ValidParamNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
	}{
		{name: "alphanumeric", pattern: ":user123.example.com"},
		{name: "underscore", pattern: ":user_name.example.com"},
		{name: "hyphen", pattern: ":user-name.example.com"},
		{name: "mixed", pattern: ":user_123-name.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.NotPanics(t, func() {
				parseDomainPattern(tt.pattern)
			})
		})
	}
}

// Test_Domain_Security_NonASCIIRejected tests that non-ASCII characters are rejected
// in both domain labels and parameter names (DNS names are ASCII-only).
func Test_Domain_Security_NonASCIIRejected(t *testing.T) {
	t.Parallel()

	// Non-ASCII in constant labels
	require.Panics(t, func() {
		parseDomainPattern("ünïcödé.example.com")
	})

	// Non-ASCII in parameter names
	require.Panics(t, func() {
		parseDomainPattern(":üser.example.com")
	})
}

// Test_Domain_Security_NonASCIIHostnameRejected tests that non-ASCII hostnames
// are rejected at runtime (DNS names are ASCII-only).
func Test_Domain_Security_NonASCIIHostnameRejected(t *testing.T) {
	t.Parallel()

	app := New()
	app.Domain(":sub.example.com").Get("/", func(c Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Host = "ünïcödé.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountReusable verifies that mounting the same sub-app on
// multiple domain routers does not double-wrap handlers (the original sub-app
// is not mutated).
func Test_Domain_UseMountReusable(t *testing.T) {
	t.Parallel()

	app := New()
	subApp := New()

	subApp.Get("/data", func(c Ctx) error {
		return c.SendString("data response")
	})

	// Mount the same sub-app on two different domains
	app.Domain("api.example.com").Use("/v1", subApp)
	app.Domain("admin.example.com").Use("/v1", subApp)

	// Test first domain works
	req := httptest.NewRequest(MethodGet, "/v1/data", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "data response", string(body))

	// Test second domain works
	req = httptest.NewRequest(MethodGet, "/v1/data", http.NoBody)
	req.Host = "admin.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "data response", string(body))

	// Test wrong domain is rejected for both
	req = httptest.NewRequest(MethodGet, "/v1/data", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountRoutesAfterMount verifies that routes added to a sub-app
// after it has been mounted on a domain router are NOT domain-filtered (since
// mount clones routes at mount time).
func Test_Domain_UseMountRoutesAfterMount(t *testing.T) {
	t.Parallel()

	app := New()
	subApp := New()

	// Register a route BEFORE mounting
	subApp.Get("/before", func(c Ctx) error {
		return c.SendString("before mount")
	})

	// Mount on domain router
	app.Domain("api.example.com").Use("/api", subApp)

	// Register a route AFTER mounting — this will NOT be domain-filtered
	// because mount() clones routes at mount time.
	subApp.Get("/after", func(c Ctx) error {
		return c.SendString("after mount")
	})

	// Route registered before mount should be domain-filtered
	req := httptest.NewRequest(MethodGet, "/api/before", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "before mount", string(body))

	// Route registered before mount should be rejected on wrong domain
	req = httptest.NewRequest(MethodGet, "/api/before", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	// Route registered after mount on the original sub-app is NOT included
	// in the wrapper. Since the mount group references the wrapper, the
	// after-mount route is never expanded into the parent app.
	req = httptest.NewRequest(MethodGet, "/api/after", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountNested verifies that a sub-app that itself mounts another
// app can be mounted on a domain router: the nested routes are served under the
// domain and are filtered by host like any other domain-mounted route.
func Test_Domain_UseMountNested(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	subApp := New()
	subApp.Use("/v1", child)
	subApp.Get("/flat", func(c Ctx) error {
		return c.SendString("flat")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	// The nested route is reachable on the matching host
	req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))

	// The sub-app's own routes still work alongside the nested ones
	req = httptest.NewRequest(MethodGet, "/api/flat", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "flat", string(body))

	// Both are rejected on a non-matching host
	for _, path := range []string{"/api/v1/x", "/api/flat"} {
		req = httptest.NewRequest(MethodGet, path, http.NoBody)
		req.Host = "www.example.com"
		resp, err = app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode, path)
	}
}

// Test_Domain_UseMountNestedDeep verifies that mounts nested more than one
// level deep are expanded, and that domain parameters resolve inside them.
func Test_Domain_UseMountNestedDeep(t *testing.T) {
	t.Parallel()

	grandChild := New()
	grandChild.Get("/deep", func(c Ctx) error {
		return c.SendString("tenant=" + DomainParam(c, "tenant"))
	})

	child := New()
	child.Use("/v2", grandChild)

	subApp := New()
	subApp.Use("/v1", child)

	app := New()
	app.Domain(":tenant.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/v1/v2/deep", http.NoBody)
	req.Host = "acme.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "tenant=acme", string(body))

	req = httptest.NewRequest(MethodGet, "/api/v1/v2/deep", http.NoBody)
	req.Host = "acme.example.org"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountNestedMiddleware verifies that middleware of a nested
// sub-app is domain-filtered too, instead of running on every host.
func Test_Domain_UseMountNestedMiddleware(t *testing.T) {
	t.Parallel()

	var hits int
	child := New()
	child.Use(func(c Ctx) error {
		hits++
		return c.Next()
	})
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	subApp := New()
	subApp.Use("/v1", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)
	// Same path outside the domain, so the request is served instead of 404ing
	// and any nested middleware that leaked would run.
	app.Get("/api/v1/x", func(c Ctx) error {
		return c.SendString("fallback")
	})

	req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "fallback", string(body))
	require.Equal(t, 0, hits, "nested middleware must not run on a non-matching host")

	req = httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))
	require.Equal(t, 1, hits)
}

// Test_Domain_UseMountNestedAtRoot verifies the nested mount also works when
// both the domain mount and the nested mount sit at the root path.
func Test_Domain_UseMountNestedAtRoot(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	subApp := New()
	subApp.Use(child)

	app := New()
	app.Domain("api.example.com").Use(subApp)

	req := httptest.NewRequest(MethodGet, "/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))

	req = httptest.NewRequest(MethodGet, "/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountNestedFromGroup verifies a nested mount registered
// through a group's domain router picks up the group prefix.
func Test_Domain_UseMountNestedFromGroup(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/data", func(c Ctx) error {
		return c.SendString("data response")
	})

	subApp := New()
	subApp.Use("/v1", child)

	app := New()
	api := app.Group("/api")
	api.Domain("api.example.com").Use("/mnt", subApp)

	req := httptest.NewRequest(MethodGet, "/api/mnt/v1/data", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "data response", string(body))

	req = httptest.NewRequest(MethodGet, "/api/mnt/v1/data", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountNestedReusable verifies that a sub-app with a nested
// mount can be mounted on several domains and still be usable unfiltered
// elsewhere, i.e. the original sub-app's handlers are never domain-wrapped.
func Test_Domain_UseMountNestedReusable(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	subApp := New()
	subApp.Use("/v1", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)
	app.Domain("admin.example.com").Use("/api", subApp)

	for _, host := range []string{"api.example.com", "admin.example.com"} {
		req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
		req.Host = host
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode, host)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "nested x", string(body), host)
	}

	req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	// The original sub-app's handlers were never wrapped, so a plain mount of
	// it still serves on any host.
	plain := New()
	plain.Use("/api", subApp)

	req = httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "anything.example.net"
	resp, err = plain.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))
}

// Test_Domain_UseMountNestedOrder verifies the nested app's routes are expanded
// in the position its mount was registered at, so registration order still
// decides which route wins.
func Test_Domain_UseMountNestedOrder(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("child")
	})

	subApp := New()
	subApp.Get("/v1/x", func(c Ctx) error {
		return c.SendString("before mount")
	})
	subApp.Use("/v1", child)
	subApp.Get("/v1/y", func(c Ctx) error {
		return c.SendString("after mount")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	// Registered before the mount, so it takes precedence over the nested /x
	req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "before mount", string(body))

	// Routes registered after the mount are kept as well
	req = httptest.NewRequest(MethodGet, "/api/v1/y", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "after mount", string(body))
}

// Test_Domain_UseMountNestedParams verifies parameters and wildcards of a
// nested sub-app survive the prefixing done while expanding the mount.
func Test_Domain_UseMountNestedParams(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/user/:id", func(c Ctx) error {
		return c.SendString("id=" + c.Params("id"))
	})
	child.Get("/files/*", func(c Ctx) error {
		return c.SendString("file=" + c.Params("*"))
	})

	subApp := New()
	subApp.Use("/v1", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/v1/user/42", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "id=42", string(body))

	req = httptest.NewRequest(MethodGet, "/api/v1/files/a/b.txt", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "file=a/b.txt", string(body))
}

// Test_Domain_UseMountNestedDomainMount verifies that a sub-app whose own
// nested mount was registered on a domain router can itself be mounted, both
// on a domain router and plainly.
func Test_Domain_UseMountNestedDomainMount(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	subApp := New()
	subApp.Domain("api.example.com").Use("/v1", child)

	// Mounted on a matching domain: the host check applies twice, which is
	// satisfied by the same host.
	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))

	req = httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	// Mounted plainly: the sub-app's own domain filter still applies.
	plain := New()
	plain.Use("/api", subApp)

	req = httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err = plain.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))

	req = httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err = plain.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountNestedRestrictedMethods verifies that a sub-app which
// declares fewer request methods than the app it is mounted on can still be
// domain-mounted: the wrapper's stack has to stay indexable at the parent's
// method indexes.
func Test_Domain_UseMountNestedRestrictedMethods(t *testing.T) {
	t.Parallel()

	subApp := New(Config{RequestMethods: []string{MethodGet}})
	subApp.Get("/x", func(c Ctx) error {
		return c.SendString("restricted x")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "restricted x", string(body))

	req = httptest.NewRequest(MethodGet, "/api/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountNestedExtraMethods verifies that a sub-app can be
// domain-mounted on an app whose request methods go beyond the defaults: the
// parent reads the wrapper's stack at its own method indexes, so the wrapper
// has to be as long as the parent's method table.
func Test_Domain_UseMountNestedExtraMethods(t *testing.T) {
	t.Parallel()

	methods := append(append([]string{}, DefaultMethods...), "PURGE")
	app := New(Config{RequestMethods: methods})

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})
	subApp := New()
	subApp.Use("/v1", child)

	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))

	req = httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountNestedRequestMethods verifies that a nested sub-app
// listing its own request methods keeps each route under the method it was
// registered with, and that a shorter table does not cut the clone short.
func Test_Domain_UseMountNestedRequestMethods(t *testing.T) {
	t.Parallel()

	// Swap GET and POST, so the two apps' stack indexes disagree.
	methods := append([]string{}, DefaultMethods...)
	for i, method := range methods {
		switch method {
		case MethodGet:
			methods[i] = MethodPost
		case MethodPost:
			methods[i] = MethodGet
		}
	}

	child := New(Config{RequestMethods: methods})
	child.Get("/x", func(c Ctx) error {
		return c.SendString("get x")
	})
	child.Post("/x", func(c Ctx) error {
		return c.SendString("post x")
	})

	subApp := New()
	subApp.Use("/v1", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	for method, want := range map[string]string{MethodGet: "get x", MethodPost: "post x"} {
		req := httptest.NewRequest(method, "/api/v1/x", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode, method)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, want, string(body), method)
	}
}

// Test_Domain_UseMountAutoHead verifies that domain-mounted routes get their
// automatic HEAD companion, for the sub-app's own routes and for the routes of
// an app it has mounted, and that the HEAD route is domain-filtered too.
func Test_Domain_UseMountAutoHead(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/nested", func(c Ctx) error {
		return c.SendString("nested")
	})

	subApp := New()
	subApp.Use("/v1", child)
	subApp.Get("/flat", func(c Ctx) error {
		return c.SendString("flat")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	// HEAD is the first request, so the route has to exist after the single
	// startup pass a served app performs.
	for _, path := range []string{"/api/v1/nested", "/api/flat"} {
		req := httptest.NewRequest(MethodHead, path, http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode, path)

		req = httptest.NewRequest(MethodHead, path, http.NoBody)
		req.Host = "www.example.com"
		resp, err = app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode, path)
	}
}

// Test_Domain_UseMountAutoHeadDisabled verifies that a sub-app which turned
// automatic HEAD registration off does not get HEAD routes from the mount.
func Test_Domain_UseMountAutoHeadDisabled(t *testing.T) {
	t.Parallel()

	subApp := New(Config{DisableHeadAutoRegister: true})
	subApp.Get("/flat", func(c Ctx) error {
		return c.SendString("flat")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodHead, "/api/flat", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
}

// Test_Domain_UseMountCustomConstraint verifies that custom constraints of a
// domain-mounted app, and of an app it has mounted in turn, still validate once
// the routes are expanded into the parent.
func Test_Domain_UseMountCustomConstraint(t *testing.T) {
	t.Parallel()

	child := New()
	child.RegisterCustomConstraint(&onlyFooConstraint{})
	child.Get("/nested/:name<onlyfoo>", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	})

	subApp := New()
	subApp.RegisterCustomConstraint(&onlyBarConstraint{})
	subApp.Use("/v1", child)
	subApp.Get("/flat/:name<onlybar>", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	for path, want := range map[string]int{
		"/api/v1/nested/foo": StatusOK,
		"/api/v1/nested/bar": StatusNotFound,
		"/api/flat/bar":      StatusOK,
		"/api/flat/foo":      StatusNotFound,
	} {
		req := httptest.NewRequest(MethodGet, path, http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, want, resp.StatusCode, path)
	}
}

// Test_Domain_UseMountPerHostConfig verifies that two sub-apps mounted at the
// same path on different domains each keep their own error handler and view
// engine. Keyed by path alone they would displace each other.
func Test_Domain_UseMountPerHostConfig(t *testing.T) {
	t.Parallel()

	engineA := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, engineA.Load())

	engineB := &testTemplateEngine{path: "testdata3"}
	require.NoError(t, engineB.Load())

	subA := New(Config{
		Views: engineA,
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(591).SendString("a error")
		},
	})
	subA.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})
	subA.Get("/view", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	subB := New(Config{
		Views: engineB,
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(592).SendString("b error")
		},
	})
	subB.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})
	subB.Get("/view", func(c Ctx) error {
		return c.Render("hello_world.tmpl", Map{"Name": "b"})
	})

	app := New()
	app.Domain("a.example.com").Use("/api", subA)
	app.Domain("b.example.com").Use("/api", subB)

	req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "a.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 591, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "a error", string(body))

	req = httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "b.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 592, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "b error", string(body))

	req = httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "a.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>I'm Bruh</h1>", string(body))

	req = httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "b.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello b!</h1>", string(body))
}

// Test_Domain_UseMountConfigIgnoredOnOtherHost verifies that a domain-mounted
// sub-app's error handler does not answer for a host its pattern rejects.
func Test_Domain_UseMountConfigIgnoredOnOtherHost(t *testing.T) {
	t.Parallel()

	subApp := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(593).SendString("sub error")
		},
	})
	subApp.Get("/mounted", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)
	// A route of the parent, under the same prefix, on every host.
	app.Get("/api/own", func(_ Ctx) error {
		return errors.New("boom")
	})

	req := httptest.NewRequest(MethodGet, "/api/own", http.NoBody)
	req.Host = "www.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusInternalServerError, resp.StatusCode)

	// On a matching host the sub-app still governs its own prefix.
	req = httptest.NewRequest(MethodGet, "/api/mounted", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 593, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "sub error", string(body))
}

// Test_Domain_UseMountNestedErrorHandler verifies that the deepest mount
// covering the request wins, nested apps included.
func Test_Domain_UseMountNestedErrorHandler(t *testing.T) {
	t.Parallel()

	child := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(594).SendString("child error")
		},
	})
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(595).SendString("sub error")
		},
	})
	subApp.Use("/v1", child)
	subApp.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	for path, want := range map[string]int{"/api/v1/boom": 594, "/api/boom": 595} {
		req := httptest.NewRequest(MethodGet, path, http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, want, resp.StatusCode, path)
	}
}

// Test_Domain_UseMountLeavesSubAppIntact verifies that mounting a sub-app on a
// domain does not flatten the sub-app's own mounts, so it stays usable on its
// own afterwards.
func Test_Domain_UseMountLeavesSubAppIntact(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	subApp := New()
	subApp.Use("/v1", child)

	getIndex := subApp.methodInt(MethodGet)
	routesBefore := len(subApp.stack[getIndex])

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	require.Len(t, subApp.stack[getIndex], routesBefore, "the sub-app's own stack was rewritten")
	require.True(t, subApp.stack[getIndex][0].mount, "the sub-app's mount was expanded in place")

	// The sub-app still resolves its own mount when served on its own.
	resp, err = subApp.Test(httptest.NewRequest(MethodGet, "/v1/x", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "nested x", string(body))
}

// Test_Domain_UseMountViewsLayout verifies that a domain-mounted sub-app's
// ViewsLayout is applied when the caller passes no layout of its own.
func Test_Domain_UseMountViewsLayout(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	require.NoError(t, engine.Load())

	subApp := New(Config{Views: engine, ViewsLayout: "main.tmpl"})
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "Hello, World!"})
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1><h1>I'm main</h1>", string(body))
}

// Test_Domain_UseMountViewsError verifies that a render failure in a
// domain-mounted sub-app's engine is reported rather than falling through to
// the parent's views.
func Test_Domain_UseMountViewsError(t *testing.T) {
	t.Parallel()

	parentEngine := &testTemplateEngine{}
	require.NoError(t, parentEngine.Load())

	subApp := New(Config{Views: errorTemplateEngine{}})
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "Hello, World!"})
	})

	app := New(Config{Views: parentEngine})
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "errorTemplateEngine")
}

// Test_Domain_UseMountWithoutViews verifies that a domain-mounted sub-app with
// no view engine of its own leaves rendering to the parent.
func Test_Domain_UseMountWithoutViews(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	require.NoError(t, engine.Load())

	subApp := New()
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "from parent"})
	})

	app := New(Config{Views: engine})
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>from parent</h1>", string(body))
}

// Test_Domain_UseMountReloadViews verifies that ReloadViews reaches the view
// engine of a domain-mounted sub-app, which appList no longer holds.
func Test_Domain_UseMountReloadViews(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, engine.Load())

	subApp := New(Config{Views: engine})
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	require.NoError(t, app.ReloadViews())

	req := httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>I'm Bruh</h1>", string(body))
}

// Test_Domain_UseMountSelf verifies that mounting an app on its own domain
// router returns instead of deadlocking on the app's mutex.
func Test_Domain_UseMountSelf(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/x", func(c Ctx) error {
		return c.SendString("x")
	})

	done := make(chan struct{})

	var mountPanic any
	go func() {
		defer close(done)
		defer func() { mountPanic = recover() }()

		app.Domain("api.example.com").Use("/self", app)
	}()

	select {
	case <-done:
		require.Nil(t, mountPanic, "mounting an app on its own domain router panicked")
	case <-time.After(10 * time.Second):
		require.FailNow(t, "mounting an app on its own domain router did not return")
	}
}

// Test_Domain_UseMountViewsPathOnly verifies that a domain-mounted sub-app's
// views are chosen by the request path, not by the mount path appearing
// anywhere in the URL — a query string carrying it must not select them.
func Test_Domain_UseMountViewsPathOnly(t *testing.T) {
	t.Parallel()

	subEngine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, subEngine.Load())

	parentEngine := &testTemplateEngine{}
	require.NoError(t, parentEngine.Load())

	subApp := New(Config{Views: subEngine})
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	app := New(Config{Views: parentEngine})
	app.Domain("api.example.com").Use("/api", subApp)
	app.Get("/elsewhere", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "parent"})
	})

	req := httptest.NewRequest(MethodGet, "/elsewhere?next=/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>parent</h1>", string(body))
}

// Test_Domain_UseMountAutoHeadKeepsMiddleware verifies that a sub-app which
// does not serve HEAD gets no synthesized HEAD routes. Its middleware is
// registered for its own methods only, so a HEAD route built from the GET one
// would reach the handler with nothing in front of it.
func Test_Domain_UseMountAutoHeadKeepsMiddleware(t *testing.T) {
	t.Parallel()

	subApp := New(Config{RequestMethods: []string{MethodGet, MethodPost}})
	subApp.Use(func(c Ctx) error {
		return c.SendStatus(StatusUnauthorized)
	})
	subApp.Get("/secret", func(c Ctx) error {
		c.Set("X-Secret", "leaked")
		return c.SendString("secret")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/secret", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusUnauthorized, resp.StatusCode)

	// Twice: app.Test runs the startup process on every call, and the second
	// pass sees the mount already expanded into the parent's stack.
	for range 2 {
		req = httptest.NewRequest(MethodHead, "/api/secret", http.NoBody)
		req.Host = "api.example.com"
		resp, err = app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
		require.Empty(t, resp.Header.Get("X-Secret"))
	}
}

// Test_Domain_UseMountNestedAutoHeadDisabled verifies that a nested app which
// turned automatic HEAD routes off does not get them from the mount either.
func Test_Domain_UseMountNestedAutoHeadDisabled(t *testing.T) {
	t.Parallel()

	child := New(Config{DisableHeadAutoRegister: true})
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	subApp := New()
	subApp.Use("/v1", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodHead, "/api/v1/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
}

// Test_Domain_UseMountAfterOuterMount verifies that a domain mount registered
// on a sub-app after that sub-app was itself mounted is still found: the
// parent picks the metadata up when it discovers sub-apps at startup.
func Test_Domain_UseMountAfterOuterMount(t *testing.T) {
	t.Parallel()

	child := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(596).SendString("child error")
		},
	})
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	mid := New()

	app := New()
	app.Use("/mid", mid)
	// Registered after the outer mount, but before the app is served.
	mid.Domain("api.example.com").Use("/child", child)

	req := httptest.NewRequest(MethodGet, "/mid/child/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 596, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "child error", string(body))

	req = httptest.NewRequest(MethodGet, "/mid/child/boom", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountOverlappingPatterns verifies that when two domain mounts
// at one path have patterns that both match, each sub-app still handles the
// errors of its own routes.
func Test_Domain_UseMountOverlappingPatterns(t *testing.T) {
	t.Parallel()

	exact := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(597).SendString("exact error")
		},
	})
	exact.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	wildcard := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(598).SendString("wildcard error")
		},
	})
	wildcard.Get("/other", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("admin.example.com").Use("/api", exact)
	app.Domain(":tenant.example.com").Use("/api", wildcard)

	// admin.example.com matches both patterns; the route belongs to the first.
	req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "admin.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 597, resp.StatusCode)

	req = httptest.NewRequest(MethodGet, "/api/other", http.NoBody)
	req.Host = "acme.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 598, resp.StatusCode)
}

// Test_Domain_UseMountParametricPrefix verifies that a parametric mount prefix
// inside a domain-mounted sub-app still matches as a pattern once the routes
// have been prefixed twice.
func Test_Domain_UseMountParametricPrefix(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("version=" + c.Params("version"))
	})

	subApp := New()
	subApp.Use("/v1/:version", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/v1/42/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "version=42", string(body))

	req = httptest.NewRequest(MethodGet, "/api/v1/42/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountCaseInsensitivePath verifies that a domain mount
// registered with a differently-cased path still owns its config on a
// case-insensitive app, the way its routes do.
func Test_Domain_UseMountCaseInsensitivePath(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, engine.Load())

	subApp := New(Config{
		Views: engine,
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(599).SendString("sub error")
		},
	})
	subApp.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	app := New()
	app.Domain("api.example.com").Use("/API", subApp)

	req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 599, resp.StatusCode)

	req = httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>I'm Bruh</h1>", string(body))

	// Still host-scoped: another host gets neither the route nor the config.
	req = httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountRecordedOnce verifies that a domain mount reached through
// several layers of ordinary mounts is recorded once. Each app list already
// holds the descendants of the apps it lists, so the startup walk arrives at
// the same mount by every path that leads to it.
func Test_Domain_UseMountRecordedOnce(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})

	inner := New()
	inner.Domain("api.example.com").Use("/child", child)

	app := inner
	for range 6 {
		outer := New()
		outer.Use("/layer", app)
		app = outer
	}

	req := httptest.NewRequest(MethodGet, "/unrouted", http.NoBody)
	req.Host = "api.example.com"
	_, err := app.Test(req)
	require.NoError(t, err)

	require.Len(t, app.mountFields.domainAppList, 1)
}

// Test_Domain_UseMountReloadViewsBeforeStart verifies that ReloadViews reaches
// a domain-mounted app nested inside an ordinary mount before the app has been
// through its startup process.
func Test_Domain_UseMountReloadViewsBeforeStart(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, engine.Load())

	child := New(Config{Views: engine})
	child.Get("/view", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	mid := New()
	mid.Domain("api.example.com").Use("/child", child)

	app := New()
	app.Use("/mid", mid)

	require.NoError(t, app.ReloadViews())
}

// Test_Domain_UseMountOverlappingViews verifies that when two domain mounts at
// one path both match, the one that owns the route decides which views apply —
// a later overlapping mount's engine is not borrowed for it.
func Test_Domain_UseMountOverlappingViews(t *testing.T) {
	t.Parallel()

	rootEngine := &testTemplateEngine{}
	require.NoError(t, rootEngine.Load())

	wildcardEngine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, wildcardEngine.Load())

	// Registered first, owns the route, and configures no views of its own.
	exact := New()
	exact.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "exact"})
	})

	wildcard := New(Config{Views: wildcardEngine})
	wildcard.Get("/other", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	app := New(Config{Views: rootEngine})
	app.Domain("admin.example.com").Use("/api", exact)
	app.Domain(":tenant.example.com").Use("/api", wildcard)

	req := httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "admin.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>exact</h1>", string(body))
}

// Test_Domain_UseMountOverlappingViewsLaterMount verifies that the second of two
// mounts at one path renders through its own engine when it owns the route: the
// mounts are equally deep and both match the host, so only the route that ran
// tells them apart.
func Test_Domain_UseMountOverlappingViewsLaterMount(t *testing.T) {
	t.Parallel()

	rootEngine := &testTemplateEngine{}
	require.NoError(t, rootEngine.Load())

	exactEngine := &testTemplateEngine{}
	require.NoError(t, exactEngine.Load())

	wildcardEngine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, wildcardEngine.Load())

	exact := New(Config{Views: exactEngine})
	exact.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "exact"})
	})

	// Registered second, and owns the route this request runs.
	wildcard := New(Config{Views: wildcardEngine})
	wildcard.Get("/other", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	app := New(Config{Views: rootEngine})
	app.Domain("admin.example.com").Use("/api", exact)
	app.Domain(":tenant.example.com").Use("/api", wildcard)

	// bruh.tmpl only exists in the wildcard mount's engine, so rendering it at
	// all is what proves the engine of the mount that owns the route was used.
	req := httptest.NewRequest(MethodGet, "/api/other", http.NoBody)
	req.Host = "admin.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>I'm Bruh</h1>", string(body))
}

// Test_Domain_UseMountOverlappingErrorHandler verifies that two mounts at one
// path each answer for their own routes. They are equally deep and both match
// the host, so the request is only attributable through the route that ran.
func Test_Domain_UseMountOverlappingErrorHandler(t *testing.T) {
	t.Parallel()

	first := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(581).SendString("first")
	}})
	first.Get("/first", func(_ Ctx) error {
		return errors.New("boom")
	})

	second := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(582).SendString("second")
	}})
	second.Get("/second", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("admin.example.com").Use("/api", first)
	app.Domain(":tenant.example.com").Use("/api", second)

	for path, want := range map[string]int{"/api/first": 581, "/api/second": 582} {
		req := httptest.NewRequest(MethodGet, path, http.NoBody)
		req.Host = "admin.example.com" // matches both patterns
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, want, resp.StatusCode, path)
	}
}

// Test_Domain_UseMountOverlappingErrorHandlerAutoHead verifies that an automatic
// HEAD route resolves the same owner its GET route does, rather than falling
// back to the first mount that covers the path.
func Test_Domain_UseMountOverlappingErrorHandlerAutoHead(t *testing.T) {
	t.Parallel()

	first := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(583).SendString("first")
	}})
	first.Get("/first", func(_ Ctx) error {
		return errors.New("boom")
	})

	second := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(584).SendString("second")
	}})
	second.Get("/second", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("admin.example.com").Use("/api", first)
	app.Domain(":tenant.example.com").Use("/api", second)

	req := httptest.NewRequest(MethodHead, "/api/second", http.NoBody)
	req.Host = "admin.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 584, resp.StatusCode)
}

// Test_Domain_UseMountDeclinedHostKeepsRootErrorHandler verifies that a mount
// whose handlers declined the host does not answer for the request they passed
// on: the route it registered is the one that ran, but it served nothing.
func Test_Domain_UseMountDeclinedHostKeepsRootErrorHandler(t *testing.T) {
	t.Parallel()

	subApp := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(585).SendString("sub")
	}})
	subApp.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "other.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountViewsLayoutWithoutViews verifies that a domain mount
// configuring only a layout renders through the engine above it and keeps its
// layout, as an ordinary mount without an engine of its own does.
func Test_Domain_UseMountViewsLayoutWithoutViews(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	require.NoError(t, engine.Load())

	subApp := New(Config{ViewsLayout: "main.tmpl"})
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "Hello"})
	})

	app := New(Config{Views: engine})
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello</h1><h1>I'm main</h1>", string(body))
}

// Test_Domain_UseMountBehindPlainDescendant verifies that an app domain-mounted
// on an ordinary descendant of a domain-mounted app is found: those records live
// only on the app they were registered on, and ordinary mounting does not carry
// them upward.
func Test_Domain_UseMountBehindPlainDescendant(t *testing.T) {
	t.Parallel()

	inner := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(586).SendString("inner")
	}})
	inner.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	mid := New()
	outer := New()
	outer.Use("/mid", mid)
	mid.Domain("admin.example.com").Use("/child", inner)

	app := New()
	app.Domain(":tenant.example.com").Use("/api", outer)

	req := httptest.NewRequest(MethodGet, "/api/mid/child/boom", http.NoBody)
	req.Host = "admin.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 586, resp.StatusCode)

	// Both patterns have to match: the inner app is behind its own domain
	// router as well as the one it was reached through.
	req = httptest.NewRequest(MethodGet, "/api/mid/child/boom", http.NoBody)
	req.Host = "other.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_Domain_UseMountStartedSubApp verifies that a sub-app which has already
// expanded its own mounts keeps its children as the owners of their routes when
// it is domain-mounted afterwards, rather than being credited with all of them.
func Test_Domain_UseMountStartedSubApp(t *testing.T) {
	t.Parallel()

	child := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(587).SendString("child")
	}})
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(588).SendString("sub")
	}})
	subApp.Use("/mid", child)

	// Run the sub-app on its own first, which replaces its mount placeholders
	// with the child's routes.
	_, err := subApp.Test(httptest.NewRequest(MethodGet, "/mid/boom", http.NoBody))
	require.NoError(t, err)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/mid/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 587, resp.StatusCode)
}

// Test_Domain_UseMountLateOrdinaryDescendant verifies that an app mounted on a
// descendant after that descendant was itself mounted is still found: the
// flattened list is filled in as apps are mounted, so only the mount metadata
// of each app in turn is current.
func Test_Domain_UseMountLateOrdinaryDescendant(t *testing.T) {
	t.Parallel()

	child := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(589).SendString("child")
	}})
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	mid := New()
	outer := New()
	outer.Use("/mid", mid)
	mid.Use("/child", child) // mounted after mid itself was

	app := New()
	app.Domain("api.example.com").Use("/api", outer)

	req := httptest.NewRequest(MethodGet, "/api/mid/child/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 589, resp.StatusCode)
}

// Test_Domain_UseMountViewsSkipsPlainSibling verifies that a domain mount
// without an engine of its own renders through the engine enclosing it rather
// than through a plain mount at its own path, which did not serve the request.
func Test_Domain_UseMountViewsSkipsPlainSibling(t *testing.T) {
	t.Parallel()

	rootEngine := &testTemplateEngine{}
	require.NoError(t, rootEngine.Load())

	// Registered at the same path, and holds no template the domain mount asks
	// for: borrowing it would fail rather than render the wrong page.
	plainEngine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, plainEngine.Load())

	plain := New(Config{Views: plainEngine})
	plain.Get("/other", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	subApp := New()
	subApp.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "domain"})
	})

	app := New(Config{Views: rootEngine})
	app.Use("/api", plain)
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>domain</h1>", string(body))
}

// Test_Domain_UseMountInheritsEnclosingErrorHandler verifies that a domain
// mount configuring no error handler inherits the one of the app it is mounted
// inside, as an ordinarily nested mount does.
func Test_Domain_UseMountInheritsEnclosingErrorHandler(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	mid := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(590).SendString("mid")
	}})
	mid.Domain("api.example.com").Use("/child", child)

	app := New()
	app.Use("/mid", mid)

	req := httptest.NewRequest(MethodGet, "/mid/child/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 590, resp.StatusCode)
}

// Test_Domain_UseMountDropsPlainSiblingErrorHandler verifies that a domain
// mount configuring no error handler falls through to the app enclosing it
// rather than to a plain mount at its own path, which did not serve the
// request.
func Test_Domain_UseMountDropsPlainSiblingErrorHandler(t *testing.T) {
	t.Parallel()

	plain := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(592).SendString("plain")
	}})
	plain.Get("/other", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New()
	subApp.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(593).SendString("root")
	}})
	app.Use("/api", plain)
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 593, resp.StatusCode)
}

// Test_Domain_UseMountCyclicOrdinaryMounts verifies that domain-mounting an app
// whose ordinary mounts form a cycle terminates and still serves its routes.
func Test_Domain_UseMountCyclicOrdinaryMounts(t *testing.T) {
	t.Parallel()

	first := New()
	second := New()
	second.Get("/ping", func(c Ctx) error {
		return c.SendString("pong")
	})

	first.Use("/second", second)
	second.Use("/first", first)

	app := New()
	app.Domain("api.example.com").Use("/api", first)

	req := httptest.NewRequest(MethodGet, "/api/second/ping", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "pong", string(body))
}

// Test_Domain_UseMountAutoHeadPerMount verifies that a HEAD route one mounted
// app registers does not stand in for another's GET route at the same path:
// on a hostname the first app's pattern rejects, its route declines and the GET
// route needs an automatic companion of its own.
func Test_Domain_UseMountAutoHeadPerMount(t *testing.T) {
	t.Parallel()

	inner := New()
	inner.Head("/x", func(c Ctx) error {
		c.Set("X-Handler", "inner")
		return nil
	})

	outer := New()
	outer.Get("/x", func(c Ctx) error {
		c.Set("X-Handler", "outer")
		return nil
	})
	outer.Domain("admin.example.com").Use("/", inner)

	app := New()
	app.Domain(":tenant.example.com").Use("/api", outer)

	for host, want := range map[string]string{
		"admin.example.com": "inner", // matches both patterns
		"other.example.com": "outer", // only the outer one
	} {
		req := httptest.NewRequest(MethodHead, "/api/x", http.NoBody)
		req.Host = host
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode, host)
		require.Equal(t, want, resp.Header.Get("X-Handler"), host)
	}
}

// Test_Domain_UseMountInheritsDomainErrorHandler verifies that an app mounted
// inside a domain-mounted one inherits its error handler when it configures
// none, as an ordinarily nested mount does.
func Test_Domain_UseMountInheritsDomainErrorHandler(t *testing.T) {
	t.Parallel()

	child := New()
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(594).SendString("sub")
	}})
	subApp.Use("/child", child)

	app := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(595).SendString("root")
	}})
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/child/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 594, resp.StatusCode)
}

// Test_Domain_UseMountInheritsOnlyFromCoveringMounts verifies that inheritance
// reaches the mounts a request passed through, not every shallower one: a
// domain mount on another path encloses nothing here.
func Test_Domain_UseMountInheritsOnlyFromCoveringMounts(t *testing.T) {
	t.Parallel()

	other := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(596).SendString("other")
	}})
	other.Get("/thing", func(c Ctx) error {
		return c.SendString("thing")
	})

	child := New()
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New()
	subApp.Use("/child", child)

	app := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(597).SendString("root")
	}})
	app.Domain("api.example.com").Use("/other", other)
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/child/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 597, resp.StatusCode)
}

// Test_Domain_UseMountIgnoresOverlappingSibling verifies that inheritance
// follows the mounts an app is registered inside, not every shallower one that
// happens to reach the same request: a separately registered mount overlapping
// by host and path encloses nothing.
func Test_Domain_UseMountIgnoresOverlappingSibling(t *testing.T) {
	t.Parallel()

	wildcard := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(598).SendString("wildcard")
	}})
	wildcard.Get("/thing", func(c Ctx) error {
		return c.SendString("thing")
	})

	exact := New() // no handler of its own
	exact.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(599).SendString("root")
	}})
	app.Domain(":tenant.example.com").Use("/api", wildcard)
	app.Domain("admin.example.com").Use("/api/child", exact)

	req := httptest.NewRequest(MethodGet, "/api/child/boom", http.NoBody)
	req.Host = "admin.example.com" // matches both patterns
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 599, resp.StatusCode)
}

// Test_Domain_UseMountPlainRouteKeepsItsOwnConfig verifies that a domain mount
// does not answer for a route an ordinary mount served, even where it covers
// the same prefix on a matching host.
func Test_Domain_UseMountPlainRouteKeepsItsOwnConfig(t *testing.T) {
	t.Parallel()

	plain := New() // no handler of its own
	plain.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(601).SendString("domain")
	}})
	subApp.Get("/other", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(602).SendString("root")
	}})
	app.Use("/api", plain)
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 602, resp.StatusCode)

	// The domain mount still answers for the routes it did serve.
	req = httptest.NewRequest(MethodGet, "/api/other", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 601, resp.StatusCode)
}

// Test_Domain_UseMountConcurrentRegistration verifies that cloning a sub-app's
// routes does not race registration on that sub-app: a route registered on a
// path it already serves has its handlers appended in place.
func Test_Domain_UseMountConcurrentRegistration(t *testing.T) {
	t.Parallel()

	handler := func(c Ctx) error { return c.SendString("x") }
	subApp := New()
	subApp.Get("/x", handler)

	var wg sync.WaitGroup

	wg.Go(func() {
		for range 200 {
			subApp.Get("/x", handler)
		}
	})

	wg.Go(func() {
		for range 200 {
			New().Domain("api.example.com").Use("/api", subApp)
		}
	})

	wg.Wait()
}

// Test_Domain_UseMountNestedPrefixConstraint verifies that a constraint named in
// a nested mount prefix is still applied when the mount is recorded on the app
// above: the constraint is registered on the app that wrote the prefix, not on
// the one recording it.
func Test_Domain_UseMountNestedPrefixConstraint(t *testing.T) {
	t.Parallel()

	child := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(603).SendString("child")
	}})
	child.Get("/x", func(c Ctx) error {
		return c.SendString("x")
	})

	mid := New()
	mid.RegisterCustomConstraint(&onlyFooConstraint{})
	mid.Use("/:name<onlyfoo>", child)

	outer := New()
	outer.Use("/mid", mid)

	app := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(604).SendString("root")
	}})
	app.Domain("api.example.com").Use("/api", outer)

	// The mount covers the path the constraint accepts, and nothing else.
	for path, want := range map[string]int{"/api/mid/foo/nope": 603, "/api/mid/bar/nope": 604} {
		req := httptest.NewRequest(MethodGet, path, http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, want, resp.StatusCode, path)
	}
}

// Test_Domain_UseMountAncestryIsStable verifies that a mount reached both
// through its own parent and as an entry of the app above it keeps the ancestry
// of the former, whichever the unordered walk happens to record first.
func Test_Domain_UseMountAncestryIsStable(t *testing.T) {
	t.Parallel()

	for range 40 {
		child := New()
		child.Get("/boom", func(_ Ctx) error {
			return errors.New("boom")
		})

		mid := New(Config{ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(605).SendString("mid")
		}})
		mid.Use("/child", child)

		outer := New()
		outer.Use("/mid", mid)

		app := New(Config{ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(606).SendString("root")
		}})
		app.Domain("api.example.com").Use("/api", outer)

		req := httptest.NewRequest(MethodGet, "/api/mid/child/boom", http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 605, resp.StatusCode)
	}
}

// Test_Domain_UseMountLayoutStopsAtEngine verifies that a mount rendering
// through an engine of its own takes no layout from the mount enclosing it, as
// an ordinary mount does not: the scan stops at the first engine covering the
// request either way.
func Test_Domain_UseMountLayoutStopsAtEngine(t *testing.T) {
	t.Parallel()

	childEngine := &testTemplateEngine{}
	require.NoError(t, childEngine.Load())

	subEngine := &testTemplateEngine{}
	require.NoError(t, subEngine.Load())

	child := New(Config{Views: childEngine}) // an engine, and no layout
	child.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "child"})
	})

	subApp := New(Config{Views: subEngine, ViewsLayout: "main.tmpl"})
	subApp.Use("/child", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/child/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>child</h1>", string(body))
}

// Test_Domain_UseMountAutoHeadPerSubtree verifies that one app opting out of
// automatic HEAD routes withholds them from itself and the apps it is mounted
// in front of, and from nothing else — the routes of the app it sits inside
// keep theirs, as they do under an ordinary mount.
func Test_Domain_UseMountAutoHeadPerSubtree(t *testing.T) {
	t.Parallel()

	handler := func(c Ctx) error { return c.SendString("x") }

	child := New(Config{DisableHeadAutoRegister: true})
	child.Get("/deep", handler)

	subApp := New()
	subApp.Get("/top", handler)
	subApp.Use("/v1", child)

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)

	for path, want := range map[string]int{
		"/api/top":     StatusOK,
		"/api/v1/deep": StatusMethodNotAllowed,
	} {
		req := httptest.NewRequest(MethodHead, path, http.NoBody)
		req.Host = "api.example.com"
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, want, resp.StatusCode, path)
	}
}

// Test_Domain_UseMountInheritsEqualDepthAncestor verifies that a mount as deep
// as the owner is only passed over when it is beside it. A domain mount at the
// root of an ordinary one covers exactly the paths that mount does, and its
// error handler is still the one enclosing the request.
func Test_Domain_UseMountInheritsEqualDepthAncestor(t *testing.T) {
	t.Parallel()

	child := New() // no handler of its own
	child.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(607).SendString("sub")
	}})
	subApp.Domain("api.example.com").Use("/", child)

	app := New()
	app.Use("/mid", subApp)

	req := httptest.NewRequest(MethodGet, "/mid/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 607, resp.StatusCode)
}

// Test_Domain_UseMountTreeVisitsEachMountOnce verifies that recording a mount
// does not walk a descendant once per route through the app lists leading to
// it: those lists hold descendants, so an unguarded walk grows exponentially
// with the depth of the tree.
func Test_Domain_UseMountTreeVisitsEachMountOnce(t *testing.T) {
	t.Parallel()

	const depth = 12

	apps := make([]*App, depth+1)
	for i := range apps {
		apps[i] = New()
	}

	for i := depth; i > 0; i-- {
		apps[i-1].Use("/layer", apps[i])
	}

	// Quadratic at worst: an app is walked again only when it is reached
	// through a longer chain than one it has already been reached with.
	require.Less(t, len(apps[0].mountTree()), (depth+1)*(depth+1))
}

// Test_Domain_UseMountSharedAppOnTwoPatterns verifies that one app mounted at a
// path for two hostnames is recorded for both: the mounts differ only in the
// patterns that reach them.
func Test_Domain_UseMountSharedAppOnTwoPatterns(t *testing.T) {
	t.Parallel()

	shared := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(608).SendString("shared")
	}})
	shared.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	subApp := New()
	subApp.Domain("a.example.com").Use("/", shared)
	subApp.Domain("b.example.com").Use("/", shared)

	app := New()
	app.Domain(":sub.example.com").Use("/api", subApp)

	for _, host := range []string{"a.example.com", "b.example.com"} {
		req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
		req.Host = host
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 608, resp.StatusCode, host)
	}
}

// Test_Domain_UseMountOutranksDeeperUnrelatedMount verifies that the app a route
// was mounted from answers for it even where an unrelated plain mount reaches
// deeper: that mount serves none of these routes, and depth only stands in for
// ownership where nothing better is known.
func Test_Domain_UseMountOutranksDeeperUnrelatedMount(t *testing.T) {
	t.Parallel()

	subApp := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(609).SendString("domain")
	}})
	subApp.Get("/admin/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	plain := New(Config{ErrorHandler: func(c Ctx, _ error) error {
		return c.Status(610).SendString("plain")
	}})
	plain.Get("/other", func(c Ctx) error {
		return c.SendString("other")
	})

	app := New()
	app.Domain("api.example.com").Use("/api", subApp)
	app.Use("/api/admin", plain)

	req := httptest.NewRequest(MethodGet, "/api/admin/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 609, resp.StatusCode)

	// The deeper mount still answers for the routes it does serve.
	req = httptest.NewRequest(MethodGet, "/api/admin/missing", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 610, resp.StatusCode)
}

// Test_Domain_UseMountInheritsDomainViews verifies the same inheritance for the
// view engine: a child of a domain-mounted app renders through that app's
// engine rather than the root's.
func Test_Domain_UseMountInheritsDomainViews(t *testing.T) {
	t.Parallel()

	// Holds no template the child asks for, so borrowing it would fail.
	rootEngine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, rootEngine.Load())

	subEngine := &testTemplateEngine{}
	require.NoError(t, subEngine.Load())

	child := New()
	child.Get("/view", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "child"})
	})

	subApp := New(Config{Views: subEngine})
	subApp.Use("/child", child)

	app := New(Config{Views: rootEngine})
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodGet, "/api/child/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>child</h1>", string(body))
}

// Test_Domain_UseMountKeepsParentAutoHead verifies that a domain mount which
// registers no automatic HEAD routes only withholds them from its own routes,
// leaving a route the parent registered at the same path with its own.
func Test_Domain_UseMountKeepsParentAutoHead(t *testing.T) {
	t.Parallel()

	subApp := New(Config{DisableHeadAutoRegister: true})
	subApp.Get("/x", func(c Ctx) error {
		return c.SendString("sub x")
	})

	app := New()
	app.Get("/api/x", func(c Ctx) error {
		return c.SendString("parent x")
	})
	app.Domain("api.example.com").Use("/api", subApp)

	req := httptest.NewRequest(MethodHead, "/api/x", http.NoBody)
	req.Host = "www.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode, "the parent's own route keeps its HEAD route")
}

// Test_Domain_UseMountParametricMountPath verifies that a domain mount
// registered at a parametric path owns the requests its routes serve, so its
// error handler and views apply to them.
func Test_Domain_UseMountParametricMountPath(t *testing.T) {
	t.Parallel()

	subApp := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(571).SendString("sub error")
		},
	})
	subApp.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("api.example.com").Use("/:tenant", subApp)

	req := httptest.NewRequest(MethodGet, "/acme/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 571, resp.StatusCode)
}

// Test_Domain_UseMountRootIsShallowest verifies that a mount at the root is
// the least specific of the mounts covering a request, so a named mount
// registered after it still owns its own routes.
func Test_Domain_UseMountRootIsShallowest(t *testing.T) {
	t.Parallel()

	rootApp := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(572).SendString("root mount")
		},
	})
	rootApp.Get("/other", func(_ Ctx) error {
		return errors.New("boom")
	})

	apiApp := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(573).SendString("api mount")
		},
	})
	apiApp.Get("/boom", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Domain("api.example.com").Use("/", rootApp)
	app.Domain("api.example.com").Use("/api", apiApp)

	req := httptest.NewRequest(MethodGet, "/api/boom", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 573, resp.StatusCode)
}

// Test_Domain_UseMountViewsRankedAgainstPlain verifies that a domain mount and
// an ordinary one are ranked against each other by mount depth, so the deeper
// of the two renders with its own engine.
func Test_Domain_UseMountViewsRankedAgainstPlain(t *testing.T) {
	t.Parallel()

	domainEngine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, domainEngine.Load())

	plainEngine := &testTemplateEngine{path: "testdata3"}
	require.NoError(t, plainEngine.Load())

	domainApp := New(Config{Views: domainEngine})
	domainApp.Get("/x", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	plainApp := New(Config{Views: plainEngine})
	plainApp.Get("/view", func(c Ctx) error {
		return c.Render("hello_world.tmpl", Map{"Name": "deep"})
	})

	app := New()
	app.Domain("api.example.com").Use("/api", domainApp)
	app.Use("/api/admin", plainApp)

	// The deeper plain mount owns this one.
	req := httptest.NewRequest(MethodGet, "/api/admin/view", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello deep!</h1>", string(body))

	// And the domain mount still owns its own.
	req = httptest.NewRequest(MethodGet, "/api/x", http.NoBody)
	req.Host = "api.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>I'm Bruh</h1>", string(body))
}

// Test_Domain_UseMountNestedCycle verifies that cloning a sub-app whose mount
// graph contains a cycle terminates, and stops at the point the cycle closes.
// It drives the clone directly: such an app cannot be served, because the
// parent's startup loops in appendSubAppLists, independently of domain routing.
func Test_Domain_UseMountNestedCycle(t *testing.T) {
	t.Parallel()

	first := New()
	second := New()
	second.Get("/x", func(c Ctx) error {
		return c.SendString("nested x")
	})
	first.Use("/second", second)
	second.Use("/first", first)

	app := New()
	d := &domainRouter{app: app, matcher: parseDomainPattern("api.example.com")}
	wrapper := New()
	d.cloneRoutesForDomain(wrapper, first)

	// second's route is cloned once, under the prefix it was mounted at, and
	// the mount that closes the cycle back onto first contributes nothing.
	var paths []string
	for _, route := range wrapper.stack[app.methodInt(MethodGet)] {
		paths = append(paths, route.path)
	}
	require.Equal(t, []string{"/second/x"}, paths)
}

// Test_Domain_Security_PatternLengthLimits verifies RFC 1035 length limits
// are enforced for domain patterns (253 total, 63 per label).
func Test_Domain_Security_PatternLengthLimits(t *testing.T) {
	t.Parallel()

	// Pattern exceeding 253 total characters
	t.Run("total length exceeds 253", func(t *testing.T) {
		t.Parallel()
		// 250 chars of 'a' + ".com" = 254 chars total > 253
		longPattern := strings.Repeat("a", 250) + ".com"
		require.Greater(t, len(longPattern), 253)
		require.Panics(t, func() {
			parseDomainPattern(longPattern)
		})
	})

	// Single label exceeding 63 characters
	t.Run("label exceeds 63 chars", func(t *testing.T) {
		t.Parallel()
		longLabel := strings.Repeat("a", 64)
		require.Panics(t, func() {
			parseDomainPattern(longLabel + ".example.com")
		})
	})

	// Pattern at exactly 253 characters should not panic
	t.Run("253 chars total is valid", func(t *testing.T) {
		t.Parallel()
		label63 := strings.Repeat("a", 63)
		pattern := label63 + "." + label63 + "." + label63 + "." + strings.Repeat("b", 59)
		require.LessOrEqual(t, len(pattern), 253)
		require.NotPanics(t, func() {
			parseDomainPattern(pattern)
		})
	})

	// Label at exactly 63 characters should not panic
	t.Run("63 char label is valid", func(t *testing.T) {
		t.Parallel()
		label63 := strings.Repeat("a", 63)
		require.NotPanics(t, func() {
			parseDomainPattern(label63 + ".com")
		})
	})
}

func Test_Domain_OpenAPI_Helpers_Advanced(t *testing.T) {
	t.Parallel()

	t.Run("Security", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).
			Security(map[string][]string{"bearerAuth": {}})
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Len(t, route.Security, 1)
		require.Contains(t, route.Security[0], "bearerAuth")
	})

	t.Run("Hidden", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).Hidden()
		route := app.stack[app.methodInt(MethodGet)][0]
		require.True(t, route.IsHidden())
	})

	t.Run("ResponseHeader", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).
			ResponseHeader(StatusOK, "X-Rate-Limit", "requests per hour", map[string]any{"type": "integer"})
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Contains(t, route.Responses["200"].Headers, "X-Rate-Limit")
	})

	t.Run("AddParameter", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).
			AddParameter(RouteParameter{Name: "limit", In: "query", Schema: map[string]any{"type": "integer"}})
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Len(t, route.Parameters, 1)
		require.Equal(t, "limit", route.Parameters[0].Name)
		require.Equal(t, "query", route.Parameters[0].In)
	})

	t.Run("OperationExternalDocs", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).
			OperationExternalDocs("More info", "https://example.com/docs")
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, "https://example.com/docs", route.ExternalDocs["url"])
		require.Equal(t, "More info", route.ExternalDocs["description"])
	})

	t.Run("RequestBodyContent", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Post("/users", testEmptyHandler).
			RequestBodyContent("User", true, map[string]RouteMediaType{
				MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
			})
		route := app.stack[app.methodInt(MethodPost)][0]
		require.NotNil(t, route.RequestBody)
		require.Contains(t, route.RequestBody.Content, MIMEApplicationJSON)
	})

	t.Run("ResponseContent", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).
			ResponseContent(StatusOK, "OK", map[string]RouteMediaType{
				MIMEApplicationJSON: {Schema: map[string]any{"type": "array"}},
			})
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Contains(t, route.Responses["200"].Content, MIMEApplicationJSON)
	})

	t.Run("ResponseLink", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).
			ResponseLink(StatusOK, "self", map[string]any{"operationId": "getUsers"})
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Contains(t, route.Responses["200"].Links, "self")
	})

	t.Run("OperationExtension", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example.com").Get("/users", testEmptyHandler).
			OperationExtension(map[string]any{"x-internal": true})
		route := app.stack[app.methodInt(MethodGet)][0]
		require.Equal(t, true, route.OperationExtensions["x-internal"])
	})
}

func Test_Domain_RouteChain_OpenAPI_Helpers(t *testing.T) {
	t.Parallel()
	app := New()
	domain := app.Domain("api.example.com")

	domain.RouteChain("/users").Post(testEmptyHandler).
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
		Tags("users", "write").
		Deprecated().
		Security(map[string][]string{"bearerAuth": {}}).
		OperationExternalDocs("docs", "https://example.com/docs").
		OperationExtension(map[string]any{"x-team": "core"})

	post := app.stack[app.methodInt(MethodPost)][0]
	require.Equal(t, "createUser", post.Name)
	require.Equal(t, "Create a user", post.Summary)
	require.Equal(t, "Creates a new user", post.Description)
	//nolint:testifylint // MIME type string, not a JSON payload
	require.Equal(t, MIMEApplicationJSON, post.Consumes)
	require.Equal(t, MIMEApplicationXML, post.Produces)
	require.Len(t, post.Parameters, 3)
	require.Contains(t, post.Responses, "201")
	require.Contains(t, post.Responses, "202")
	require.Contains(t, post.Responses["201"].Headers, "Location")
	require.Contains(t, post.Responses["200"].Content, MIMEApplicationJSON)
	require.Contains(t, post.Responses["201"].Links, "self")
	require.Equal(t, []string{"users", "write"}, post.Tags)
	require.True(t, post.Deprecated)
	require.Len(t, post.Security, 1)
	require.Equal(t, "https://example.com/docs", post.ExternalDocs["url"])
	require.Equal(t, "core", post.OperationExtensions["x-team"])

	domain.RouteChain("/rb-plain").Put(testEmptyHandler).RequestBody("Body", true, MIMEApplicationJSON)
	require.Equal(t, []string{MIMEApplicationJSON}, app.stack[app.methodInt(MethodPut)][0].RequestBody.MediaTypes)

	domain.RouteChain("/rb-example").Patch(testEmptyHandler).
		RequestBodyWithExample("Body", true, nil, "#/components/schemas/User", nil, nil, MIMEApplicationJSON)
	require.Equal(t, "#/components/schemas/User", app.stack[app.methodInt(MethodPatch)][0].RequestBody.SchemaRef)

	domain.RouteChain("/rb-content").Delete(testEmptyHandler).
		RequestBodyContent("Body", true, map[string]RouteMediaType{MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}})
	require.Contains(t, app.stack[app.methodInt(MethodDelete)][0].RequestBody.Content, MIMEApplicationJSON)

	domain.RouteChain("/secret").Get(testEmptyHandler).Hidden()
	require.True(t, app.stack[app.methodInt(MethodGet)][0].IsHidden())
}

func Test_Domain_Group_Use_EmptyHandlers(t *testing.T) {
	t.Parallel()
	app := New()
	dg := app.Domain("api.example.com").Group("/api")
	dg.Use("/sub")
	dg.Get("/users", func(c Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(MethodGet, "/api/users", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))
}

func Test_Domain_Use_InvalidHandler(t *testing.T) {
	t.Parallel()
	app := New()
	require.Panics(t, func() {
		app.Domain("api.example.com").Use(12345)
	})
}

func Test_Domain_Group_HookError(t *testing.T) {
	t.Parallel()
	app := New()
	app.Hooks().OnGroup(func(Group) error { return errTestDomainHook })
	require.PanicsWithValue(t, errTestDomainHook, func() {
		app.Domain("api.example.com").Group("/api")
	})
}

func Test_Domain_Mount_HookError(t *testing.T) {
	t.Parallel()
	app := New()
	sub := New()
	sub.Hooks().OnMount(func(*App) error { return errTestDomainHook })
	require.PanicsWithValue(t, errTestDomainHook, func() {
		app.Domain("api.example.com").Use("/api", sub)
	})
}

func Test_Domain_AutoHead_PerDomain(t *testing.T) {
	t.Parallel()

	app := New()
	app.Domain("a.example.com").Get("/x", func(c Ctx) error { return c.SendString("A") })
	app.Domain("b.example.com").Get("/x", func(c Ctx) error { return c.SendString("B") })

	for _, host := range []string{"a.example.com", "b.example.com"} {
		req := httptest.NewRequest(MethodHead, "http://"+host+"/x", http.NoBody)
		req.Host = host
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equalf(t, StatusOK, resp.StatusCode, "HEAD %s", host)
	}
}
