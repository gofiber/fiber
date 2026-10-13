package openapi

import (
	"github.com/gofiber/fiber/v3"
)

// routePolicy is every setting that changes what a route is documented as, apart from what the route
// itself says. Resolving a route reads nothing else.
type routePolicy struct {
	// equal compares route text the way the app's router does.
	equal segmentEqual
	// defaultProduces and defaultConsumes fill the media types a route leaves unset.
	defaultProduces string
	defaultConsumes string
	// handlerSummaries and groupTags allow a summary and a tag to be derived when a route sets none.
	handlerSummaries bool
	groupTags        bool
	// middlewareInference allows recognized middleware to shape the operation.
	middlewareInference bool
	// validates is set when a validator can reject what a route binds, making a 400 documentable.
	validates bool
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
	// security is the route's explicit requirement; nil leaves it to the middleware that covers the route.
	security *[]map[string][]string
	summary  string
	// respType is the media type of the response body.
	respType string
	// ownConsumes is the media type the route states itself; bodyType is the one used, after the default.
	ownConsumes string
	bodyType    string
	tags        []string
	declared    []fiber.RouteParameter
	// chain is the middleware that shapes the operation, without authentication when the route documents its own.
	chain middlewareSet
	// documentsInput is set when the route declares a body or parameters.
	documentsInput bool
	// badRequest is set when a 400 should be documented: a validator may reject what the route binds.
	badRequest bool
}

// resolveRoute fills unset route metadata from handler names, groups, app-wide media types and recognized
// middleware. It reads only its arguments; registering the security schemes, parameter models and CSRF
// header the facts imply is left to the caller.
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
		// Documented authentication suppresses inference of both the requirement and the 401.
		facts.chain &^= authMiddleware
		security := r.Security
		facts.security = &security
	}
	facts.badRequest = policy.validates && facts.documentsInput
	return facts
}
