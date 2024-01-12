package fiber

type Router interface {
	FindNextHandler(method string, path string) Handler
	GetAllRoutes() []any // TODO: specific routes ?
}

type Route struct {
	// Public fields
	Method string `json:"method"` // HTTP method
	Name   string `json:"name"`   // Route's name
	//nolint:revive // Having both a Path (uppercase) and a path (lowercase) is fine
	Path     string    `json:"path"`   // Original registered route path
	Params   []string  `json:"params"` // Case sensitive param keys
	Handlers []Handler `json:"-"`      // Ctx handlers
}

// TODO: add Route getters

type IGroup interface {
	GetPrefix() string
}

// Group struct
type Group struct {
	Prefix string
	IGroup
}
