package binder

import "errors"

var (
	ErrSuitableContentNotFound = errors.New("binder: suitable content not found to parse body")
)

var (
	mimeApplicationJSON = "application/json"
	mimeApplicationForm = "application/x-www-form-urlencoded"
	mimeMultipartForm   = "multipart/form-data"
	mimeTextXML         = "text/xml"
	mimeApplicationXML  = "application/xml"
)

var HeaderBinder = &headerBinding{}
var QueryBinder = &queryBinding{}
var FormBinder = &formBinding{}
var XMLBinder = &xmlBinding{}
var JSONBinder = &jsonBinding{}
