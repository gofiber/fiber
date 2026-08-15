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

	require.NotPanics(t, func() {
		app.Domain("api.example.com").Use("/self", app)
	})
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
