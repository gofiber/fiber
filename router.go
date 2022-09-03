// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// Router defines all router handle interface includes app and group router.
type Router interface {
	Stack() [][]*Route

	Use(args ...interface{}) Router

	Get(path string, handlers ...Handler) Router
	Head(path string, handlers ...Handler) Router
	Post(path string, handlers ...Handler) Router
	Put(path string, handlers ...Handler) Router
	Delete(path string, handlers ...Handler) Router
	Connect(path string, handlers ...Handler) Router
	Options(path string, handlers ...Handler) Router
	Trace(path string, handlers ...Handler) Router
	Patch(path string, handlers ...Handler) Router

	Add(method, path string, handlers ...Handler) Router
	Static(prefix, root string, config ...Static) Router
	All(path string, handlers ...Handler) Router

	Name(name string) Router
}

// Route is a struct that holds all metadata for each registered handler
type Route struct {
	// Data for routing
	pos         uint32      // Position in stack -> important for the sort of the matched routes
	use         bool        // USE matches path prefixes
	star        bool        // Path equals '*'
	root        bool        // Path equals '/'
	path        string      // Prettified path
	routeParser routeParser // Parameter parser

	// Public fields
	Method   string    `json:"method"` // HTTP method
	Name     string    `json:"name"`   // Route's name
	Path     string    `json:"path"`   // Original registered route path
	Params   []string  `json:"params"` // Case sensitive param keys
	Handlers []Handler `json:"-"`      // Ctx handlers
}

type RouterConfig struct {
	CaseSensitive bool `json:"case_sensitive"`
	MergeParams   bool `json:"merge_params"`
	Strict        bool `json:"strict"`
}

var DefaultRouterConfig = RouterConfig{
	CaseSensitive: false,
	MergeParams:   false,
	Strict:        false,
}

func NewRouter(config ...RouterConfig) Router {
	cfg := DefaultRouterConfig

	if len(config) > 0 {
		if config[0].CaseSensitive {
			cfg.CaseSensitive = true
		}
		if config[0].MergeParams {
			cfg.MergeParams = true
		}
		if config[0].Strict {
			cfg.Strict = true
		}
	}

	//TODO : do config feature (not working now)
	return New(Config{
		CaseSensitive: cfg.CaseSensitive,
		StrictRouting: cfg.Strict,
	})
}
