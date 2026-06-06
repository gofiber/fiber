package routeguard

import (
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

const stateKey = "routeguard:router"

type node struct {
	static        map[string]*node
	param         *node
	wildcard      *node
	methods       map[string]bool
	trailMethods  map[string]bool // methods for paths with trailing slash
}

func newNode() *node {
	return &node{
		static:       make(map[string]*node),
		methods:      make(map[string]bool),
		trailMethods: make(map[string]bool),
	}
}

type Router struct {
	mu            sync.RWMutex
	root          *node
	caseSensitive bool
	strictRouting bool
}

func (r *Router) insert(method, path string) {
	cur := r.root
	hasTrailingSlash := len(path) > 1 && path[len(path)-1] == '/'
	path = strings.Trim(path, "/")

	if path == "" {
		if r.strictRouting && hasTrailingSlash {
			cur.trailMethods[method] = true
		} else {
			cur.methods[method] = true
		}
		return
	}

	for seg := range strings.SplitSeq(path, "/") {
		if !r.caseSensitive {
			seg = strings.ToLower(seg)
		}
		switch {
		case seg == "*" || strings.HasPrefix(seg, "+"):
			if cur.wildcard == nil {
				cur.wildcard = newNode()
			}
			cur = cur.wildcard
			cur.methods[method] = true
			return
		case strings.HasPrefix(seg, ":"):
			if cur.param == nil {
				cur.param = newNode()
			}
			cur = cur.param
		default:
			child, ok := cur.static[seg]
			if !ok {
				child = newNode()
				cur.static[seg] = child
			}
			cur = child
		}
	}

	if r.strictRouting && hasTrailingSlash {
		cur.trailMethods[method] = true
	} else {
		cur.methods[method] = true
	}
}

func (r *Router) lookup(method, path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hasTrailingSlash := len(path) > 1 && path[len(path)-1] == '/'
	path = strings.Trim(path, "/")

	return r.walk(r.root, path, 0, method, hasTrailingSlash)
}

func (r *Router) walk(n *node, path string, offset int, method string, hasTrailingSlash bool) bool {
	if n == nil {
		return false
	}

	if offset >= len(path) {
		if r.strictRouting && hasTrailingSlash {
			return n.trailMethods[method] ||
				(method == fiber.MethodHead && n.trailMethods[fiber.MethodGet])
		}
		return n.methods[method] ||
			(method == fiber.MethodHead && n.methods[fiber.MethodGet])
	}

	end := offset
	for end < len(path) && path[end] != '/' {
		end++
	}
	seg := path[offset:end]
	if !r.caseSensitive {
		seg = strings.ToLower(seg)
	}
	nextOffset := end + 1

	if child, ok := n.static[seg]; ok {
		if r.walk(child, path, nextOffset, method, hasTrailingSlash) {
			return true
		}
	}
	if r.walk(n.param, path, nextOffset, method, hasTrailingSlash) {
		return true
	}
	if n.wildcard != nil {
		return n.wildcard.methods[method] ||
			(method == fiber.MethodHead && n.wildcard.methods[fiber.MethodGet])
	}
	return false
}

func Build(app *fiber.App) {
	cfg := app.Config()
	router := &Router{
		root:          newNode(),
		caseSensitive: cfg.CaseSensitive,
		strictRouting: cfg.StrictRouting,
	}
	for _, r := range app.GetRoutes(true) {
		router.insert(r.Method, r.Path)
	}
	app.State().Set(stateKey, router)
}

func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		router, ok := c.App().State().Get(stateKey)
		if !ok {
			return c.Next()
		}

		r := router.(*Router)
		if !r.lookup(c.Method(), c.Path()) {
			return cfg.ErrorHandler(c)
		}
		return c.Next()
	}
}
