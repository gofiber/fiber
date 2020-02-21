package fiber

import "strings"

// Group ...
type Group struct {
	prefix string
	app    *App
}

// Group ...
func (app *App) Group(prefix string, args ...interface{}) *Group {
	if len(args) > 0 {
		app.register("USE", prefix, args...)
	}
	return &Group{
		prefix: prefix,
		app:    app,
	}
}

// Group ...
func (grp *Group) Group(newPrfx string, args ...interface{}) *Group {
	var prefix = grp.prefix
	if len(newPrfx) > 0 && newPrfx[0] != '/' && newPrfx[0] != '*' {
		newPrfx = "/" + newPrfx
	}
	// When grouping, always remove single slash
	if len(prefix) > 0 && newPrfx == "/" {
		newPrfx = ""
	}
	// Prepent group prefix if exist
	prefix = prefix + newPrfx
	// Clean path by removing double "//" => "/"
	prefix = strings.Replace(prefix, "//", "/", -1)
	if len(args) > 0 {
		grp.app.register("USE", prefix, args...)
	}
	return &Group{
		prefix: prefix,
		app:    grp.app,
	}
}

// Static ...
func (grp *Group) Static(args ...string) *Group {
	grp.app.registerStatic(grp.prefix, args...)
	return grp
}

// WebSocket ...
func (grp *Group) WebSocket(args ...interface{}) *Group {
	grp.app.register("GET", grp.prefix, args...)
	return grp
}

// Connect ...
func (grp *Group) Connect(args ...interface{}) *Group {
	grp.app.register("CONNECT", grp.prefix, args...)
	return grp
}

// Put ...
func (grp *Group) Put(args ...interface{}) *Group {
	grp.app.register("PUT", grp.prefix, args...)
	return grp
}

// Post ...
func (grp *Group) Post(args ...interface{}) *Group {
	grp.app.register("POST", grp.prefix, args...)
	return grp
}

// Delete ...
func (grp *Group) Delete(args ...interface{}) *Group {
	grp.app.register("DELETE", grp.prefix, args...)
	return grp
}

// Head ...
func (grp *Group) Head(args ...interface{}) *Group {
	grp.app.register("HEAD", grp.prefix, args...)
	return grp
}

// Patch ...
func (grp *Group) Patch(args ...interface{}) *Group {
	grp.app.register("PATCH", grp.prefix, args...)
	return grp
}

// Options ...
func (grp *Group) Options(args ...interface{}) *Group {
	grp.app.register("OPTIONS", grp.prefix, args...)
	return grp
}

// Trace ...
func (grp *Group) Trace(args ...interface{}) *Group {
	grp.app.register("TRACE", grp.prefix, args...)
	return grp
}

// Get ...
func (grp *Group) Get(args ...interface{}) *Group {
	grp.app.register("GET", grp.prefix, args...)
	return grp
}

// All ...
func (grp *Group) All(args ...interface{}) *Group {
	grp.app.register("ALL", grp.prefix, args...)
	return grp
}

// Use ...
func (grp *Group) Use(args ...interface{}) *Group {
	grp.app.register("USE", grp.prefix, args...)
	return grp
}
