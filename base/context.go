// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package base

import (
	"mime/multipart"
	"time"
)

// Map is a shortcut for map[string]interface{}, usefull for JSON returns
type Map map[string]interface{}

// Range data for ctx.Range
type Range struct {
	Type   string
	Ranges []struct {
		Start int
		End   int
	}
}

// Cookie data for ctx.Cookie
type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	Expires  time.Time
	Secure   bool
	HTTPOnly bool
	SameSite string
}

type INihility interface {
	// Route returns the matched Route struct.
	// Route() *Route

	// Status sets the HTTP status for the response.
	// This method is chainable.
	// Status(status int) *Ctx

	// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
	// Type(ext string) *Ctx
}

// Ctx represents the Context which hold the HTTP request and response.
// It has methods for the request query string, parameters, body, HTTP headers and so on.
type IContext interface {
	IBaseRequest
	IBaseResponse

	// Accepts checks if the specified extensions or content types are acceptable.
	Accepts(offers ...string) string

	// AcceptsCharsets checks if the specified charset is acceptable.
	AcceptsCharsets(offers ...string) string

	// AcceptsEncodings checks if the specified encoding is acceptable.
	AcceptsEncodings(offers ...string) string

	// AcceptsLanguages checks if the specified language is acceptable.
	AcceptsLanguages(offers ...string) string

	// Append the specified value to the HTTP response header field.
	// If the header is not already set, it creates the header with the specified value.
	Append(field string, values ...string)

	// Attachment sets the HTTP response Content-Disposition header field to attachment.
	Attachment(filename ...string)

	// ClearCookie expires a specific cookie by key on the client side.
	// If no key is provided it expires all cookies that came with the request.
	ClearCookie(key ...string)

	// Cookie sets a cookie by passing a cookie struct
	Cookie(cookie *Cookie)

	// Cookies is used for getting a cookie value by key
	Cookies(key string) (value string)

	// Download transfers the file from path as an attachment.
	// Typically, browsers will prompt the user for download.
	// By default, the Content-Disposition header filename= parameter is the filepath (this typically appears in the browser dialog).
	// Override this default with the filename parameter.
	Download(file string, filename ...string)

	// Error contains the error information passed via the Next(err) method.
	Error() error

	// Format performs content-negotiation on the Accept HTTP header.
	// It uses Accepts to select a proper format.
	// If the header is not specified or there is no proper format, text/plain is used.
	Format(body interface{})

	// FormFile returns the first file by key from a MultipartForm.
	FormFile(key string) (*multipart.FileHeader, error)

	// FormValue returns the first value by key from a MultipartForm.
	FormValue(key string) (value string)

	// Fresh When the response is still “fresh” in the client’s cache true is returned,
	// otherwise false is returned to indicate that the client cache is now stale
	// and the full response should be sent.
	// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
	// reload request, this module will return false to make handling these requests transparent.
	// https://github.com/jshttp/fresh/blob/10e0471669dbbfbfd8de65bc6efac2ddd0bfa057/index.js#L33
	Fresh() bool

	// Get returns the HTTP request header specified by field.
	// Field names are case-insensitive
	Get(key string) (value string)

	// Hostname contains the hostname derived from the Host HTTP header.
	Hostname() string

	// IP returns the remote IP address of the request.
	IP() string

	// IPs returns an string slice of IP addresses specified in the X-Forwarded-For request header.
	IPs() []string

	// Is returns the matching content type,
	// if the incoming request’s Content-Type HTTP header field matches the MIME type specified by the type parameter
	Is(extension string) (match bool)

	// Links joins the links followed by the property to populate the response’s Link HTTP header field.
	Links(link ...string)

	// Locals makes it possible to pass interface{} values under string keys scoped to the request
	// and therefore available to all following routes that match the request.
	Locals(key string, value ...interface{}) (val interface{})

	// Location sets the response Location HTTP header to the specified path parameter.
	Location(path string)

	// Method contains a string corresponding to the HTTP method of the request: GET, POST, PUT and so on.
	Method(override ...string) string

	// MultipartForm parse form entries from binary.
	// This returns a map[string][]string, so given a key the value will be a string slice.
	MultipartForm() (*multipart.Form, error)

	// Next executes the next method in the stack that matches the current route.
	// You can pass an optional error for custom error handling.
	Next(err ...error)

	// OriginalURL contains the original request URL.
	OriginalURL() string

	// Params is used to get the route parameters.
	// Defaults to empty string "", if the param doesn't exist.
	Params(key string) string

	// Path returns the path part of the request URL.
	// Optionally, you could override the path.
	Path(override ...string) string

	// Protocol contains the request protocol string: http or https for TLS requests.
	Protocol() string

	// Query returns the query string parameter in the url.
	Query(key string) (value string)

	// Range returns a struct containing the type and a slice of ranges.
	Range(size int) (rangeData Range, err error)

	// Redirect to the URL derived from the specified path, with specified status.
	// If status is not specified, status defaults to 302 Found
	Redirect(location string, status ...int)

	// Render a template with data and sends a text/html response.
	// We support the following engines: html, amber, handlebars, mustache, pug
	Render(file string, bind interface{}) error

	// SaveFile saves any multipart file to disk.
	SaveFile(fileheader *multipart.FileHeader, path string) error

	// Secure returns a boolean property, that is true, if a TLS connection is established.
	Secure() bool

	// SendBytes sets the HTTP response body for []byte types
	// This means no type assertion, recommended for faster performance
	SendBytes(body []byte)

	// SendFile transfers the file from the given path.
	// The file is compressed by default, disable this by passing a 'true' argument
	// Sets the Content-Type response HTTP header field based on the filenames extension.
	SendFile(file string, uncompressed ...bool)

	// SendStatus sets the HTTP status code and if the response body is empty,
	// it sets the correct status message in the body.
	SendStatus(status int)

	// SendString sets the HTTP response body for string types
	// This means no type assertion, recommended for faster performance
	SendString(body string)

	// Set sets the response’s HTTP header field to the specified key, value.
	Set(key, val string)

	// Subdomains returns a string slice of subdomains in the domain name of the request.
	// The subdomain offset, which defaults to 2, is used for determining the beginning of the subdomain segments.
	Subdomains(offset ...int) []string

	// Stale is not implemented yet, pull requests are welcome!
	Stale() bool

	// Vary adds the given header field to the Vary response header.
	// This will append the header, if not already listed, otherwise leaves it listed in the current location.
	Vary(fields ...string)

	// XHR returns a Boolean property, that is true, if the request’s X-Requested-With header field is XMLHttpRequest,
	// indicating that the request was issued by a client library (such as jQuery).
	XHR() bool
}
