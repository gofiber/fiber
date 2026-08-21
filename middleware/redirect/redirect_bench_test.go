package redirect

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// benchRules builds n non-overlapping rules, the last of which is the one a
// benchmark request matches.
func benchRules(n int) []Rule {
	rules := make([]Rule, 0, n)
	for i := range n {
		id := strconv.Itoa(i)
		rules = append(rules, Rule{From: "/old" + id + "/*", To: "/new" + id + "/$1"})
	}
	return rules
}

// Benchmark_Redirect_New measures building the middleware, which is where rule
// order is settled. It is parameterized over the rule count because that is
// what a per-rule startup cost shows up in.
func Benchmark_Redirect_New(b *testing.B) {
	for _, n := range []int{1, 10, 100, 500} {
		b.Run(strconv.Itoa(n)+"-rules", func(b *testing.B) {
			rules := benchRules(n)
			b.ReportAllocs()
			for b.Loop() {
				New(Config{RuleList: rules})
			}
		})
	}
}

// Benchmark_Redirect_NewDeprecatedMap is the same over the deprecated map,
// which has to be ordered before it can be used.
func Benchmark_Redirect_NewDeprecatedMap(b *testing.B) {
	for _, n := range []int{1, 10, 100, 500} {
		b.Run(strconv.Itoa(n)+"-rules", func(b *testing.B) {
			rules := make(map[string]string, n)
			for _, rule := range benchRules(n) {
				rules[rule.From] = rule.To
			}
			b.ReportAllocs()
			for b.Loop() {
				New(Config{Rules: rules})
			}
		})
	}
}

// Benchmark_Redirect_Handler measures the request path over the target shapes
// that cost different amounts to compose.
func Benchmark_Redirect_Handler(b *testing.B) {
	for _, tc := range []struct {
		name    string
		request string
		rules   []Rule
	}{
		{name: "static", rules: []Rule{{From: "/old", To: "/new"}}, request: "/old"},
		{name: "one-capture", rules: []Rule{{From: "/api/*", To: "/v2/$1"}}, request: "/api/users"},
		{name: "two-captures", rules: []Rule{{From: "/users/*/orders/*", To: "/user/$1/order/$2"}}, request: "/users/7/orders/9"},
		{name: "guarded-host", rules: []Rule{{From: "/cdn/*", To: "https://$1.cdn.example.com/"}}, request: "/cdn/images"},
		{name: "with-query", rules: []Rule{{From: "/api/*", To: "/v2/$1"}}, request: "/api/users?a=1&b=2"},
		{name: "miss", rules: benchRules(50), request: "/nothing/here"},
		{name: "last-of-50", rules: benchRules(50), request: "/old49/thing"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			app := fiber.New()
			app.Use(New(Config{RuleList: tc.rules}))
			app.Get("/*", func(c fiber.Ctx) error {
				return c.SendString("fell through")
			})

			b.ReportAllocs()
			for b.Loop() {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tc.request, http.NoBody))
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close() //nolint:errcheck // the body is empty and the error says nothing here
			}
		})
	}
}
