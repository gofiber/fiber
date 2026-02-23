// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
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

func Test_Domain_UseMountPanic(t *testing.T) {
	t.Parallel()

	app := New()
	subApp := New()

	require.Panics(t, func() {
		app.Domain("api.example.com").Use(subApp)
	})
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
