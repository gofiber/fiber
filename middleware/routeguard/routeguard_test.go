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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/bypass"
		},
	}))
	Build(app)

	req := httptest.NewRequest("GET", "/bypass", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("got %d, want 404", resp.StatusCode)
	}
}

func TestCustomErrorHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: func(c fiber.Ctx) error {
			return c.Status(418).SendString("teapot")
		},
	}))
	Build(app)

	req := httptest.NewRequest("GET", "/nope", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 418 {
		t.Errorf("got %d, want 418", resp.StatusCode)
	}
}

func TestCaseInsensitive(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{CaseSensitive: false})
	app.Use(New())
	app.Get("/Api/Users", func(c fiber.Ctx) error { return c.SendString("ok") })
	Build(app)

	cases := []struct {
		path string
		want int
	}{
		{"/Api/Users", 200},
		{"/api/users", 200},
		{"/API/USERS", 200},
		{"/ApI/uSeRs", 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d", tc.path, resp.StatusCode, tc.want)
		}
	}
}

func TestCaseSensitive(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{CaseSensitive: true})
	app.Use(New())
	app.Get("/Api/Users", func(c fiber.Ctx) error { return c.SendString("ok") })
	Build(app)

	cases := []struct {
		path string
		want int
	}{
		{"/Api/Users", 200},
		{"/api/users", 404},
		{"/API/USERS", 404},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d", tc.path, resp.StatusCode, tc.want)
		}
	}
}

func TestStrictRouting(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{StrictRouting: true})
	app.Use(New())
	app.Get("/users", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/items/", func(c fiber.Ctx) error { return c.SendString("ok") })
	Build(app)

	cases := []struct {
		path string
		want int
	}{
		{"/users", 200},
		{"/users/", 404},
		{"/items/", 200},
		{"/items", 404},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d", tc.path, resp.StatusCode, tc.want)
		}
	}
}

func TestNonStrictRouting(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{StrictRouting: false})
	app.Use(New())
	app.Get("/users", func(c fiber.Ctx) error { return c.SendString("ok") })
	Build(app)

	cases := []struct {
		path string
		want int
	}{
		{"/users", 200},
		{"/users/", 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d", tc.path, resp.StatusCode, tc.want)
		}
	}
}

func TestMultiAppIsolation(t *testing.T) {
	t.Parallel()
	app1 := fiber.New()
	app1.Use(New())
	app1.Get("/app1-only", func(c fiber.Ctx) error { return c.SendString("app1") })
	Build(app1)

	app2 := fiber.New()
	app2.Use(New())
	app2.Get("/app2-only", func(c fiber.Ctx) error { return c.SendString("app2") })
	Build(app2)

	req1 := httptest.NewRequest("GET", "/app1-only", nil)
	resp1, err := app1.Test(req1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp1.StatusCode != 200 {
		t.Errorf("app1 /app1-only: got %d, want 200", resp1.StatusCode)
	}

	req2 := httptest.NewRequest("GET", "/app2-only", nil)
	resp2, err := app1.Test(req2)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp2.StatusCode != 404 {
		t.Errorf("app1 /app2-only: got %d, want 404", resp2.StatusCode)
	}

	req3 := httptest.NewRequest("GET", "/app2-only", nil)
	resp3, err := app2.Test(req3)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp3.StatusCode != 200 {
		t.Errorf("app2 /app2-only: got %d, want 200", resp3.StatusCode)
	}

	req4 := httptest.NewRequest("GET", "/app1-only", nil)
	resp4, err := app2.Test(req4)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp4.StatusCode != 404 {
		t.Errorf("app2 /app1-only: got %d, want 404", resp4.StatusCode)
	}
}

func TestRouteguardCaseInsensitive(t *testing.T) {
	app := fiber.New(fiber.Config{
		CaseSensitive: false,
	})
	app.Use(New())
	app.Get("/API/Users", func(c fiber.Ctx) error { return c.SendString("ok") })
	Build(app)

	req := httptest.NewRequest("GET", "/api/users", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRouteguardStrictRouting(t *testing.T) {
	app := fiber.New(fiber.Config{
		StrictRouting: true,
	})
	app.Use(New())
	app.Get("/api/users/", func(c fiber.Ctx) error { return c.SendString("ok") })
	Build(app)

	// Should match with trailing slash
	req1 := httptest.NewRequest("GET", "/api/users/", nil)
	resp1, _ := app.Test(req1)
	if resp1.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp1.StatusCode)
	}

	// Should NOT match without trailing slash
	req2 := httptest.NewRequest("GET", "/api/users", nil)
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp2.StatusCode)
	}
}

