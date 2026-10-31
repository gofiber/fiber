// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// domainTestExpect sends a GET request for path with the given Host through app
// and fails the test unless the response has the given status. The response is
// returned for assertions on its headers.
func domainTestExpect(t *testing.T, app *App, host, path string, status int) *http.Response {
	t.Helper()

	return routerDomainExpectMethod(t, app, MethodGet, host, path, status)
}

// routerDomainExpectMethod is domainTestExpect for a request of the given method.
func routerDomainExpectMethod(t *testing.T, app *App, method, host, path string, status int) *http.Response {
	t.Helper()

	return apptest.Expect(t, app, apptest.Req(method, path, apptest.WithHost(host)), status)
}

// domainTestExpectBody is domainTestExpect that also checks the response body.
func domainTestExpectBody(t *testing.T, app *App, host, path string, status int, body string) {
	t.Helper()

	apptest.ExpectBody(t, app, apptest.Req(MethodGet, path, apptest.WithHost(host)), status, body)
}

// domainTestApp returns a new app, configured with cfg, whose only route
// answers GET path with body.
func domainTestApp(path, body string, cfg ...Config) *App {
	app := New(cfg...)
	app.Get(path, func(c Ctx) error { return c.SendString(body) })
	return app
}

// domainTestRenderApp returns a new app, configured with cfg, whose only route
// answers GET path by rendering the template tmpl with data.
func domainTestRenderApp(path, tmpl string, data Map, cfg ...Config) *App {
	app := New(cfg...)
	app.Get(path, func(c Ctx) error { return c.Render(tmpl, data) })
	return app
}

// domainTestBoom is a handler that always fails, to reach the error handler of
// the app it is registered on.
func domainTestBoom(_ Ctx) error {
	return errors.New("boom")
}

// domainTestErrorHandler returns an error handler that answers every error with
// the given status and body, so a test can tell whose handler ran.
func domainTestErrorHandler(status int, body string) ErrorHandler {
	return func(c Ctx, _ error) error { return c.Status(status).SendString(body) }
}

// domainTestFailingApp returns a new app whose only route, GET path, fails and
// whose error handler answers with the given status and body.
func domainTestFailingApp(status int, body, path string) *App {
	app := New(Config{ErrorHandler: domainTestErrorHandler(status, body)})
	app.Get(path, domainTestBoom)
	return app
}

// routerDomainMount returns a new app, configured with cfg, that serves sub under /api on api.example.com.
func routerDomainMount(sub *App, cfg ...Config) *App {
	app := New(cfg...)
	app.Domain("api.example.com").Use("/api", sub)
	return app
}

// routerDomainWith returns a new app that has sub mounted under prefix.
func routerDomainWith(prefix string, sub *App) *App {
	app := New()
	app.Use(prefix, sub)
	return app
}

// domainTestEngine returns a loaded template engine for the templates in
// .github/<dir>, or for the default set when dir is empty.
func domainTestEngine(t *testing.T, dir string) *testTemplateEngine {
	t.Helper()

	engine := &testTemplateEngine{path: dir}
	require.NoError(t, engine.Load())
	return engine
}

