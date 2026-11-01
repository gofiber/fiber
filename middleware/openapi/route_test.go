package openapi

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func fullPolicy() routePolicy {
	return routePolicy{
		equal:               func(a, b string) bool { return a == b },
		defaultProduces:     fiber.MIMEApplicationJSON,
		defaultConsumes:     fiber.MIMEApplicationJSON,
		handlerSummaries:    true,
		groupTags:           true,
		middlewareInference: true,
		validates:           true,
	}
}

func Test_resolveRoute_Summary(t *testing.T) {
	t.Parallel()

	route := &fiber.Route{Method: fiber.MethodGet, Path: "/users", Handlers: []fiber.Handler{listUsers}}

	require.Equal(t, "List users", resolveRoute(route, nil, fullPolicy()).summary)

	policy := fullPolicy()
	policy.handlerSummaries = false
	require.Empty(t, resolveRoute(route, nil, policy).summary)

	route.Summary = "Explicit"
	require.Equal(t, "Explicit", resolveRoute(route, nil, fullPolicy()).summary)
}

func Test_resolveRoute_MediaTypes(t *testing.T) {
	t.Parallel()

	route := &fiber.Route{Method: fiber.MethodPost, Path: "/x"}
	facts := resolveRoute(route, nil, fullPolicy())
	require.Equal(t, fiber.MIMEApplicationJSON, facts.respType)
	require.Equal(t, fiber.MIMEApplicationJSON, facts.bodyType)
	require.Empty(t, facts.ownConsumes, "a default is not a statement by the route")

	route.Produces = fiber.MIMETextPlain
	route.Consumes = fiber.MIMEApplicationForm
	facts = resolveRoute(route, nil, fullPolicy())
	require.Equal(t, fiber.MIMETextPlain, facts.respType)
	require.Equal(t, fiber.MIMEApplicationForm, facts.bodyType)
	require.Equal(t, fiber.MIMEApplicationForm, facts.ownConsumes)
}

func Test_resolveRoute_BadRequest(t *testing.T) {
	t.Parallel()

	declares := &fiber.Route{Method: fiber.MethodPost, Path: "/x", Parameters: []fiber.RouteParameter{{Name: "q", In: fiber.ParamInQuery}}}
	bare := &fiber.Route{Method: fiber.MethodGet, Path: "/x"}

	require.True(t, resolveRoute(declares, nil, fullPolicy()).badRequest)
	require.False(t, resolveRoute(bare, nil, fullPolicy()).badRequest, "nothing to reject")

	policy := fullPolicy()
	policy.validates = false
	require.False(t, resolveRoute(declares, nil, policy).badRequest, "no validator")
}

func Test_resolveRoute_Security(t *testing.T) {
	t.Parallel()

	covering := []coveringMiddleware{{prefix: "/", set: 1 << kindBasicAuth}}
	route := &fiber.Route{Method: fiber.MethodGet, Path: "/x"}

	facts := resolveRoute(route, covering, fullPolicy())
	require.True(t, facts.chain.has(kindBasicAuth))
	require.Nil(t, facts.security, "the requirement is registered later")

	route.Security = []map[string][]string{}
	facts = resolveRoute(route, covering, fullPolicy())
	require.False(t, facts.chain.has(kindBasicAuth), "documented security suppresses inference")
	require.NotNil(t, facts.security)
	require.Empty(t, *facts.security)

	policy := fullPolicy()
	policy.middlewareInference = false
	route.Security = nil
	require.Zero(t, resolveRoute(route, covering, policy).chain)
}
