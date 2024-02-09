package binder

import (
	"errors"
)

// Binder errors
var (
	ErrSuitableContentNotFound = errors.New("binder: suitable content not found to parse body")
	ErrMapNotConvertable       = errors.New("binder: map is not convertable to map[string]string or map[string][]string")
)

// Init default binders for Fiber
var (
	HeaderBinder     = &headerBinding{}
	RespHeaderBinder = &respHeaderBinding{}
	CookieBinder     = &cookieBinding{}
	QueryBinder      = &queryBinding{}
	FormBinder       = &formBinding{}
	URIBinder        = &uriBinding{}
	XMLBinder        = &xmlBinding{}
	JSONBinder       = &jsonBinding{}
)
