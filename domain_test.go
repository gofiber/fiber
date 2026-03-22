// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

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

	app.Domain("api.example.com").Get("/test",
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

	for _, method := range []string{MethodPut, MethodDelete, MethodPatch, MethodOptions} {
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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
		app.Domain("api.example.com").Get("/test",
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

// Test_Domain_Security_PatternLengthLimits verifies RFC 1035 length limits
// are enforced for domain patterns (253 total, 63 per label).
func Test_Domain_Security_PatternLengthLimits(t *testing.T) {
	t.Parallel()

	// Pattern exceeding 253 total characters
	t.Run("total length exceeds 253", func(t *testing.T) {
		t.Parallel()
		// Build a valid-looking pattern that exceeds 253 chars
		longPattern := strings.Repeat("a.", 127) + "com"
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