// Test_Domain_Routing registers routes through the domain router API on a new
// app per row and sends the row's requests, each with its Host.
func Test_Domain_Routing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		register func(app *App)
		name     string
		checks   []routerReq
	}{
		{name: "basic", register: func(app *App) {
			app.Domain("api.example.com").Get("/hello", func(c Ctx) error { return c.SendString("api hello") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/hello", status: StatusOK, body: "api hello"},
			{host: "www.example.com", path: "/hello", status: StatusNotFound},
		}},
		{name: "multiple domains", register: func(app *App) {
			app.Domain("api.example.com").Get("/", func(c Ctx) error { return c.SendString("api") })
			app.Domain("www.example.com").Get("/", func(c Ctx) error { return c.SendString("www") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/", status: StatusOK, body: "api"},
			{host: "www.example.com", path: "/", status: StatusOK, body: "www"},
			{host: "other.example.com", path: "/", status: StatusNotFound},
		}},
		{name: "group of a domain", register: func(app *App) {
			app.Domain("api.example.com").Group("/api").Get("/users", func(c Ctx) error {
				return c.SendString("users list")
			})
		}, checks: []routerReq{
			{host: "api.example.com", path: "/api/users", status: StatusOK, body: "users list"},
			{host: "www.example.com", path: "/api/users", status: StatusNotFound},
		}},
		{name: "domain of a group", register: func(app *App) {
			app.Group("/api").Domain("api.example.com").Get("/users", func(c Ctx) error {
				return c.SendString("users from group domain")
			})
		}, checks: []routerReq{
			{host: "api.example.com", path: "/api/users", status: StatusOK, body: "users from group domain"},
			{host: "www.example.com", path: "/api/users", status: StatusNotFound},
		}},
		{name: "domain of a domain", register: func(app *App) {
			// replaces the pattern of the domain router it is created from
			other := app.Domain("api.example.com").Domain("www.example.com")
			other.Get("/", func(c Ctx) error { return c.SendString("www") })
		}, checks: []routerReq{{host: "www.example.com", path: "/", status: StatusOK, body: "www"}}},
		{name: "nested groups", register: func(app *App) {
			users := app.Domain("api.example.com").Group("/v1").Group("/users")
			users.Get("/", func(c Ctx) error { return c.SendString("v1 users") })
		}, checks: []routerReq{{host: "api.example.com", path: "/v1/users/", status: StatusOK, body: "v1 users"}}},
		{name: "path params", register: func(app *App) {
			app.Domain(":tenant.example.com").Get("/users/:id", func(c Ctx) error {
				return c.SendString(DomainParam(c, "tenant") + ":" + c.Params("id"))
			})
		}, checks: []routerReq{{host: "acme.example.com", path: "/users/42", status: StatusOK, body: "acme:42"}}},
		{name: "stale domain params cleared", register: func(app *App) {
			app.Domain(":tenant.example.com").Use(func(c Ctx) error {
				c.Set("X-Tenant", DomainParam(c, "tenant"))
				return c.Next()
			})
			// a static domain (no params) must clear the params the parametric one set
			app.Domain("static.example.com").Get("/check", func(c Ctx) error {
				return c.SendString("tenant=" + DomainParam(c, "tenant"))
			})
		}, checks: []routerReq{{host: "static.example.com", path: "/check", status: StatusOK, body: "tenant="}}},
		{name: "fallback to a route without domain", register: func(app *App) {
			// the domain route is registered first
			app.Domain("api.example.com").Get("/", func(c Ctx) error { return c.SendString("api") })
			app.Get("/", func(c Ctx) error { return c.SendString("fallback") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/", status: StatusOK, body: "api"},
			{host: "www.example.com", path: "/", status: StatusOK, body: "fallback"},
		}},
		{name: "HEAD route", register: func(app *App) {
			app.Domain("api.example.com").Head("/test", func(c Ctx) error {
				c.Set("X-Custom", "head")
				return c.SendStatus(StatusOK)
			})
		}, checks: []routerReq{{method: MethodHead, host: "api.example.com", path: "/test", status: StatusOK}}},
		{name: "All", register: func(app *App) {
			app.Domain("api.example.com").All("/test", func(c Ctx) error { return c.SendString("all methods") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/test", status: StatusOK},
			{method: MethodPost, host: "api.example.com", path: "/test", status: StatusOK},
			{method: MethodPut, host: "api.example.com", path: "/test", status: StatusOK},
			{method: MethodDelete, host: "api.example.com", path: "/test", status: StatusOK},
		}},
		{name: "Add", register: func(app *App) {
			app.Domain("api.example.com").Add([]string{MethodGet, MethodPost}, "/test", func(c Ctx) error {
				return c.SendString("ok")
			})
		}, checks: []routerReq{
			{host: "api.example.com", path: "/test", status: StatusOK},
			{method: MethodPost, host: "api.example.com", path: "/test", status: StatusOK},
		}},
		{name: "chained registration", register: func(app *App) {
			// the methods return the domain router for chaining
			app.Domain("api.example.com").Get("/a", func(c Ctx) error { return c.SendString("a") }).
				Post("/b", func(c Ctx) error { return c.SendString("b") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/a", status: StatusOK},
			{method: MethodPost, host: "api.example.com", path: "/b", status: StatusOK},
		}},
		{name: "Route", register: func(app *App) {
			app.Domain("api.example.com").Route("/api", func(router Router) {
				router.Get("/users", func(c Ctx) error { return c.SendString("users") })
				router.Get("/posts", func(c Ctx) error { return c.SendString("posts") })
			})
		}, checks: []routerReq{
			{host: "api.example.com", path: "/api/users", status: StatusOK, body: "users"},
			{host: "api.example.com", path: "/api/posts", status: StatusOK, body: "posts"},
			{host: "www.example.com", path: "/api/users", status: StatusNotFound},
		}},
		{name: "RouteChain", register: func(app *App) {
			app.Domain("api.example.com").RouteChain("/api/users").
				Get(func(c Ctx) error { return c.SendString("get users") }).
				Post(func(c Ctx) error { return c.SendString("create user") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/api/users", status: StatusOK, body: "get users"},
			{method: MethodPost, host: "api.example.com", path: "/api/users", status: StatusOK, body: "create user"},
			{host: "www.example.com", path: "/api/users", status: StatusNotFound},
		}},
		{name: "nested RouteChain", register: func(app *App) {
			app.Domain("api.example.com").RouteChain("/api").RouteChain("/v1").
				Get(func(c Ctx) error { return c.SendString("v1") })
		}, checks: []routerReq{{host: "api.example.com", path: "/api/v1", status: StatusOK, body: "v1"}}},
		{name: "mounted sub-app", register: func(app *App) {
			subApp := New()
			subApp.Get("/users", func(c Ctx) error { return c.SendString("users list") })
			subApp.Get("/posts", func(c Ctx) error { return c.SendString("posts list") })
			app.Domain("api.example.com").Use("/api", subApp)
		}, checks: []routerReq{
			{host: "api.example.com", path: "/api/users", status: StatusOK, body: "users list"},
			{host: "api.example.com", path: "/api/posts", status: StatusOK, body: "posts list"},
			{host: "www.example.com", path: "/api/users", status: StatusNotFound},
		}},
		{name: "sub-app mounted at the root", register: func(app *App) {
			app.Domain("api.example.com").Use(domainTestApp("/users", "users list"))
		}, checks: []routerReq{
			{host: "api.example.com", path: "/users", status: StatusOK, body: "users list"},
			{host: "www.example.com", path: "/users", status: StatusNotFound},
		}},
		{name: "sub-app mounted through a group", register: func(app *App) {
			app.Group("/api").Domain("api.example.com").Use("/v1", domainTestApp("/data", "data response"))
		}, checks: []routerReq{
			// the group prefix, then the mount prefix
			{host: "api.example.com", path: "/api/v1/data", status: StatusOK, body: "data response"},
			{host: "www.example.com", path: "/api/v1/data", status: StatusNotFound},
		}},
		{name: "middleware", register: func(app *App) {
			domain := app.Domain("api.example.com")
			domain.Use(func(c Ctx) error {
				c.Set("X-Domain", "api")
				return c.Next()
			})
			domain.Get("/", func(c Ctx) error { return c.SendString("ok") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/", status: StatusOK, header: "X-Domain", value: "api"},
			{host: "www.example.com", path: "/", status: StatusNotFound, header: "X-Domain"},
		}},
		{name: "middleware under a prefix", register: func(app *App) {
			domain := app.Domain("api.example.com")
			domain.Use("/api", func(c Ctx) error {
				c.Set("X-API", "true")
				return c.Next()
			})
			domain.Get("/api/data", func(c Ctx) error { return c.SendString("data") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/api/data", status: StatusOK, header: "X-API", value: "true"},
		}},
		{name: "middleware under several prefixes", register: func(app *App) {
			domain := app.Domain("api.example.com")
			domain.Use([]string{"/a", "/b"}, func(c Ctx) error {
				c.Set("X-Domain-MW", "true")
				return c.Next()
			})
			domain.Get("/a/test", func(c Ctx) error { return c.SendString("a") })
			domain.Get("/b/test", func(c Ctx) error { return c.SendString("b") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/a/test", status: StatusOK, header: "X-Domain-MW", value: "true"},
			{host: "api.example.com", path: "/b/test", status: StatusOK, header: "X-Domain-MW", value: "true"},
		}},
		{name: "group middleware", register: func(app *App) {
			api := app.Domain("api.example.com").Group("/api", func(c Ctx) error {
				c.Set("X-Group-MW", "yes")
				return c.Next()
			})
			api.Get("/data", func(c Ctx) error { return c.SendString("data") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/api/data", status: StatusOK, header: "X-Group-MW", value: "yes"},
			{host: "www.example.com", path: "/api/data", status: StatusNotFound, header: "X-Group-MW"},
		}},
		{name: "several handlers", register: func(app *App) {
			app.Domain("api.example.com").Get("/test", func(c Ctx) error {
				c.Set("X-First", "true")
				return c.Next()
			}, func(c Ctx) error { return c.SendString("final") })
		}, checks: []routerReq{
			{host: "api.example.com", path: "/test", status: StatusOK, body: "final", header: "X-First", value: "true"},
			{host: "www.example.com", path: "/test", status: StatusNotFound, header: "X-First"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			tc.register(app)
			routerTestExpectAll(t, app, tc.checks...)
		})
	}
}

// Test_Domain_Params registers one domain route per row, answering with what
// DomainParam reads from the request's Host, and requests it with that host.
func Test_Domain_Params(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		reply   func(c Ctx) string
		name    string
		pattern string
		host    string
		want    string
	}{
		{
			name: "single parameter", pattern: ":user.blog.example.com", host: "john.blog.example.com", want: "blog of john",
			reply: func(c Ctx) string { return "blog of " + DomainParam(c, "user") },
		},
		{
			name: "multiple parameters", pattern: ":sub.:region.example.com", host: "api.us-east.example.com", want: "api-us-east",
			reply: func(c Ctx) string { return DomainParam(c, "sub") + "-" + DomainParam(c, "region") },
		},
		{
			// Use mixed-case param name ":User" — DomainParam should find it by exact name
			name: "parameter name case preserved", pattern: ":User.example.com", host: "alice.example.com", want: "alice",
			reply: func(c Ctx) string { return DomainParam(c, "User") },
		},
		{
			// No domain params set, should return default
			name: "default value", pattern: "example.com", host: "example.com", want: "default",
			reply: func(c Ctx) string { return DomainParam(c, "user", "default") },
		},
		{
			name: "no default", pattern: "example.com", host: "example.com", want: "user=",
			reply: func(c Ctx) string { return "user=" + DomainParam(c, "user") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Domain(tc.pattern).Get("/", func(c Ctx) error { return c.SendString(tc.reply(c)) })

			domainTestExpectBody(t, app, tc.host, "/", StatusOK, tc.want)
		})
	}
}

// Test_Domain_HostMatching registers one domain route answering "ok" per row and
// sends a request with the row's Host: the route must answer when matched is
// set and must not be reached (404) otherwise.
func Test_Domain_HostMatching(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		pattern string
		host    string
		matched bool
	}{
		{name: "case-insensitive pattern", pattern: "API.Example.COM", host: "api.example.com", matched: true},
		// Fully-qualified domain name with trailing dot should match
		{name: "trailing dot", pattern: "api.example.com", host: "api.example.com.", matched: true},
		// Host with port - Hostname() strips the port
		{name: "host with port", pattern: "api.example.com", host: "api.example.com:8080", matched: true},
		// Different number of domain parts
		{name: "fewer parts", pattern: "api.example.com", host: "example.com"},
		{name: "more parts", pattern: "api.example.com", host: "sub.api.example.com"},
		// Empty host should not match
		{name: "empty host", pattern: "api.example.com", host: ""},
		// Hostname with too many labels should not match (DoS protection)
		{name: "too many labels", pattern: "example.com", host: strings.Repeat("sub.", 19) + "sub"},
		// Hostname exceeding 253 characters should not match (DoS protection)
		{name: "hostname longer than 253 characters", pattern: "example.com", host: strings.Repeat("a", 254) + ".com"},
		// Label exceeding 63 characters should not match (DoS protection)
		{name: "label longer than 63 characters", pattern: "example.com", host: strings.Repeat("a", 64) + ".example.com"},
		{name: "empty hostname label", pattern: "example.com", host: "example..com"},
		// Non-ASCII hostnames are rejected at runtime (DNS names are ASCII-only)
		{name: "non-ASCII hostname", pattern: ":sub.example.com", host: "ünïcödé.example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Domain(tc.pattern).Get("/", func(c Ctx) error { return c.SendString("ok") })

			if tc.matched {
				domainTestExpectBody(t, app, tc.host, "/", StatusOK, "ok")
			} else {
				domainTestExpect(t, app, tc.host, "/", StatusNotFound)
			}
		})
	}
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
			m.reg(domain, "/test", func(c Ctx) error { return c.SendString(m.method) })

			routerDomainExpectMethod(t, app, m.method, "api.example.com", "/test", StatusOK)
		})
	}
}

// One path on two patterns is two routes: merged into one, both hostnames would
// run under the name given last, and c.Route() could not tell them apart.
func Test_Domain_SamePathKeepsItsOwnRoute(t *testing.T) {
	t.Parallel()
	app := New()
	served := func(c Ctx) error { return c.SendString(c.Route().Name) }

	app.Domain("admin.example.com").Get("/test", served).Name("admin")
	app.Domain("www.example.com").Get("/test", served).Name("www")
	app.Get("/test", served).Name("any")
	// One pattern twice is still one route
	app.Domain("api.example.com").Get("/merged", func(c Ctx) error { return c.Next() })
	app.Domain("API.example.com.").Get("/merged", served).Name("api")

	for host, want := range map[string]string{
		"admin.example.com": "admin",
		"www.example.com":   "www",
		"other.example.com": "any",
	} {
		req := httptest.NewRequest(MethodGet, "/test", http.NoBody)
		req.Host = host
		resp := apptest.Do(t, app, req)
		body := apptest.Body(t, resp)
		require.Equal(t, want, body, host)
	}

	var merged []Route
	for _, route := range app.GetRoutes(true) {
		if route.Method == MethodGet && route.Path == "/merged" {
			merged = append(merged, route)
		}
	}
	require.Len(t, merged, 1)
	require.Len(t, merged[0].Handlers, 2)
}

// The automatic HEAD route of one pattern declines every other hostname, so it
// cannot stand in for the GET route another pattern has on the same path.
func Test_Domain_AutoHeadPerPattern(t *testing.T) {
	t.Parallel()
	app := New()
	served := func(c Ctx) error {
		c.Set("X-Served", c.Route().Name)
		return c.SendStatus(StatusOK)
	}

	app.Domain("admin.example.com").Get("/test", served).Name("admin")
	app.Domain("www.example.com").Get("/test", served).Name("www")
	app.Domain("api.example.com").Head("/test", served).Name("api.head")
	app.Get("/test", served).Name("any")

	head := func(host string) *http.Response {
		req := httptest.NewRequest(MethodHead, "/test", http.NoBody)
		req.Host = host
		resp := apptest.Do(t, app, req)
		return resp
	}

	require.Equal(t, "admin", head("admin.example.com").Header.Get("X-Served"))
	require.Equal(t, "www", head("www.example.com").Header.Get("X-Served"))
	require.Equal(t, "api.head", head("api.example.com").Header.Get("X-Served"), "renamed by another host's GET route")
	require.Equal(t, "any", head("other.example.com").Header.Get("X-Served"))

	// Removing one pattern's GET route takes its own companion along, not another's
	app.RemoveRouteByName("admin", MethodGet)
	app.RebuildTree()

	require.Equal(t, "any", head("admin.example.com").Header.Get("X-Served"))
	require.Equal(t, "www", head("www.example.com").Header.Get("X-Served"))
}

// Test_Domain_AutoHeadAheadOfLaterHead pins that a HEAD route on another Domain
// pattern, or in a mounted app, runs behind the automatic route of an earlier GET.
func Test_Domain_AutoHeadAheadOfLaterHead(t *testing.T) {
	t.Parallel()
	handler := func(name string) Handler {
		return func(c Ctx) error {
			c.Set("X-Handler", name)
			return nil
		}
	}

	testCases := []struct {
		register func(app *App)
		want     map[string]string
		name     string
	}{
		{
			name: "Domain HEAD after GET",
			register: func(app *App) {
				app.Get("/x", handler("get"))
				app.Domain("admin.example.com").Head("/x", handler("domain head"))
			},
			want: map[string]string{"admin.example.com": "get", "other.example.com": "get"},
		},
		{
			name: "HEAD after Domain GET",
			register: func(app *App) {
				app.Domain("admin.example.com").Get("/x", handler("domain get"))
				app.Head("/x", handler("head"))
			},
			want: map[string]string{"admin.example.com": "domain get", "other.example.com": "head"},
		},
		{
			name: "mounted HEAD after GET",
			register: func(app *App) {
				app.Get("/x", handler("get"))
				sub := New()
				sub.Head("/x", handler("sub head"))
				app.Use("/", sub)
			},
			want: map[string]string{"admin.example.com": "get"},
		},
		{
			name: "Domain-mounted HEAD after GET",
			register: func(app *App) {
				app.Get("/x", handler("get"))
				sub := New()
				sub.Head("/x", handler("sub head"))
				app.Domain("admin.example.com").Use("/", sub)
			},
			want: map[string]string{"admin.example.com": "get", "other.example.com": "get"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			tc.register(app)

			for host, want := range tc.want {
				req := httptest.NewRequest(MethodHead, "/x", http.NoBody)
				req.Host = host
				resp := apptest.Do(t, app, req)
				require.Equal(t, StatusOK, resp.StatusCode, host)
				require.Equal(t, want, resp.Header.Get("X-Handler"), host)
			}
		})
	}
}

func Test_Domain_Name(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		register func(app *App)
		name     string
		want     string
	}{
		{name: "route", want: "api-test", register: func(app *App) {
			app.Domain("api.example.com").Get("/test", func(c Ctx) error { return c.SendString("ok") }).Name("api-test")
		}},
		// When Domain is used with Route(prefix, fn, name), the group Name()
		// should apply properly via delegation to the underlying group.
		{name: "group", want: "v1.items-list", register: func(app *App) {
			app.Domain("api.example.com").Route("/v1", func(r Router) {
				r.Get("/items", func(c Ctx) error { return c.SendString("items") }).Name("items-list")
			}, "v1.")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			tc.register(app)

			var found bool
			for _, routes := range app.Stack() {
				for _, route := range routes {
					found = found || route.Name == tc.want
				}
			}
			require.True(t, found, "route should be named %q", tc.want)
		})
	}
}

func Test_Domain_RoutePanic(t *testing.T) {
	t.Parallel()
	app := New()

	require.Panics(t, func() { app.Domain("api.example.com").Route("/test", nil) })
}

func Test_Domain_RouteChainAllMethods(t *testing.T) {
	t.Parallel()
	app := New()

	rc := app.Domain("api.example.com").RouteChain("/test")
	rc.All(func(c Ctx) error {
		c.Set("X-All", "yes")
		return c.Next()
	})
	rc.Head(func(c Ctx) error { return c.SendStatus(StatusOK) })
	rc.Put(func(c Ctx) error { return c.SendString("put") })
	rc.Delete(func(c Ctx) error { return c.SendString("delete") })
	rc.Connect(func(c Ctx) error { return c.SendString("connect") })
	rc.Options(func(c Ctx) error { return c.SendString("options") })
	rc.Trace(func(c Ctx) error { return c.SendString("trace") })
	rc.Patch(func(c Ctx) error { return c.SendString("patch") })
	rc.Query(func(c Ctx) error { return c.SendString("query") })

	for _, method := range []string{MethodPut, MethodDelete, MethodPatch, MethodOptions, MethodQuery} {
		routerDomainExpectMethod(t, app, method, "api.example.com", "/test", StatusOK)
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
	// Handlers that are the only one of their route.
	for _, tc := range []struct {
		handler any
		name    string
		header  string // response header the handler sets, if it does
		value   string // the value that header must have
		body    string // response body the handler writes, if it is checked
	}{
		{name: "fiber handler", body: "fiber", handler: func(c Ctx) error { return c.SendString("fiber") }},
		{name: "fiber handler no error", header: "X-Handler", value: "no-error", handler: func(c Ctx) {
			c.Set("X-Handler", "no-error")
		}},
		{name: "express req res error", body: "express-err", handler: func(_ Req, res Res) error {
			return res.SendString("express-err")
		}},
		{name: "express req res no error", header: "X-Express", value: "ok", handler: func(_ Req, res Res) {
			res.Set("X-Express", "ok")
		}},
		{name: "net/http HandlerFunc", handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})},
		{name: "net/http HandlerFunc writing a body", body: "net/http handler", handler: http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("net/http handler"))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			})},
		{name: "net/http func handler", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}},
		{name: "fasthttp RequestHandler", handler: fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("fasthttp")
		})},
		{name: "fasthttp handler with error", handler: func(ctx *fasthttp.RequestCtx) error {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("fasthttp-err")
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Domain("api.example.com").Get("/test", tc.handler)

			resp := domainTestExpect(t, app, "api.example.com", "/test", StatusOK)
			if tc.header != "" {
				require.Equal(t, tc.value, resp.Header.Get(tc.header))
			}
			if tc.body != "" {
				require.Equal(t, tc.body, apptest.Body(t, resp))
			}
		})
	}

	// Express-style middleware in front of a final fiber handler: it sets X-MW
	// and hands over, so the final handler must have run too.
	for _, tc := range []struct {
		middleware any
		name       string
	}{
		{name: "express next-err returns-err", middleware: func(_ Req, res Res, next func() error) error {
			res.Set("X-MW", "yes")
			return next()
		}},
		{name: "express with next error", middleware: func(_ Req, res Res, next func() error) {
			res.Set("X-MW", "yes")
			_ = next() //nolint:errcheck // test
		}},
		{name: "express with noarg next error", middleware: func(_ Req, res Res, next func()) error {
			res.Set("X-MW", "yes")
			next()
			return nil
		}},
		{name: "express with noarg next", middleware: func(_ Req, res Res, next func()) {
			res.Set("X-MW", "yes")
			next()
		}},
		{name: "express with error next", middleware: func(_ Req, res Res, next func(error)) {
			res.Set("X-MW", "yes")
			next(nil)
		}},
		{name: "express with error next error", middleware: func(_ Req, res Res, next func(error)) error {
			res.Set("X-MW", "yes")
			next(nil)
			return nil
		}},
		{name: "express errnext-err returns-err", middleware: func(_ Req, res Res, next func(error) error) {
			res.Set("X-MW", "yes")
			_ = next(nil) //nolint:errcheck // test
		}},
		{name: "express errnext-err returns-err err", middleware: func(_ Req, res Res, next func(error) error) error {
			res.Set("X-MW", "yes")
			return next(nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Domain("api.example.com").Get(
				"/test",
				tc.middleware,
				func(c Ctx) error { return c.SendString("chained") },
			)

			resp := domainTestExpect(t, app, "api.example.com", "/test", StatusOK)
			require.Equal(t, "yes", resp.Header.Get("X-MW"))
			require.Equal(t, "chained", apptest.Body(t, resp))
		})
	}

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

		resp := domainTestExpect(t, app, "wrong.example.com", "/test", StatusNotFound)
		require.Empty(t, resp.Header.Get("X-Handler"))
	})
}

func Test_Domain_UseHandlerTypes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		middleware any
		name       string
		want       string
	}{
		{name: "fiber handler middleware", want: "fiber", middleware: func(c Ctx) error {
			c.Set("X-MW", "fiber")
			return c.Next()
		}},
		{name: "express middleware", want: "express", middleware: func(_ Req, res Res, next func() error) error {
			res.Set("X-MW", "express")
			return next()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			domain := app.Domain("api.example.com")
			domain.Use(tc.middleware)
			domain.Get("/test", func(c Ctx) error { return c.SendString("ok") })

			resp := domainTestExpect(t, app, "api.example.com", "/test", StatusOK)
			require.Equal(t, tc.want, resp.Header.Get("X-MW"))
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

	app.Domain("api.example.com").Get("/test", func(c Ctx) error { return c.SendString("ok") })

	req := apptest.Req(MethodGet, "/test", apptest.WithHost("api.example.com"))

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

	app.Get("/test", func(c Ctx) error { return c.SendString("ok") })

	req := apptest.Req(MethodGet, "/test")

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

// Test_Domain_Security_Patterns checks which domain patterns are rejected when
// the domain router is created, and that valid parameter names are accepted.
func Test_Domain_Security_Patterns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		pattern string
		valid   bool
	}{
		{name: "empty", pattern: ""},
		{name: "blank", pattern: "   "},
		{name: "empty label in the middle", pattern: "example..com"},
		{name: "empty leading label", pattern: ".example.com"},
		{name: "empty parameter name", pattern: ":.example.com"},
		{name: "parameter name with at sign", pattern: ":user@host.example.com"},
		{name: "parameter name with dollar sign", pattern: ":user$.example.com"},
		{name: "parameter name with space", pattern: ":user name.example.com"},
		{name: "label with dollar sign", pattern: "example$.com"},
		{name: "label with at sign", pattern: "example@domain.com"},
		{name: "label with space", pattern: "example domain.com"},
		// DoS protection against excessive domain labels
		{name: "too many parts", pattern: strings.Repeat("sub.", 19) + "sub"},
		// Non-ASCII characters are rejected in both domain labels and parameter
		// names (DNS names are ASCII-only).
		{name: "non-ASCII label", pattern: "ünïcödé.example.com"},
		{name: "non-ASCII parameter name", pattern: ":üser.example.com"},
		{name: "alphanumeric parameter name", pattern: ":user123.example.com", valid: true},
		{name: "parameter name with underscore", pattern: ":user_name.example.com", valid: true},
		{name: "parameter name with hyphen", pattern: ":user-name.example.com", valid: true},
		{name: "mixed parameter name", pattern: ":user_123-name.example.com", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.valid {
				require.NotPanics(t, func() { parseDomainPattern(tc.pattern) })
				return
			}
			require.Panics(t, func() { parseDomainPattern(tc.pattern) })
		})
	}
}

func Test_Domain_Security_InvalidHostnameChars(t *testing.T) {
	t.Parallel()
	// Note: Some invalid chars (like spaces) are rejected by httptest.NewRequest itself
	for _, tc := range []struct {
		name string
		host string
	}{
		{name: "dollar sign", host: "example$.com"},
		{name: "at sign", host: "example@domain.com"},
		{name: "NUL", host: "example\x00.com"},
		{name: "newline", host: "example\n.com"},
		{name: "semicolon", host: "example;.com"},
		{name: "slash", host: "example/.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Domain("example.com").Get("/", func(c Ctx) error { return c.SendString("ok") })

			resp, err := app.Test(apptest.Req(MethodGet, "/", apptest.WithHost(tc.host)))
			if err == nil {
				require.Equal(t, StatusNotFound, resp.StatusCode, "Should reject hostname: %s", tc.host)
			}
			// If there's an error, the validation happened at an earlier layer which is also acceptable
		})
	}
}

// Test_Domain_UseMountReusable verifies that mounting the same sub-app on
// multiple domain routers does not double-wrap handlers (the original sub-app
// is not mutated).
func Test_Domain_UseMountReusable(t *testing.T) {
	t.Parallel()
	app := New()
	subApp := domainTestApp("/data", "data response")
	app.Domain("api.example.com").Use("/v1", subApp)
	app.Domain("admin.example.com").Use("/v1", subApp)

	domainTestExpectBody(t, app, "api.example.com", "/v1/data", StatusOK, "data response")
	domainTestExpectBody(t, app, "admin.example.com", "/v1/data", StatusOK, "data response")
	domainTestExpect(t, app, "www.example.com", "/v1/data", StatusNotFound)
}

// Test_Domain_UseMountRoutesAfterMount verifies that routes added to a sub-app
// after it has been mounted on a domain router are NOT domain-filtered (since
// mount clones routes at mount time).
func Test_Domain_UseMountRoutesAfterMount(t *testing.T) {
	t.Parallel()
	app := New()
	subApp := domainTestApp("/before", "before mount")
	app.Domain("api.example.com").Use("/api", subApp)

	// Register a route AFTER mounting — this will NOT be domain-filtered
	// because mount() clones routes at mount time.
	subApp.Get("/after", func(c Ctx) error { return c.SendString("after mount") })

	domainTestExpectBody(t, app, "api.example.com", "/api/before", StatusOK, "before mount")
	domainTestExpect(t, app, "www.example.com", "/api/before", StatusNotFound)

	// Route registered after mount on the original sub-app is NOT included
	// in the wrapper. Since the mount group references the wrapper, the
	// after-mount route is never expanded into the parent app.
	domainTestExpect(t, app, "api.example.com", "/api/after", StatusNotFound)
}

// Test_Domain_UseMountNested verifies that a sub-app that itself mounts another
// app can be mounted on a domain router: the nested routes are served under the
// domain and are filtered by host like any other domain-mounted route.
func Test_Domain_UseMountNested(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/x", "nested x")

	subApp := routerDomainWith("/v1", child)
	subApp.Get("/flat", func(c Ctx) error { return c.SendString("flat") })

	app := routerDomainMount(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/v1/x", StatusOK, "nested x")
	// The sub-app's own routes still work alongside the nested ones
	domainTestExpectBody(t, app, "api.example.com", "/api/flat", StatusOK, "flat")
	domainTestExpect(t, app, "www.example.com", "/api/v1/x", StatusNotFound)
	domainTestExpect(t, app, "www.example.com", "/api/flat", StatusNotFound)
}

// Test_Domain_UseMountNestedDeep verifies that mounts nested more than one
// level deep are expanded, and that domain parameters resolve inside them.
func Test_Domain_UseMountNestedDeep(t *testing.T) {
	t.Parallel()
	grandChild := New()
	grandChild.Get("/deep", func(c Ctx) error { return c.SendString("tenant=" + DomainParam(c, "tenant")) })

	child := routerDomainWith("/v2", grandChild)

	subApp := routerDomainWith("/v1", child)

	app := New()
	app.Domain(":tenant.example.com").Use("/api", subApp)

	domainTestExpectBody(t, app, "acme.example.com", "/api/v1/v2/deep", StatusOK, "tenant=acme")
	domainTestExpect(t, app, "acme.example.org", "/api/v1/v2/deep", StatusNotFound)
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
	child.Get("/x", func(c Ctx) error { return c.SendString("nested x") })

	subApp := routerDomainWith("/v1", child)
	app := routerDomainMount(subApp)
	// Same path outside the domain, so the request is served instead of 404ing
	// and any nested middleware that leaked would run.
	app.Get("/api/v1/x", func(c Ctx) error { return c.SendString("fallback") })

	domainTestExpectBody(t, app, "www.example.com", "/api/v1/x", StatusOK, "fallback")
	require.Equal(t, 0, hits, "nested middleware must not run on a non-matching host")
	domainTestExpectBody(t, app, "api.example.com", "/api/v1/x", StatusOK, "nested x")
	require.Equal(t, 1, hits)
}

// Test_Domain_UseMountNestedAtRoot verifies the nested mount also works when
// both the domain mount and the nested mount sit at the root path.
func Test_Domain_UseMountNestedAtRoot(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/x", "nested x")

	subApp := New()
	subApp.Use(child)

	app := New()
	app.Domain("api.example.com").Use(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/x", StatusOK, "nested x")
	domainTestExpect(t, app, "www.example.com", "/x", StatusNotFound)
}

// Test_Domain_UseMountNestedFromGroup verifies a nested mount registered
// through a group's domain router picks up the group prefix.
func Test_Domain_UseMountNestedFromGroup(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/data", "data response")

	subApp := routerDomainWith("/v1", child)

	app := New()
	api := app.Group("/api")
	api.Domain("api.example.com").Use("/mnt", subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/mnt/v1/data", StatusOK, "data response")
	domainTestExpect(t, app, "www.example.com", "/api/mnt/v1/data", StatusNotFound)
}

// Test_Domain_UseMountNestedReusable verifies that a sub-app with a nested
// mount can be mounted on several domains and still be usable unfiltered
// elsewhere, i.e. the original sub-app's handlers are never domain-wrapped.
func Test_Domain_UseMountNestedReusable(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/x", "nested x")

	subApp := routerDomainWith("/v1", child)
	app := routerDomainMount(subApp)
	app.Domain("admin.example.com").Use("/api", subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/v1/x", StatusOK, "nested x")
	domainTestExpectBody(t, app, "admin.example.com", "/api/v1/x", StatusOK, "nested x")
	domainTestExpect(t, app, "www.example.com", "/api/v1/x", StatusNotFound)

	// The original sub-app's handlers were never wrapped, so a plain mount of
	// it still serves on any host.
	plain := routerDomainWith("/api", subApp)

	domainTestExpectBody(t, plain, "anything.example.net", "/api/v1/x", StatusOK, "nested x")
}

// Test_Domain_UseMountNestedOrder verifies the nested app's routes are expanded
// in the position its mount was registered at, so registration order still
// decides which route wins.
func Test_Domain_UseMountNestedOrder(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/x", "child")

	subApp := New()
	subApp.Get("/v1/x", func(c Ctx) error { return c.SendString("before mount") })
	subApp.Use("/v1", child)
	subApp.Get("/v1/y", func(c Ctx) error { return c.SendString("after mount") })

	app := routerDomainMount(subApp)

	// Registered before the mount, so it takes precedence over the nested /x
	domainTestExpectBody(t, app, "api.example.com", "/api/v1/x", StatusOK, "before mount")

	// Routes registered after the mount are kept as well
	domainTestExpectBody(t, app, "api.example.com", "/api/v1/y", StatusOK, "after mount")
}

// Test_Domain_UseMountNestedParams verifies parameters and wildcards of a
// nested sub-app survive the prefixing done while expanding the mount.
func Test_Domain_UseMountNestedParams(t *testing.T) {
	t.Parallel()
	child := New()
	child.Get("/user/:id", func(c Ctx) error { return c.SendString("id=" + c.Params("id")) })
	child.Get("/files/*", func(c Ctx) error { return c.SendString("file=" + c.Params("*")) })

	subApp := routerDomainWith("/v1", child)
	app := routerDomainMount(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/v1/user/42", StatusOK, "id=42")
	domainTestExpectBody(t, app, "api.example.com", "/api/v1/files/a/b.txt", StatusOK, "file=a/b.txt")
}

// Test_Domain_UseMountNestedDomainMount verifies that a sub-app whose own
// nested mount was registered on a domain router can itself be mounted, both
// on a domain router and plainly.
func Test_Domain_UseMountNestedDomainMount(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/x", "nested x")

	subApp := New()
	subApp.Domain("api.example.com").Use("/v1", child)

	// Mounted on a matching domain: the host check applies twice, which is
	// satisfied by the same host.
	app := routerDomainMount(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/v1/x", StatusOK, "nested x")
	domainTestExpect(t, app, "www.example.com", "/api/v1/x", StatusNotFound)

	// Mounted plainly: the sub-app's own domain filter still applies.
	plain := routerDomainWith("/api", subApp)

	domainTestExpectBody(t, plain, "api.example.com", "/api/v1/x", StatusOK, "nested x")
	domainTestExpect(t, plain, "www.example.com", "/api/v1/x", StatusNotFound)
}

// Test_Domain_UseMountNestedRestrictedMethods verifies that a sub-app which
// declares fewer request methods than the app it is mounted on can still be
// domain-mounted: the wrapper's stack has to stay indexable at the parent's
// method indexes.
func Test_Domain_UseMountNestedRestrictedMethods(t *testing.T) {
	t.Parallel()
	subApp := domainTestApp("/x", "restricted x", Config{RequestMethods: []string{MethodGet}})

	app := routerDomainMount(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/x", StatusOK, "restricted x")
	domainTestExpect(t, app, "www.example.com", "/api/x", StatusNotFound)
}

// Test_Domain_UseMountNestedExtraMethods verifies that a sub-app can be
// domain-mounted on an app whose request methods go beyond the defaults: the
// parent reads the wrapper's stack at its own method indexes, so the wrapper
// has to be as long as the parent's method table.
func Test_Domain_UseMountNestedExtraMethods(t *testing.T) {
	t.Parallel()
	methods := append(append([]string{}, DefaultMethods...), "PURGE")
	app := New(Config{RequestMethods: methods})

	child := domainTestApp("/x", "nested x")
	subApp := routerDomainWith("/v1", child)

	app.Domain("api.example.com").Use("/api", subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/v1/x", StatusOK, "nested x")
	domainTestExpect(t, app, "www.example.com", "/api/v1/x", StatusNotFound)
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
	child.Get("/x", func(c Ctx) error { return c.SendString("get x") })
	child.Post("/x", func(c Ctx) error { return c.SendString("post x") })

	subApp := routerDomainWith("/v1", child)
	app := routerDomainMount(subApp)

	apptest.ExpectBody(t, app, apptest.Req(MethodGet, "/api/v1/x", apptest.WithHost("api.example.com")), StatusOK, "get x")
	apptest.ExpectBody(t, app, apptest.Req(MethodPost, "/api/v1/x", apptest.WithHost("api.example.com")), StatusOK, "post x")
}

// Test_Domain_UseMountAutoHead verifies that domain-mounted routes get their
// automatic HEAD companion, for the sub-app's own routes and for the routes of
// an app it has mounted, and that the HEAD route is domain-filtered too.
func Test_Domain_UseMountAutoHead(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/nested", "nested")

	subApp := routerDomainWith("/v1", child)
	subApp.Get("/flat", func(c Ctx) error { return c.SendString("flat") })

	app := routerDomainMount(subApp)

	// HEAD is the first request, so the route has to exist after the single
	// startup pass a served app performs.
	for _, path := range []string{"/api/v1/nested", "/api/flat"} {
		routerDomainExpectMethod(t, app, MethodHead, "api.example.com", path, StatusOK)
		routerDomainExpectMethod(t, app, MethodHead, "www.example.com", path, StatusNotFound)
	}
}

// Test_Domain_UseMountAutoHeadDisabled verifies that a sub-app, or an app nested in it, which turned automatic HEAD
// registration off does not get HEAD routes from the mount.
func Test_Domain_UseMountAutoHeadDisabled(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, nestedAt string }{{name: "direct"}, {name: "nested", nestedAt: "/v1"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			subApp := domainTestApp("/flat", "flat", Config{DisableHeadAutoRegister: true})
			if tc.nestedAt != "" {
				subApp = routerDomainWith(tc.nestedAt, subApp)
			}

			app := routerDomainMount(subApp)

			routerDomainExpectMethod(t, app, MethodHead, "api.example.com", "/api"+tc.nestedAt+"/flat", StatusMethodNotAllowed)
		})
	}
}

// Test_Domain_UseMountCustomConstraint verifies that custom constraints of a
// domain-mounted app, and of an app it has mounted in turn, still validate once
// the routes are expanded into the parent.
func Test_Domain_UseMountCustomConstraint(t *testing.T) {
	t.Parallel()
	child := New()
	child.RegisterCustomConstraint(&onlyFooConstraint{})
	child.Get("/nested/:name<onlyfoo>", func(c Ctx) error { return c.SendString(c.Params("name")) })

	subApp := New()
	subApp.RegisterCustomConstraint(&onlyBarConstraint{})
	subApp.Use("/v1", child)
	subApp.Get("/flat/:name<onlybar>", func(c Ctx) error { return c.SendString(c.Params("name")) })

	app := routerDomainMount(subApp)

	domainTestExpect(t, app, "api.example.com", "/api/v1/nested/foo", StatusOK)
	domainTestExpect(t, app, "api.example.com", "/api/v1/nested/bar", StatusNotFound)
	domainTestExpect(t, app, "api.example.com", "/api/flat/bar", StatusOK)
	domainTestExpect(t, app, "api.example.com", "/api/flat/foo", StatusNotFound)
}

// Test_Domain_UseMountPerHostConfig verifies that two sub-apps mounted at the
// same path on different domains each keep their own error handler and view
// engine. Keyed by path alone they would displace each other.
func Test_Domain_UseMountPerHostConfig(t *testing.T) {
	t.Parallel()
	subA := New(Config{Views: domainTestEngine(t, "testdata2"), ErrorHandler: domainTestErrorHandler(591, "a error")})
	subA.Get("/boom", domainTestBoom)
	subA.Get("/view", func(c Ctx) error { return c.Render("bruh.tmpl", Map{}) })

	subB := New(Config{Views: domainTestEngine(t, "testdata3"), ErrorHandler: domainTestErrorHandler(592, "b error")})
	subB.Get("/boom", domainTestBoom)
	subB.Get("/view", func(c Ctx) error { return c.Render("hello_world.tmpl", Map{"Name": "b"}) })

	app := New()
	app.Domain("a.example.com").Use("/api", subA)
	app.Domain("b.example.com").Use("/api", subB)

	domainTestExpectBody(t, app, "a.example.com", "/api/boom", 591, "a error")
	domainTestExpectBody(t, app, "b.example.com", "/api/boom", 592, "b error")
	domainTestExpectBody(t, app, "a.example.com", "/api/view", StatusOK, "<h1>I'm Bruh</h1>")
	domainTestExpectBody(t, app, "b.example.com", "/api/view", StatusOK, "<h1>Hello b!</h1>")
}

// Test_Domain_UseMountConfigIgnoredOnOtherHost verifies that a domain-mounted
// sub-app's error handler does not answer for a host its pattern rejects.
func Test_Domain_UseMountConfigIgnoredOnOtherHost(t *testing.T) {
	t.Parallel()
	subApp := domainTestFailingApp(593, "sub error", "/mounted")

	app := routerDomainMount(subApp)
	// A route of the parent, under the same prefix, on every host.
	app.Get("/api/own", domainTestBoom)

	domainTestExpect(t, app, "www.example.com", "/api/own", StatusInternalServerError)

	// On a matching host the sub-app still governs its own prefix.
	domainTestExpectBody(t, app, "api.example.com", "/api/mounted", 593, "sub error")
}

// Test_Domain_UseMountNestedErrorHandler verifies that the deepest mount
// covering the request wins, nested apps included.
func Test_Domain_UseMountNestedErrorHandler(t *testing.T) {
	t.Parallel()
	child := domainTestFailingApp(594, "child error", "/boom")

	subApp := New(Config{ErrorHandler: domainTestErrorHandler(595, "sub error")})
	subApp.Use("/v1", child)
	subApp.Get("/boom", domainTestBoom)

	app := routerDomainMount(subApp)

	domainTestExpect(t, app, "api.example.com", "/api/v1/boom", 594)
	domainTestExpect(t, app, "api.example.com", "/api/boom", 595)
}

// Test_Domain_UseMountLeavesSubAppIntact verifies that mounting a sub-app on a
// domain does not flatten the sub-app's own mounts, so it stays usable on its
// own afterwards.
func Test_Domain_UseMountLeavesSubAppIntact(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/x", "nested x")

	subApp := routerDomainWith("/v1", child)

	getIndex := subApp.methodInt(MethodGet)
	routesBefore := len(subApp.stack[getIndex])

	app := routerDomainMount(subApp)

	domainTestExpect(t, app, "api.example.com", "/api/v1/x", StatusOK)
	require.Len(t, subApp.stack[getIndex], routesBefore, "the sub-app's own stack was rewritten")
	require.True(t, subApp.stack[getIndex][0].mount, "the sub-app's mount was expanded in place")

	// The sub-app still resolves its own mount when served on its own.
	apptest.ExpectBody(t, subApp, "/v1/x", StatusOK, "nested x")
}

// Test_Domain_UseMountViewsLayout verifies that a domain-mounted sub-app's
// ViewsLayout is applied when the caller passes no layout of its own.
func Test_Domain_UseMountViewsLayout(t *testing.T) {
	t.Parallel()
	subApp := domainTestRenderApp("/view", "index.tmpl", Map{"Title": "Hello, World!"},
		Config{Views: domainTestEngine(t, ""), ViewsLayout: "main.tmpl"})

	app := routerDomainMount(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/view", StatusOK, "<h1>Hello, World!</h1><h1>I'm main</h1>")
}

// Test_Domain_UseMountViewsError verifies that a render failure in a
// domain-mounted sub-app's engine is reported rather than falling through to
// the parent's views.
func Test_Domain_UseMountViewsError(t *testing.T) {
	t.Parallel()
	subApp := domainTestRenderApp("/view", "index.tmpl", Map{"Title": "Hello, World!"},
		Config{Views: errorTemplateEngine{}})

	app := routerDomainMount(subApp, Config{Views: domainTestEngine(t, "")})

	resp := domainTestExpect(t, app, "api.example.com", "/api/view", StatusInternalServerError)
	require.Contains(t, apptest.Body(t, resp), "errorTemplateEngine")
}

// Test_Domain_UseMountWithoutViews verifies that a domain-mounted sub-app with
// no view engine of its own leaves rendering to the parent, and that one
// configuring only a layout renders through the engine above it and keeps its
// layout, as an ordinary mount without an engine of its own does.
func Test_Domain_UseMountWithoutViews(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, layout, title, want string
	}{
		{name: "no config", title: "from parent", want: "<h1>from parent</h1>"},
		{name: "layout only", layout: "main.tmpl", title: "Hello", want: "<h1>Hello</h1><h1>I'm main</h1>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			subApp := domainTestRenderApp("/view", "index.tmpl", Map{"Title": tc.title}, Config{ViewsLayout: tc.layout})

			app := routerDomainMount(subApp, Config{Views: domainTestEngine(t, "")})

			domainTestExpectBody(t, app, "api.example.com", "/api/view", StatusOK, tc.want)
		})
	}
}

// Test_Domain_UseMountReloadViews verifies that ReloadViews reaches the view
// engine of a domain-mounted sub-app, which appList no longer holds.
func Test_Domain_UseMountReloadViews(t *testing.T) {
	t.Parallel()
	subApp := domainTestRenderApp("/view", "bruh.tmpl", Map{}, Config{Views: domainTestEngine(t, "testdata2")})

	app := routerDomainMount(subApp)

	require.NoError(t, app.ReloadViews())
	domainTestExpectBody(t, app, "api.example.com", "/api/view", StatusOK, "<h1>I'm Bruh</h1>")
}

// Test_Domain_UseMountSelf verifies that mounting an app on its own domain
// router returns instead of deadlocking on the app's mutex.
func Test_Domain_UseMountSelf(t *testing.T) {
	t.Parallel()
	app := domainTestApp("/x", "x")

	done := make(chan struct{})

	var mountPanic any
	go func() {
		defer close(done)
		defer func() { mountPanic = recover() }()

		app.Domain("api.example.com").Use("/self", app)
	}()

	apptest.Recv(t, done, 10*time.Second, "mounting an app on its own domain router did not return")
	require.Nil(t, mountPanic, "mounting an app on its own domain router panicked")
}

// Test_Domain_UseMountAutoHeadKeepsMiddlewareWhenFlattened pins the same once
// the app that left HEAD out is no longer walked, only its routes' marks are.
func Test_Domain_UseMountAutoHeadKeepsMiddlewareWhenFlattened(t *testing.T) {
	t.Parallel()
	newSubApp := func() *App {
		subApp := New(Config{RequestMethods: []string{MethodGet, MethodPost}})
		subApp.Use(func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
		subApp.Get("/secret", func(c Ctx) error {
			c.Set("X-Secret", "leaked")
			return c.SendString("secret")
		})
		return subApp
	}

	for name, mountOn := range map[string]func(mid *App){
		"inner domain mount": func(mid *App) { mid.Domain("api.example.com").Use("/sub", newSubApp()) },
		"plain mount over a domain mount": func(mid *App) {
			inner := New()
			inner.Domain("api.example.com").Use("/sub", newSubApp())
			mid.Use("/", inner)
		},
		"started before mounting": func(mid *App) {
			mid.Use("/sub", newSubApp())
			mid.Handler()
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mid := New()
			mountOn(mid)

			app := New()
			app.Domain("api.example.com").Use("/mid", mid)

			req := httptest.NewRequest(MethodGet, "/mid/sub/secret", http.NoBody)
			req.Host = "api.example.com"
			resp := apptest.Expect(t, app, req, StatusUnauthorized)

			for range 2 {
				req = httptest.NewRequest(MethodHead, "/mid/sub/secret", http.NoBody)
				req.Host = "api.example.com"
				resp = apptest.Expect(t, app, req, StatusMethodNotAllowed)
				require.Empty(t, resp.Header.Get("X-Secret"))
			}
		})
	}
}

// Test_Domain_UseMountAfterOuterMount verifies that a domain mount registered
// on a sub-app after that sub-app was itself mounted is still found: the
// parent picks the metadata up when it discovers sub-apps at startup.
func Test_Domain_UseMountAfterOuterMount(t *testing.T) {
	t.Parallel()
	child := domainTestFailingApp(596, "child error", "/boom")

	mid := New()

	app := routerDomainWith("/mid", mid)
	// Registered after the outer mount, but before the app is served.
	mid.Domain("api.example.com").Use("/child", child)

	domainTestExpectBody(t, app, "api.example.com", "/mid/child/boom", 596, "child error")
	domainTestExpect(t, app, "www.example.com", "/mid/child/boom", StatusNotFound)
}

// Test_Domain_UseMountParametricPrefix verifies that a parametric mount prefix
// inside a domain-mounted sub-app still matches as a pattern once the routes
// have been prefixed twice.
func Test_Domain_UseMountParametricPrefix(t *testing.T) {
	t.Parallel()
	child := New()
	child.Get("/x", func(c Ctx) error { return c.SendString("version=" + c.Params("version")) })

	subApp := New()
	subApp.Use("/v1/:version", child)

	app := routerDomainMount(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/v1/42/x", StatusOK, "version=42")
	domainTestExpect(t, app, "www.example.com", "/api/v1/42/x", StatusNotFound)
}

// Test_Domain_UseMountCaseInsensitivePath verifies that a domain mount
// registered with a differently-cased path still owns its config on a
// case-insensitive app, the way its routes do.
func Test_Domain_UseMountCaseInsensitivePath(t *testing.T) {
	t.Parallel()
	subApp := New(Config{
		Views:        domainTestEngine(t, "testdata2"),
		ErrorHandler: domainTestErrorHandler(599, "sub error"),
	})
	subApp.Get("/boom", domainTestBoom)
	subApp.Get("/view", func(c Ctx) error { return c.Render("bruh.tmpl", Map{}) })

	app := New()
	app.Domain("api.example.com").Use("/API", subApp)

	domainTestExpect(t, app, "api.example.com", "/api/boom", 599)
	domainTestExpectBody(t, app, "api.example.com", "/api/view", StatusOK, "<h1>I'm Bruh</h1>")

	// Still host-scoped: another host gets neither the route nor the config.
	domainTestExpect(t, app, "www.example.com", "/api/boom", StatusNotFound)
}

// Test_Domain_UseMountRecordedOnce verifies that a domain mount reached through
// several layers of ordinary mounts is recorded once. Each app list already
// holds the descendants of the apps it lists, so the startup walk arrives at
// the same mount by every path that leads to it.
func Test_Domain_UseMountRecordedOnce(t *testing.T) {
	t.Parallel()
	child := domainTestApp("/x", "nested x")

	inner := New()
	inner.Domain("api.example.com").Use("/child", child)

	app := inner
	for range 6 {
		outer := New()
		outer.Use("/layer", app)
		app = outer
	}

	apptest.Do(t, app, apptest.Req(MethodGet, "/unrouted", apptest.WithHost("api.example.com")))
	require.Len(t, app.mountFields.domainAppList, 1)
}

// Test_Domain_UseMountReloadViewsBeforeStart verifies that ReloadViews reaches
// a domain-mounted app nested inside an ordinary mount before the app has been
// through its startup process.
func Test_Domain_UseMountReloadViewsBeforeStart(t *testing.T) {
	t.Parallel()
	child := domainTestRenderApp("/view", "bruh.tmpl", Map{}, Config{Views: domainTestEngine(t, "testdata2")})

	mid := New()
	mid.Domain("api.example.com").Use("/child", child)

	app := routerDomainWith("/mid", mid)

	require.NoError(t, app.ReloadViews())
}

// Test_Domain_UseMountOverlappingViews verifies that when two domain mounts at
// one path both match, the one that owns the route decides which views apply:
// a later overlapping mount's engine is not borrowed for it, and the second of
// the two renders through its own engine when it owns the route. The mounts are
// equally deep and both match the host, so only the route that ran tells them
// apart.
func Test_Domain_UseMountOverlappingViews(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, want string
		exactViews       bool // the mount registered first brings an engine of its own
	}{
		// Registered first, owns the route, and configures no views of its own.
		{name: "first mount", path: "/api/view", want: "<h1>exact</h1>"},
		// Registered second, and owns the route this request runs. bruh.tmpl only
		// exists in the wildcard mount's engine, so rendering it at all is what
		// proves the engine of the mount that owns the route was used.
		{name: "later mount", exactViews: true, path: "/api/other", want: "<h1>I'm Bruh</h1>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var exactCfg Config
			if tc.exactViews {
				exactCfg.Views = domainTestEngine(t, "")
			}
			exact := domainTestRenderApp("/view", "index.tmpl", Map{"Title": "exact"}, exactCfg)
			wildcard := domainTestRenderApp("/other", "bruh.tmpl", Map{}, Config{Views: domainTestEngine(t, "testdata2")})

			app := New(Config{Views: domainTestEngine(t, "")})
			app.Domain("admin.example.com").Use("/api", exact)
			app.Domain(":tenant.example.com").Use("/api", wildcard)

			domainTestExpectBody(t, app, "admin.example.com", tc.path, StatusOK, tc.want)
		})
	}
}

// Test_Domain_UseMountOverlappingErrorHandler verifies that when two domain
// mounts at one path have patterns that both match, each sub-app still handles
// the errors of its own routes. admin.example.com matches both patterns and
// the mounts are equally deep, so a request is only attributable through the
// route that ran.
func Test_Domain_UseMountOverlappingErrorHandler(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		method string
		host   string
		path   string
		status int
	}{
		// admin.example.com matches both patterns; the route belongs to the first.
		{name: "route of the first mount", method: MethodGet, host: "admin.example.com", path: "/api/first", status: 581},
		{name: "route of the second mount", method: MethodGet, host: "admin.example.com", path: "/api/second", status: 582},
		// acme.example.com matches the second pattern only.
		{name: "route of the second mount on its own host", method: MethodGet, host: "acme.example.com", path: "/api/second", status: 582},
		// An automatic HEAD route resolves the same owner its GET route does,
		// rather than falling back to the first mount that covers the path.
		{name: "automatic HEAD route of the second mount", method: MethodHead, host: "admin.example.com", path: "/api/second", status: 582},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			first := domainTestFailingApp(581, "first", "/first")
			second := domainTestFailingApp(582, "second", "/second")

			app := New()
			app.Domain("admin.example.com").Use("/api", first)
			app.Domain(":tenant.example.com").Use("/api", second)

			apptest.Expect(t, app, apptest.Req(tc.method, tc.path, apptest.WithHost(tc.host)), tc.status)
		})
	}
}

func Test_Domain_UseMountWildcardPrefixErrorHandler(t *testing.T) {
	t.Parallel()
	sub := New(Config{ErrorHandler: func(c Ctx, _ error) error { return c.Status(581).SendString("sub") }})
	sub.Get("/:page", func(_ Ctx) error { return errors.New("boom") })

	app := New()
	app.Domain("admin.example.com").Use("/api/*/admin", sub)

	req := httptest.NewRequest(MethodGet, "/api/acme/admin/admins", http.NoBody)
	req.Host = "admin.example.com"
	apptest.Expect(t, app, req, 581)
}

func Test_Domain_UseMountParamPrefixErrorHandler(t *testing.T) {
	t.Parallel()
	sub := New(Config{ErrorHandler: func(c Ctx, _ error) error { return c.Status(581).SendString("sub") }})
	sub.Get("/raw", func(_ Ctx) error { return errors.New("boom") })

	app := New()
	app.Domain("api.example.com").Use("/files/:name.json", sub)

	req := httptest.NewRequest(MethodGet, "/files/a.json.json/raw", http.NoBody)
	req.Host = "api.example.com"
	apptest.Expect(t, app, req, 581)
}

// Test_Domain_UseMountDeclinedHostKeepsRootErrorHandler verifies that a mount
// whose handlers declined the host does not answer for the request they passed
// on: the route it registered is the one that ran, but it served nothing.
func Test_Domain_UseMountDeclinedHostKeepsRootErrorHandler(t *testing.T) {
	t.Parallel()
	subApp := domainTestFailingApp(585, "sub", "/boom")

	app := routerDomainMount(subApp)

	domainTestExpect(t, app, "other.example.com", "/api/boom", StatusNotFound)
}

// Test_Domain_UseMountBehindPlainDescendant verifies that an app domain-mounted
// on an ordinary descendant of a domain-mounted app is found: those records live
// only on the app they were registered on, and ordinary mounting does not carry
// them upward.
func Test_Domain_UseMountBehindPlainDescendant(t *testing.T) {
	t.Parallel()
	inner := domainTestFailingApp(586, "inner", "/boom")

	mid := New()
	outer := routerDomainWith("/mid", mid)
	mid.Domain("admin.example.com").Use("/child", inner)

	app := New()
	app.Domain(":tenant.example.com").Use("/api", outer)

	domainTestExpect(t, app, "admin.example.com", "/api/mid/child/boom", 586)

	// Both patterns have to match: the inner app is behind its own domain
	// router as well as the one it was reached through.
	domainTestExpect(t, app, "other.example.com", "/api/mid/child/boom", StatusNotFound)
}

// Test_Domain_UseMountStartedSubApp verifies that a sub-app which has already
// expanded its own mounts keeps its children as the owners of their routes when
// it is domain-mounted afterwards, rather than being credited with all of them.
func Test_Domain_UseMountStartedSubApp(t *testing.T) {
	t.Parallel()
	child := domainTestFailingApp(587, "child", "/boom")

	subApp := New(Config{ErrorHandler: domainTestErrorHandler(588, "sub")})
	subApp.Use("/mid", child)

	// Run the sub-app on its own first, which replaces its mount placeholders
	// with the child's routes.
	apptest.Do(t, subApp, "/mid/boom")

	app := routerDomainMount(subApp)

	domainTestExpect(t, app, "api.example.com", "/api/mid/boom", 587)
}

// Test_Domain_UseMountLateOrdinaryDescendant verifies that an app mounted on a
// descendant after that descendant was itself mounted is still found: the
// flattened list is filled in as apps are mounted, so only the mount metadata
// of each app in turn is current.
func Test_Domain_UseMountLateOrdinaryDescendant(t *testing.T) {
	t.Parallel()
	child := domainTestFailingApp(589, "child", "/boom")

	mid := New()
	outer := routerDomainWith("/mid", mid)
	mid.Use("/child", child) // mounted after mid itself was

	app := routerDomainMount(outer)

	domainTestExpect(t, app, "api.example.com", "/api/mid/child/boom", 589)
}

// Test_Domain_UseMountViewsSkipsPlainSibling verifies that a domain mount
// without an engine of its own renders through the engine enclosing it rather
// than through a plain mount at its own path, which did not serve the request.
func Test_Domain_UseMountViewsSkipsPlainSibling(t *testing.T) {
	t.Parallel()
	// Registered at the same path, and holds no template the domain mount asks
	// for: borrowing it would fail rather than render the wrong page.
	plain := domainTestRenderApp("/other", "bruh.tmpl", Map{}, Config{Views: domainTestEngine(t, "testdata2")})

	subApp := domainTestRenderApp("/view", "index.tmpl", Map{"Title": "domain"})

	app := New(Config{Views: domainTestEngine(t, "")})
	app.Use("/api", plain)
	app.Domain("api.example.com").Use("/api", subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/view", StatusOK, "<h1>domain</h1>")
}

// Test_Domain_UseMountInheritsEnclosingErrorHandler verifies that a domain
// mount configuring no error handler inherits the one of the app it is mounted
// inside, as an ordinarily nested mount does.
func Test_Domain_UseMountInheritsEnclosingErrorHandler(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mount  string
		path   string
		status int
	}{
		{name: "below the enclosing mount", mount: "/child", path: "/mid/child/boom", status: 590},
		// A mount as deep as the owner is only passed over when it is beside it.
		// A domain mount at the root of an ordinary one covers exactly the paths
		// that mount does, and its error handler is still the one enclosing the
		// request.
		{name: "as deep as the enclosing mount", mount: "/", path: "/mid/boom", status: 607},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			child := New() // no handler of its own
			child.Get("/boom", domainTestBoom)

			mid := New(Config{ErrorHandler: domainTestErrorHandler(tc.status, "mid")})
			mid.Domain("api.example.com").Use(tc.mount, child)

			app := New()
			app.Use("/mid", mid)

			domainTestExpect(t, app, "api.example.com", tc.path, tc.status)
		})
	}
}

// Test_Domain_UseMountDropsPlainSiblingErrorHandler verifies that a domain
// mount configuring no error handler falls through to the app enclosing it
// rather than to a plain mount at its own path, which did not serve the
// request.
func Test_Domain_UseMountDropsPlainSiblingErrorHandler(t *testing.T) {
	t.Parallel()
	plain := domainTestFailingApp(592, "plain", "/other")

	subApp := New()
	subApp.Get("/boom", domainTestBoom)

	app := New(Config{ErrorHandler: domainTestErrorHandler(593, "root")})
	app.Use("/api", plain)
	app.Domain("api.example.com").Use("/api", subApp)

	domainTestExpect(t, app, "api.example.com", "/api/boom", 593)
}

// Test_Domain_UseMountCyclicOrdinaryMounts verifies that domain-mounting an app
// whose ordinary mounts form a cycle terminates and still serves its routes.
func Test_Domain_UseMountCyclicOrdinaryMounts(t *testing.T) {
	t.Parallel()
	first := New()
	second := domainTestApp("/ping", "pong")

	first.Use("/second", second)
	second.Use("/first", first)

	app := routerDomainMount(first)

	domainTestExpectBody(t, app, "api.example.com", "/api/second/ping", StatusOK, "pong")
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
	// mounted first, so its HEAD route runs ahead of the GET route's on its hostnames
	outer.Domain("admin.example.com").Use("/", inner)
	outer.Get("/x", func(c Ctx) error {
		c.Set("X-Handler", "outer")
		return nil
	})

	app := New()
	app.Domain(":tenant.example.com").Use("/api", outer)

	for _, tc := range []struct {
		host string
		want string
	}{
		{host: "admin.example.com", want: "inner"}, // matches both patterns
		{host: "other.example.com", want: "outer"}, // only the outer one
	} {
		resp := routerDomainExpectMethod(t, app, MethodHead, tc.host, "/api/x", StatusOK)
		require.Equal(t, tc.want, resp.Header.Get("X-Handler"), tc.host)
	}
}

// Test_Domain_UseMountInheritsDomainErrorHandler verifies that an app mounted
// inside a domain-mounted one inherits its error handler when it configures
// none, as an ordinarily nested mount does.
func Test_Domain_UseMountInheritsDomainErrorHandler(t *testing.T) {
	t.Parallel()
	child := New()
	child.Get("/boom", domainTestBoom)

	subApp := New(Config{ErrorHandler: domainTestErrorHandler(594, "sub")})
	subApp.Use("/child", child)

	app := routerDomainMount(subApp, Config{ErrorHandler: domainTestErrorHandler(595, "root")})

	domainTestExpect(t, app, "api.example.com", "/api/child/boom", 594)
}

// Test_Domain_UseMountInheritsOnlyFromCoveringMounts verifies that inheritance
// reaches the mounts a request passed through, not every shallower one: a
// domain mount on another path encloses nothing here.
func Test_Domain_UseMountInheritsOnlyFromCoveringMounts(t *testing.T) {
	t.Parallel()
	other := domainTestApp("/thing", "thing", Config{ErrorHandler: domainTestErrorHandler(596, "other")})

	child := New()
	child.Get("/boom", domainTestBoom)

	subApp := routerDomainWith("/child", child)

	app := New(Config{ErrorHandler: domainTestErrorHandler(597, "root")})
	app.Domain("api.example.com").Use("/other", other)
	app.Domain("api.example.com").Use("/api", subApp)

	domainTestExpect(t, app, "api.example.com", "/api/child/boom", 597)
}

// Test_Domain_UseMountIgnoresOverlappingSibling verifies that inheritance
// follows the mounts an app is registered inside, not every shallower one that
// happens to reach the same request: a separately registered mount overlapping
// by host and path encloses nothing.
func Test_Domain_UseMountIgnoresOverlappingSibling(t *testing.T) {
	t.Parallel()
	wildcard := domainTestApp("/thing", "thing", Config{ErrorHandler: domainTestErrorHandler(598, "wildcard")})

	exact := New() // no handler of its own
	exact.Get("/boom", domainTestBoom)

	app := New(Config{ErrorHandler: domainTestErrorHandler(599, "root")})
	app.Domain(":tenant.example.com").Use("/api", wildcard)
	app.Domain("admin.example.com").Use("/api/child", exact)

	// admin.example.com matches both patterns
	domainTestExpect(t, app, "admin.example.com", "/api/child/boom", 599)
}

// Test_Domain_UseMountPlainRouteKeepsItsOwnConfig verifies that a domain mount
// does not answer for a route an ordinary mount served, even where it covers
// the same prefix on a matching host.
func Test_Domain_UseMountPlainRouteKeepsItsOwnConfig(t *testing.T) {
	t.Parallel()
	plain := New() // no handler of its own
	plain.Get("/boom", domainTestBoom)

	subApp := domainTestFailingApp(601, "domain", "/other")

	app := New(Config{ErrorHandler: domainTestErrorHandler(602, "root")})
	app.Use("/api", plain)
	app.Domain("api.example.com").Use("/api", subApp)

	domainTestExpect(t, app, "api.example.com", "/api/boom", 602)

	// The domain mount still answers for the routes it did serve.
	domainTestExpect(t, app, "api.example.com", "/api/other", 601)
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
	child := domainTestApp("/x", "x", Config{ErrorHandler: domainTestErrorHandler(603, "child")})

	mid := New()
	mid.RegisterCustomConstraint(&onlyFooConstraint{})
	mid.Use("/:name<onlyfoo>", child)

	outer := routerDomainWith("/mid", mid)

	app := routerDomainMount(outer, Config{ErrorHandler: domainTestErrorHandler(604, "root")})

	// The mount covers the path the constraint accepts, and nothing else.
	domainTestExpect(t, app, "api.example.com", "/api/mid/foo/nope", 603)
	domainTestExpect(t, app, "api.example.com", "/api/mid/bar/nope", 604)
}

// Test_Domain_UseMountPrefixAsWritten verifies that a domain mount is recorded
// with its prefix as written, so its config answers for the paths its routes match.
func Test_Domain_UseMountPrefixAsWritten(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		prefix  string
		covered string
		outside string
	}{
		{prefix: `/v1\:internal`, covered: "/v1:internal", outside: "/v1foo"},
		{prefix: `/a\*`, covered: "/a*", outside: "/a/b"},
		{prefix: "/even/:n<isEven>", covered: "/even/4", outside: "/even/3"},
		{prefix: "/code/:code<regex(^[A-Z]{2}$)>", covered: "/code/US", outside: "/code/us"},
		{prefix: `/num/:n<regex(^\d+$)>`, covered: "/num/12", outside: "/num/dd"},
	}

	mounts := map[string]func(app *App, prefix string, child *App){
		"domain": func(app *App, prefix string, child *App) { app.Domain("example.com").Use(prefix, child) },
		"domain in app": func(app *App, prefix string, child *App) {
			mid := New()
			mid.RegisterCustomConstraint(&evenConstraint{})
			mid.Domain("example.com").Use(prefix, child)
			app.Use("/", mid)
		},
		"domain in domain": func(app *App, prefix string, child *App) {
			mid := New()
			mid.RegisterCustomConstraint(&evenConstraint{})
			mid.Domain("example.com").Use(prefix, child)
			app.Domain("example.com").Use("/", mid)
		},
	}

	for name, mount := range mounts {
		for _, tc := range testCases {
			t.Run(name+" "+tc.prefix, func(t *testing.T) {
				t.Parallel()
				child := New(Config{ErrorHandler: func(c Ctx, _ error) error {
					return c.Status(607).SendString("child")
				}})
				child.Get("/boom", func(_ Ctx) error { return errors.New("boom") })

				app := New(Config{ErrorHandler: func(c Ctx, _ error) error { return c.Status(608).SendString("root") }})
				app.RegisterCustomConstraint(&evenConstraint{})
				mount(app, tc.prefix, child)

				for path, want := range map[string]int{
					tc.covered + "/boom": 607,
					tc.covered + "/nope": 607,
					tc.outside + "/boom": 608,
					tc.outside + "/nope": 608,
				} {
					apptest.Expect(t, app, path, want)
				}
			})
		}
	}
}

// Test_Domain_UseMountAncestryIsStable verifies that a mount reached both
// through its own parent and as an entry of the app above it keeps the ancestry
// of the former, whichever the unordered walk happens to record first.
func Test_Domain_UseMountAncestryIsStable(t *testing.T) {
	t.Parallel()
	for range 40 {
		child := New()
		child.Get("/boom", domainTestBoom)

		mid := New(Config{ErrorHandler: domainTestErrorHandler(605, "mid")})
		mid.Use("/child", child)

		outer := New()
		outer.Use("/mid", mid)

		app := routerDomainMount(outer, Config{ErrorHandler: domainTestErrorHandler(606, "root")})

		domainTestExpect(t, app, "api.example.com", "/api/mid/child/boom", 605)
	}
}

// Test_Domain_UseMountLayoutStopsAtEngine verifies that a mount rendering
// through an engine of its own takes no layout from the mount enclosing it, as
// an ordinary mount does not: the scan stops at the first engine covering the
// request either way.
func Test_Domain_UseMountLayoutStopsAtEngine(t *testing.T) {
	t.Parallel()
	child := domainTestRenderApp("/view", "index.tmpl", Map{"Title": "child"},
		Config{Views: domainTestEngine(t, "")}) // an engine, and no layout

	subApp := New(Config{Views: domainTestEngine(t, ""), ViewsLayout: "main.tmpl"})
	subApp.Use("/child", child)

	app := routerDomainMount(subApp)

	domainTestExpectBody(t, app, "api.example.com", "/api/child/view", StatusOK, "<h1>child</h1>")
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

	app := routerDomainMount(subApp)

	routerDomainExpectMethod(t, app, MethodHead, "api.example.com", "/api/top", StatusOK)
	routerDomainExpectMethod(t, app, MethodHead, "api.example.com", "/api/v1/deep", StatusMethodNotAllowed)
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
	shared := domainTestFailingApp(608, "shared", "/boom")

	subApp := New()
	subApp.Domain("a.example.com").Use("/", shared)
	subApp.Domain("b.example.com").Use("/", shared)

	app := New()
	app.Domain(":sub.example.com").Use("/api", subApp)

	domainTestExpect(t, app, "a.example.com", "/api/boom", 608)
	domainTestExpect(t, app, "b.example.com", "/api/boom", 608)
}

// Test_Domain_UseMountOutranksDeeperUnrelatedMount verifies that the app a route
// was mounted from answers for it even where an unrelated plain mount reaches
// deeper: that mount serves none of these routes, and depth only stands in for
// ownership where nothing better is known.
func Test_Domain_UseMountOutranksDeeperUnrelatedMount(t *testing.T) {
	t.Parallel()
	subApp := domainTestFailingApp(609, "domain", "/admin/boom")

	plain := domainTestApp("/other", "other", Config{ErrorHandler: domainTestErrorHandler(610, "plain")})

	app := routerDomainMount(subApp)
	app.Use("/api/admin", plain)

	domainTestExpect(t, app, "api.example.com", "/api/admin/boom", 609)

	// The deeper mount still answers for the routes it does serve.
	domainTestExpect(t, app, "api.example.com", "/api/admin/missing", 610)
}

// Test_Domain_UseMountInheritsDomainViews verifies the same inheritance for the
// view engine: a child of a domain-mounted app renders through that app's
// engine rather than the root's.
func Test_Domain_UseMountInheritsDomainViews(t *testing.T) {
	t.Parallel()
	child := domainTestRenderApp("/view", "index.tmpl", Map{"Title": "child"})

	subApp := New(Config{Views: domainTestEngine(t, "")})
	subApp.Use("/child", child)

	// Holds no template the child asks for, so borrowing it would fail.
	app := routerDomainMount(subApp, Config{Views: domainTestEngine(t, "testdata2")})

	domainTestExpectBody(t, app, "api.example.com", "/api/child/view", StatusOK, "<h1>child</h1>")
}

// Test_Domain_UseMountKeepsParentAutoHead verifies that a domain mount which
// registers no automatic HEAD routes only withholds them from its own routes,
// leaving a route the parent registered at the same path with its own.
func Test_Domain_UseMountKeepsParentAutoHead(t *testing.T) {
	t.Parallel()
	subApp := domainTestApp("/x", "sub x", Config{DisableHeadAutoRegister: true})

	app := New()
	app.Get("/api/x", func(c Ctx) error { return c.SendString("parent x") })
	app.Domain("api.example.com").Use("/api", subApp)

	// the parent's own route keeps its HEAD route
	routerDomainExpectMethod(t, app, MethodHead, "www.example.com", "/api/x", StatusOK)
}

// Test_Domain_UseMountParametricMountPath verifies that a domain mount
// registered at a parametric path owns the requests its routes serve, so its
// error handler and views apply to them.
func Test_Domain_UseMountParametricMountPath(t *testing.T) {
	t.Parallel()
	subApp := domainTestFailingApp(571, "sub error", "/boom")

	app := New()
	app.Domain("api.example.com").Use("/:tenant", subApp)

	domainTestExpect(t, app, "api.example.com", "/acme/boom", 571)
}

// Test_Domain_UseMountRootIsShallowest verifies that a mount at the root is
// the least specific of the mounts covering a request, so a named mount
// registered after it still owns its own routes.
func Test_Domain_UseMountRootIsShallowest(t *testing.T) {
	t.Parallel()
	rootApp := domainTestFailingApp(572, "root mount", "/other")
	apiApp := domainTestFailingApp(573, "api mount", "/boom")

	app := New()
	app.Domain("api.example.com").Use("/", rootApp)
	app.Domain("api.example.com").Use("/api", apiApp)

	domainTestExpect(t, app, "api.example.com", "/api/boom", 573)
}

// Test_Domain_UseMountViewsRankedAgainstPlain verifies that a domain mount and
// an ordinary one are ranked against each other by mount depth, so the deeper
// of the two renders with its own engine.
func Test_Domain_UseMountViewsRankedAgainstPlain(t *testing.T) {
	t.Parallel()
	domainApp := domainTestRenderApp("/x", "bruh.tmpl", Map{}, Config{Views: domainTestEngine(t, "testdata2")})

	plainApp := domainTestRenderApp("/view", "hello_world.tmpl", Map{"Name": "deep"},
		Config{Views: domainTestEngine(t, "testdata3")})

	app := routerDomainMount(domainApp)
	app.Use("/api/admin", plainApp)

	// The deeper plain mount owns this one.
	domainTestExpectBody(t, app, "api.example.com", "/api/admin/view", StatusOK, "<h1>Hello deep!</h1>")

	// And the domain mount still owns its own.
	domainTestExpectBody(t, app, "api.example.com", "/api/x", StatusOK, "<h1>I'm Bruh</h1>")
}

// Test_Domain_UseMountNestedCycle verifies that cloning a sub-app whose mount
// graph contains a cycle terminates, and stops at the point the cycle closes.
// It drives the clone directly: such an app cannot be served, because the
// parent's startup loops in appendSubAppLists, independently of domain routing.
func Test_Domain_UseMountNestedCycle(t *testing.T) {
	t.Parallel()
	first := New()
	second := domainTestApp("/x", "nested x")
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
	t.Run("total length exceeds 253", func(t *testing.T) {
		t.Parallel()
		// 250 chars of 'a' + ".com" = 254 chars total > 253
		longPattern := strings.Repeat("a", 250) + ".com"
		require.Greater(t, len(longPattern), 253)
		require.Panics(t, func() { parseDomainPattern(longPattern) })
	})

	t.Run("label exceeds 63 chars", func(t *testing.T) {
		t.Parallel()
		longLabel := strings.Repeat("a", 64)
		require.Panics(t, func() { parseDomainPattern(longLabel + ".example.com") })
	})

	t.Run("253 chars total is valid", func(t *testing.T) {
		t.Parallel()
		label63 := strings.Repeat("a", 63)
		pattern := label63 + "." + label63 + "." + label63 + "." + strings.Repeat("b", 59)
		require.LessOrEqual(t, len(pattern), 253)
		require.NotPanics(t, func() { parseDomainPattern(pattern) })
	})

	t.Run("63 char label is valid", func(t *testing.T) {
		t.Parallel()
		label63 := strings.Repeat("a", 63)
		require.NotPanics(t, func() { parseDomainPattern(label63 + ".com") })
	})
}
