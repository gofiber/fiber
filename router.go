package fiber

type Router interface {
	FindNextHandler(method string, path string) Handler
	GetAllRoutes() []any // TODO: specific routes ?
}
