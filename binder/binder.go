package binder

import "errors"

var (
	ErrSuitableContentNotFound = errors.New("binder: suitable content not found to parse body")
)

var HeaderBinder = &headerBinding{}
var RespHeaderBinder = &respHeaderBinding{}
var CookieBinder = &cookieBinding{}
var QueryBinder = &queryBinding{}
var FormBinder = &formBinding{}
var URIBinder = &uriBinding{}
var XMLBinder = &xmlBinding{}
var JSONBinder = &jsonBinding{}
