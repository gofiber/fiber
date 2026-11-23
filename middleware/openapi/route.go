package openapi

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// routePolicy is every setting that changes what a route is documented as, apart from the route itself.
type routePolicy struct {
	equal               segmentEqual
	custom              map[codeSite]middlewareKind // by where the constructor of each Config.Middleware handler was written
	infer               func(route *fiber.Route, name string) bool
	defaultProduces     string
	defaultConsumes     string
	names               []string // the names of the Config.Middleware entries, by kind from kindCustom
	defaults            []Defaults
	handlerSummaries    bool
	groupTags           bool
	middlewareInference bool
	validates           bool
}

// customKinds maps the constructor of each Config.Middleware handler to the kind it is recognized as.
func customKinds(docs []MiddlewareDoc) map[codeSite]middlewareKind {
	if len(docs) == 0 {
		return nil
	}
	kinds := make(map[codeSite]middlewareKind, len(docs))
	for i := range docs {
		if _, taken := kinds[siteOf(docs[i].Handler)]; !taken {
			kinds[siteOf(docs[i].Handler)] = kindCustom + middlewareKind(i)
		}
	}
	return kinds
}

// middlewareName is the name Config.InferMiddleware receives for a kind, or "" for one without a name.
func (p *routePolicy) middlewareName(kind middlewareKind) string {
	if int(kind) < len(middlewareNames) {
		return middlewareNames[kind]
	}
	if i := int(kind - kindCustom); i < len(p.names) {
		return p.names[i]
	}
	return ""
}

func newRoutePolicy(cfg *Config, env specEnv) routePolicy {
	names := make([]string, len(cfg.Middleware))
	for i := range cfg.Middleware {
		names[i] = cfg.Middleware[i].Name
	}
	return routePolicy{
		custom:              customKinds(cfg.Middleware),
		infer:               cfg.InferMiddleware,
		names:               names,
		equal:               env.equal,
		defaults:            cfg.Defaults,
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
	doc                *Operation // the route's own copy of its documentation, never nil
	security           *[]map[string][]string
	defaultResponses   map[string]RouteResponse // from Config.Defaults, for the statuses the route does not declare
	defaultParams      []RouteParameter
	defaultSecurity    *[]map[string][]string // from Config.Defaults, for a route that sets none and has no authentication inferred
	host               string                 // the effective host pattern of a domain route, or "" for any host
	summary            string
	respType           string
	ownConsumes        string
	bodyType           string
	tags               []string
	declared           []RouteParameter
	chain              middlewareSet
	documentsInput     bool
	parametersDeclared bool
	badRequest         bool
}

// resolveRoute fills unset route metadata from handler names, groups, app-wide media types and recognized middleware.
func resolveRoute(r *fiber.Route, covering []coveringMiddleware, policy *routePolicy) routeFacts {
	// A private copy: what the generator derives from it must not reach the Operation every route copy shares.
	doc := cloneOperation(r.Annotation(operationAnnotation{}))
	facts := routeFacts{
		doc:         doc,
		summary:     doc.Summary,
		tags:        doc.Tags,
		respType:    doc.Produces,
		ownConsumes: doc.Consumes,
		bodyType:    doc.Consumes,
		documentsInput: doc.RequestBody != nil || len(doc.ParameterModels) > 0 ||
			len(doc.Parameters) > 0,
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
	applyDefaults(&facts, r, policy)
	if policy.middlewareInference {
		facts.chain = middlewareOn(covering, r, policy)
	}
	if doc.Security != nil {
		facts.chain &^= authMiddleware
		security := doc.Security
		facts.security = &security
	}
	facts.badRequest = policy.validates && facts.documentsInput
	return facts
}

// applyDefaults fills the tags, parameters and responses Config.Defaults supplies for r; the first entry that
// sets something wins, and what the route declares itself always wins over every entry.
func applyDefaults(facts *routeFacts, r *fiber.Route, policy *routePolicy) {
	tagged := false
	for i := range policy.defaults {
		d := &policy.defaults[i]
		if !coversRoute(d.PathPrefix, r.Path, policy.equal) {
			continue
		}
		if !tagged && len(facts.doc.Tags) == 0 && len(d.Tags) > 0 {
			facts.tags, tagged = d.Tags, true
		}
		if facts.defaultSecurity == nil && d.Security != nil {
			security := d.Security
			facts.defaultSecurity = &security
		}
		facts.defaultParams = append(facts.defaultParams, d.Parameters...)
		for status, resp := range d.Responses {
			key := keyDefault
			if status != 0 {
				key = utils.FormatInt(int64(status))
			}
			if _, declared := facts.doc.Responses[key]; declared {
				continue
			}
			if _, set := facts.defaultResponses[key]; set {
				continue
			}
			if facts.defaultResponses == nil {
				facts.defaultResponses = make(map[string]RouteResponse)
			}
			facts.defaultResponses[key] = resp
		}
	}
}
