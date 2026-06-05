package routeguard

import (
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

type node struct {
	static   map[string]*node
	param    *node
	wildcard *node
	methods  map[string]bool
}

func newNode() *node {
	return &node{static: make(map[string]*node), methods: make(map[string]bool)}
}

type Router struct {
	mu   sync.RWMutex
	root *node
}

var matcher = &Router{root: newNode()}

func (r *Router) insert(method, path string) {
	cur := r.root
	path = strings.Trim(path, "/")
	if path == "" {
		cur.methods[method] = true
		return
	}
	for seg := range strings.SplitSeq(path, "/") {
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
	cur.methods[method] = true
}

func (r *Router) lookup(method, path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// strip leading slash once
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	// strip trailing slash
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return walk(r.root, path, 0, method)
}

func walk(n *node, path string, offset int, method string) bool {
	if n == nil {
		return false
	}
	if offset >= len(path) {
		return n.methods[method] ||
			(method == fiber.MethodHead && n.methods[fiber.MethodGet])
	}

	// find next segment boundary
	end := offset
	for end < len(path) && path[end] != '/' {
		end++
	}
	seg := path[offset:end] // substring, no allocation
	nextOffset := end + 1   // skip the '/'

	// 1. static child
	if child, ok := n.static[seg]; ok {
		if walk(child, path, nextOffset, method) {
			return true
		}
	}
	// 2. param child
	if walk(n.param, path, nextOffset, method) {
		return true
	}
	// 3. wildcard
	if n.wildcard != nil {
		return n.wildcard.methods[method] ||
			(method == fiber.MethodHead && n.wildcard.methods[fiber.MethodGet])
	}
	return false
}

func Build(app *fiber.App) {
	matcher.mu.Lock()
	defer matcher.mu.Unlock()
	matcher.root = newNode()
	for _, r := range app.GetRoutes(true) {
		matcher.insert(r.Method, r.Path)
	}
}

func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		if !matcher.lookup(c.Method(), c.Path()) {
			return cfg.ErrorHandler(c)
		}
		return c.Next()
	}
}
