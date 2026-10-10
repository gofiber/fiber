package openapi

import (
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func benchApp(routes int) *fiber.App {
	app := fiber.New(fiber.Config{StructValidator: stubValidator{}})
	users := app.Group("/users", chainKeyAuth())
	for i := range routes {
		path := "/r" + strconv.Itoa(i) + "/:id"
		route := users.Get(path, listUsers)
		if i%2 == 0 {
			route.Returns(fiber.StatusOK, modelUser{}).Params("query", modelPaging{})
		}
	}
	users.Post("/create", listUsers).Accepts(modelUser{}).Returns(fiber.StatusCreated, modelUser{})
	return app
}

func Benchmark_OpenAPI_GenerateSpec(b *testing.B) {
	app := benchApp(200)
	cfg := configDefault()
	env := specEnv{validator: stubValidator{}, equal: stringsEqual}
	routes := app.GetRoutes(false)
	b.ReportAllocs()

	var spec openAPISpec
	for b.Loop() {
		spec = generateSpec(routes, &cfg, env)
	}
	require.NotEmpty(b, spec.Paths)
}

func Benchmark_OpenAPI_SchemaOf(b *testing.B) {
	b.ReportAllocs()

	var schema map[string]any
	for b.Loop() {
		schema = SchemaOf(modelUser{})
	}
	require.Equal(b, "object", schema["type"])
}

func Benchmark_OpenAPI_summaryFromFuncName(b *testing.B) {
	b.ReportAllocs()

	var summary string
	for b.Loop() {
		summary = summaryFromFuncName("github.com/acme/api/handlers.(*Server).GetUserByID-fm")
	}
	require.Equal(b, "Get user by ID", summary)
}

func Benchmark_OpenAPI_coversRoute(b *testing.B) {
	b.ReportAllocs()

	var covered bool
	for b.Loop() {
		covered = coversRoute("/users/:id", "/users/:id/posts/:post", stringsEqual)
	}
	require.True(b, covered)
}
