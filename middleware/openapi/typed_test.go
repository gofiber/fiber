package openapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

type userRequest struct {
	Q     string `query:"q"`
	Token string `header:"X-Token"`
	Name  string `json:"name" validate:"required"`
	ID    int    `uri:"id"`
}

type userResponse struct {
	Echo string `json:"echo"`
}

type searchRequest struct {
	Q string `query:"q"`
}

func Test_OpenAPI_TypedRegistersAndDocuments(t *testing.T) {
	t.Parallel()
	var order []string
	app := fiber.New()
	Typed(app, fiber.MethodPost, "/users/:id", func(c fiber.Ctx, req *userRequest) (*userResponse, error) {
		order = append(order, "handler")
		return &userResponse{Echo: strings.Join([]string{c.Params("id"), req.Q, req.Token, req.Name}, "|")}, nil
	}, WithStatus(fiber.StatusCreated), WithMiddleware(func(c fiber.Ctx) error {
		order = append(order, "middleware")
		return c.Next()
	})).Summary("Create a user")
	Typed(app, fiber.MethodGet, "/search", func(_ fiber.Ctx, req *searchRequest) (*Empty, error) {
		if req.Q == "boom" {
			return nil, errors.New("boom")
		}
		return &Empty{}, nil
	})
	Typed(app, fiber.MethodGet, "/nothing", func(_ fiber.Ctx, _ *searchRequest) (*userResponse, error) {
		return nil, nil //nolint:nilnil // a nil response is the case under test
	})

	req := httptest.NewRequest(fiber.MethodPost, "/users/7?q=x", strings.NewReader(`{"name":"ann"}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Token", "tok")
	resp := apptest.Expect(t, app, req, fiber.StatusCreated)
	require.JSONEq(t, `{"echo":"7|x|tok|ann"}`, apptest.Body(t, resp))
	require.Equal(t, []string{"middleware", "handler"}, order)

	apptest.Expect(t, app, "/search?q=ok", fiber.StatusNoContent)
	apptest.Expect(t, app, "/search?q=boom", fiber.StatusInternalServerError)
	apptest.Expect(t, app, "/nothing", fiber.StatusOK)

	spec := modelSpec(t, app)
	post := modelOperation(t, spec, "/users/{id}", "post")
	require.Equal(t, "Create a user", post["summary"])
	params := chainParams(t, post)
	require.Contains(t, params, "path:id")
	require.Contains(t, params, "query:q")
	require.Contains(t, params, "header:X-Token")

	body := requireMap(t, requireMap(t, requireMap(t, post["requestBody"])["content"])[fiber.MIMEApplicationJSON])
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/userRequest_request"}, body["schema"])
	bodySchema := requireMap(t, modelSchemas(t, spec)["userRequest_request"])
	props := requireMap(t, bodySchema["properties"])
	require.Contains(t, props, "name")
	require.NotContains(t, props, "ID", "a parameter is not part of the body")
	require.NotContains(t, props, "Q")
	require.NotContains(t, props, "Token")
	require.Equal(t, []any{"name"}, bodySchema["required"])

	created := requireMap(t, chainResponses(t, post)["201"])
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/userResponse"},
		requireMap(t, requireMap(t, created["content"])[fiber.MIMEApplicationJSON])["schema"])

	search := modelOperation(t, spec, "/search", "get")
	require.Contains(t, chainParams(t, search), "query:q")
	require.NotContains(t, search, "requestBody", "a request with no body fields has no body")
	require.Contains(t, chainResponses(t, search), "204")
	require.NotContains(t, requireMap(t, chainResponses(t, search)["204"]), "content")
}

type plainBody struct {
	Name string `json:"name"`
}

func Test_OpenAPI_RequestBodiesLeaveOutParameters(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Post("/users/:id", listUsers).Accepts(userRequest{}).Returns(fiber.StatusOK, userRequest{})
	app.Post("/plain", listUsers).Accepts(plainBody{}).Returns(fiber.StatusOK, plainBody{})
	spec := modelSpec(t, app)
	schemas := modelSchemas(t, spec)

	require.Contains(t, requireMap(t, requireMap(t, schemas["userRequest"])["properties"]), "ID", "a response keeps every field")
	require.NotContains(t, requireMap(t, requireMap(t, schemas["userRequest_request"])["properties"]), "ID")
	require.Contains(t, schemas, "plainBody")
	require.NotContains(t, schemas, "plainBody_request", "a struct without parameter fields is described once")
}

func Test_OpenAPI_TypedDocumentsAnEmptyRouteWithoutABody(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Typed(app, fiber.MethodDelete, "/things/:id", func(_ fiber.Ctx, _ *struct {
		ID int `uri:"id"`
	},
	) (*Empty, error) {
		return &Empty{}, nil
	})
	spec := modelSpec(t, app)
	op := modelOperation(t, spec, "/things/{id}", "delete")
	require.NotContains(t, op, "requestBody")
	require.Contains(t, chainResponses(t, op), "204")
	require.Contains(t, chainParams(t, op), "path:id")
	apptest.Expect(t, app, httptest.NewRequest(fiber.MethodDelete, "/things/3", http.NoBody), fiber.StatusNoContent)
	_, err := json.Marshal(spec)
	require.NoError(t, err)
}