func TestRouteguardMultiAppIsolation(t *testing.T) {
	app1 := fiber.New()
	app1.Use(New())
	app1.Get("/app1", func(c fiber.Ctx) error { return c.SendString("app1") })
	Build(app1)

	app2 := fiber.New()
	app2.Use(New())
	app2.Get("/app2", func(c fiber.Ctx) error { return c.SendString("app2") })
	Build(app2)

	// app1 should match /app1 but not /app2
	req1 := httptest.NewRequest("GET", "/app1", nil)
	resp1, _ := app1.Test(req1)
	if resp1.StatusCode != 200 {
		t.Errorf("app1: expected 200 for /app1, got %d", resp1.StatusCode)
	}

	req2 := httptest.NewRequest("GET", "/app2", nil)
	resp2, _ := app1.Test(req2)
	if resp2.StatusCode != 404 {
		t.Errorf("app1: expected 404 for /app2, got %d", resp2.StatusCode)
	}

	// app2 should match /app2 but not /app1
	req3 := httptest.NewRequest("GET", "/app2", nil)
	resp3, _ := app2.Test(req3)
	if resp3.StatusCode != 200 {
		t.Errorf("app2: expected 200 for /app2, got %d", resp3.StatusCode)
	}

	req4 := httptest.NewRequest("GET", "/app1", nil)
	resp4, _ := app2.Test(req4)
	if resp4.StatusCode != 404 {
		t.Errorf("app2: expected 404 for /app1, got %d", resp4.StatusCode)
	}
}

func getRouter(app *fiber.App) *Router {
	r, _ := app.State().Get(stateKey)
	return r.(*Router)
}

func BenchmarkTrieLookup(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("POST", "/api/v1/contacts/abc-uuid-1234/companies")
	}
}

func BenchmarkTrieMiss(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/api/v1/totally/nonexistent/path")
	}
}

func BenchmarkStaticShort(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/health")
	}
}

func BenchmarkStaticDeep(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/api/v1/contacts/list")
	}
}

func BenchmarkRootPath(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/")
	}
}

func BenchmarkSingleParam(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/api/v1/contacts/uuid-12345")
	}
}

func BenchmarkMultipleParams(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/api/v1/orgs/org123/teams/team456/members/member789")
	}
}

func BenchmarkTripleParams(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/api/v1/users/u1/posts/p2/comments/c3")
	}
}

func BenchmarkWildcardShort(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/swagger/index.html")
	}
}

func BenchmarkWildcardDeep(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/docs/a/b/c/d/e/f/g.md")
	}
}

func BenchmarkNestedWildcard(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/assets/images/icons/dark/large/icon.svg")
	}
}

func BenchmarkStaticVsParamPriority(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/api/v1/items/special")
	}
}

func BenchmarkHeadFallback(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("HEAD", "/api/v1/contacts/123")
	}
}

func BenchmarkMethodVariation(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup(methods[i%4], "/resource")
	}
}

func BenchmarkLongPath(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/api/v1/files/very/long/nested/path/to/some/file.json")
	}
}

func BenchmarkEarlyMiss(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("GET", "/unknown")
	}
}

func BenchmarkLateMiss(b *testing.B) {
	app := newTestApp()
	r := getRouter(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.lookup("POST", "/api/v1/contacts/123/unknown/extra")
	}
}
