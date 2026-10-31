package openapi

import (
	"github.com/gofiber/fiber/v3"
)

// routePolicy is every setting that changes what a route is documented as, apart from the route itself.
type routePolicy struct {
	equal               segmentEqual
	defaultProduces     string
	defaultConsumes     string
	handlerSummaries    bool
	groupTags           bool
	middlewareInference bool
	validates           bool
}

func newRoutePolicy(cfg *Config, env specEnv) routePolicy {
	return routePolicy{
		equal:               env.equal,
		defaultProduces:     cfg.DefaultProduces,
		defaultConsumes:     cfg.DefaultConsumes,
		handlerSummaries:    !cfg.DisableHandlerSummaries,
		groupTags:           !cfg.DisableGroupTags,
		middlewareInference: !cfg.DisableMiddlewareInference,
		validates:           env.validator != nil && !cfg.DisableValidationResponses,
	}
}

// routeFacts is a route's metadata with inferences filled in, before its path is expanded.
type routeFacts struct {
	security           *[]map[string][]string
	summary            string
	respType           string
	ownConsumes        string
	bodyType           string
	tags               []string
	declared           []fiber.RouteParameter
	chain              middlewareSet
	documentsInput     bool
	parametersDeclared bool
	badRequest         bool
}

// resolveRoute fills unset route metadata from handler names, groups, app-wide media types and recognized middleware.
func resolveRoute(r *fiber.Route, covering []coveringMiddleware, policy routePolicy) routeFacts {
	facts := routeFacts{
		summary:     r.Summary,
		tags:        r.Tags,
		respType:    r.Produces,
		ownConsumes: r.Consumes,
		bodyType:    r.Consumes,
		documentsInput: r.RequestBody != nil || len(r.ParameterModels) > 0 ||
			len(r.Parameters) > 0,
	}
	if facts.summary == "" && policy.handlerSummaries {
		facts.summary = handlerSummary(r.InnerHandlers())
	}
	if len(facts.tags) == 0 && policy.groupTags {
		if tag := groupTag(r); tag != "" {
			facts.tags = []string{tag}
		}
	}
	if facts.respType == "" {
		facts.respType = policy.defaultProduces
	}
	if facts.bodyType == "" {
		facts.bodyType = policy.defaultConsumes
	}
	if policy.middlewareInference {
		facts.chain = middlewareOn(covering, r, policy.equal)
	}
	if r.Security != nil {
		facts.chain &^= authMiddleware
		security := r.Security
		facts.security = &security
	}
	facts.badRequest = policy.validates && facts.documentsInput
	return facts
}
