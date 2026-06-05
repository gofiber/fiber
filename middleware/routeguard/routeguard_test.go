package routeguard

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func newTestApp() *fiber.App {
	app := fiber.New()
	app.Use(New())

	h := func(c fiber.Ctx) error { return c.SendString("ok") }

	app.Get("/health", h)
	app.Get("/swagger/*", h)
	app.Get("/public/forms/:slug", h)

	api := app.Group("/api")
	api.Get("/cdn", h)
	api.Post("/cdn/upload-file", h)

	v1 := api.Group("/v1")
	v1.Get("/me", h)
	v1.Get("/contacts", h)
	v1.Get("/contacts/list", h)
	v1.Get("/contacts/:id", h)
	v1.Post("/contacts/:contactId/companies", h)

	// Additional HTTP methods
	v1.Put("/contacts/:id", h)
	v1.Delete("/contacts/:id", h)
	v1.Patch("/contacts/:id", h)
	v1.Options("/contacts", h)

	// Deeper nesting
	v1.Get("/orgs/:orgId/teams/:teamId/members", h)
	v1.Get("/orgs/:orgId/teams/:teamId/members/:memberId", h)
	v1.Post("/orgs/:orgId/teams/:teamId/members", h)
	v1.Delete("/orgs/:orgId/teams/:teamId/members/:memberId", h)

	// Multiple params in path
	v1.Get("/users/:userId/posts/:postId/comments/:commentId", h)
	v1.Put("/users/:userId/posts/:postId", h)

	// Static vs param priority
	v1.Get("/items/special", h)
	v1.Get("/items/:itemId", h)
	v1.Get("/items/:itemId/details", h)

	// Wildcards at different depths
	app.Get("/docs/*", h)
	app.Get("/assets/images/*", h)
	v1.Get("/files/*", h)

	// Root and short paths
	app.Get("/", h)
	app.Get("/a", h)
	app.Get("/a/b", h)
	app.Get("/a/b/c", h)

	// Mixed methods on same path
	app.Get("/resource", h)
	app.Post("/resource", h)
	app.Put("/resource", h)
	app.Delete("/resource", h)

	Build(app)
	return app
}

func TestRouteguard(t *testing.T) {
	app := newTestApp()

	cases := []struct {
		name           string
		method, path   string
		wantStatusCode int
	}{
		// Basic static routes
		{"static hit", "GET", "/health", 200},
		{"nested static", "GET", "/api/v1/me", 200},
		{"static beats param", "GET", "/api/v1/contacts/list", 200},
		{"trailing slash", "GET", "/health/", 200},
		{"root path", "GET", "/", 200},
		{"single char path", "GET", "/a", 200},
		{"multi level static", "GET", "/a/b/c", 200},

		// Param routes
		{"string param", "GET", "/api/v1/contacts/abc-uuid", 200},
		{"numeric param", "GET", "/api/v1/contacts/42", 200},
		{"param with special chars", "GET", "/api/v1/contacts/user%40email.com", 200},

		// Multiple params
		{"two params deep", "GET", "/api/v1/orgs/org123/teams/team456/members", 200},
		{"three params deep", "GET", "/api/v1/orgs/org1/teams/team2/members/member3", 200},
		{"triple nested params", "GET", "/api/v1/users/u1/posts/p2/comments/c3", 200},

		// Static vs param priority
		{"static priority over param", "GET", "/api/v1/items/special", 200},
		{"param fallback", "GET", "/api/v1/items/random-id", 200},
		{"param with child static", "GET", "/api/v1/items/xyz/details", 200},

		// Wildcards
		{"wildcard one seg", "GET", "/swagger/index.html", 200},
		{"wildcard many segs", "GET", "/swagger/a/b/c.json", 200},
		{"docs wildcard", "GET", "/docs/guide/intro.md", 200},
		{"nested wildcard", "GET", "/assets/images/logo/dark/icon.png", 200},
		{"api wildcard", "GET", "/api/v1/files/path/to/file.txt", 200},

		// HTTP methods
		{"head fallback to get", "HEAD", "/health", 200},
		{"put method", "PUT", "/api/v1/contacts/123", 200},
		{"delete method", "DELETE", "/api/v1/contacts/456", 200},
		{"patch method", "PATCH", "/api/v1/contacts/789", 200},
		{"options method", "OPTIONS", "/api/v1/contacts", 200},
		{"post on nested", "POST", "/api/v1/orgs/o1/teams/t1/members", 200},
		{"delete on nested", "DELETE", "/api/v1/orgs/o1/teams/t1/members/m1", 200},

		// Mixed methods same path
		{"resource get", "GET", "/resource", 200},
		{"resource post", "POST", "/resource", 200},
		{"resource put", "PUT", "/resource", 200},
		{"resource delete", "DELETE", "/resource", 200},

		// Error cases
		{"wrong method", "POST", "/health", 404},
		{"unknown route", "GET", "/api/v1/garbage", 404},
		{"extra segment", "GET", "/api/v1/contacts/abc/garbage", 404},
		{"unknown prefix", "GET", "/totally/random", 404},
		{"method not allowed on param", "PATCH", "/api/v1/items/special", 404},
		{"deep path miss", "GET", "/api/v1/orgs/o1/teams/t1/unknown", 404},
		{"partial match fail", "GET", "/api/v1/users/u1/posts", 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			if resp.StatusCode != tc.wantStatusCode {
				t.Errorf("%s %s: got %d, want %d",
					tc.method, tc.path, resp.StatusCode, tc.wantStatusCode)
			}
		})
	}
}

func TestNextSkipsMiddleware(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/bypass"
		},
	}))

	req := httptest.NewRequest("GET", "/bypass", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("got %d, want 404", resp.StatusCode)
	}
}

func TestCustomErrorHandler(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: func(c fiber.Ctx) error {
			return c.Status(418).SendString("teapot")
		},
	}))

	req := httptest.NewRequest("GET", "/nope", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 418 {
		t.Errorf("got %d, want 418", resp.StatusCode)
	}
}

func BenchmarkTrieLookup(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/contacts/abc-uuid-1234/companies")
	}
}

func BenchmarkTrieMiss(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/totally/nonexistent/path")
	}
}

func BenchmarkStaticShort(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/health")
	}
}

func BenchmarkStaticDeep(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/contacts/list")
	}
}

func BenchmarkRootPath(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/")
	}
}

func BenchmarkSingleParam(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/contacts/uuid-12345")
	}
}

func BenchmarkMultipleParams(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/orgs/org123/teams/team456/members/member789")
	}
}

func BenchmarkTripleParams(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/users/u1/posts/p2/comments/c3")
	}
}

func BenchmarkWildcardShort(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/swagger/index.html")
	}
}

func BenchmarkWildcardDeep(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/docs/a/b/c/d/e/f/g.md")
	}
}

func BenchmarkNestedWildcard(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/assets/images/icons/dark/large/icon.svg")
	}
}

func BenchmarkStaticVsParamPriority(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/items/special")
	}
}

func BenchmarkHeadFallback(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("HEAD", "/api/v1/contacts/123")
	}
}

func BenchmarkMethodVariation(b *testing.B) {
	newTestApp()
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup(methods[i%4], "/resource")
	}
}

func BenchmarkLongPath(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/files/very/long/nested/path/to/some/file.json")
	}
}

func BenchmarkEarlyMiss(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/unknown")
	}
}

func BenchmarkLateMiss(b *testing.B) {
	newTestApp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.lookup("GET", "/api/v1/contacts/123/unknown/extra")
	}
}
