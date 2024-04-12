package cors

import (
	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// AllowOriginsFunc defines a function that will set the 'Access-Control-Allow-Origin'
	// response header to the 'origin' request header when returned true. This allows for
	// dynamic evaluation of allowed origins. Note if AllowCredentials is true, wildcard origins
	// will be not have the 'Access-Control-Allow-Credentials' header set to 'true'.
	//
	// Optional. Default: nil
	AllowOriginsFunc func(origin string) bool

	// AllowOrigin defines a list of origins that may access the resource.
	//
	// This supports subdomains wildcarding by prefixing the domain with a `*.`
	// e.g. "http://.domain.com". This will allow all level of subdomains of domain.com to access the resource.
	//
	// If the special wildcard `"*"` is present in the list, all origins will be allowed.
	//
	// Optional. Default value []string{}
	AllowOrigins []string

	// AllowMethods defines a list methods allowed when accessing the resource.
	// This is used in response to a preflight request.
	//
	// Optional. Default value []string{"GET", "POST", "HEAD", "PUT", "DELETE", "PATCH"}
	AllowMethods []string

	// AllowHeaders defines a list of request headers that can be used when
	// making the actual request. This is in response to a preflight request.
	//
	// Optional. Default value []string{}
	AllowHeaders []string

	// AllowCredentials indicates whether or not the response to the request
	// can be exposed when the credentials flag is true. When used as part of
	// a response to a preflight request, this indicates whether or not the
	// actual request can be made using credentials. Note: If true, AllowOrigins
	// cannot be set to true to prevent security vulnerabilities.
	//
	// Optional. Default value false.
	AllowCredentials bool

	// ExposeHeaders defines a whitelist headers that clients are allowed to
	// access.
	//
	// Optional. Default value []string{}.
	ExposeHeaders []string

	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached.
	// If you pass MaxAge 0, Access-Control-Max-Age header will not be added and
	// browser will use 5 seconds by default.
	// To disable caching completely, pass MaxAge value negative. It will set the Access-Control-Max-Age header 0.
	//
	// Optional. Default value 0.
	MaxAge int

	// AllowPrivateNetwork indicates whether the Access-Control-Allow-Private-Network
	// response header should be set to true, allowing requests from private networks.
	//
	// Optional. Default value false.
	AllowPrivateNetwork bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:             nil,
	AllowOriginsFunc: nil,
	AllowOrigins:     []string{"*"},
	AllowMethods: []string{
		fiber.MethodGet,
		fiber.MethodPost,
		fiber.MethodHead,
		fiber.MethodPut,
		fiber.MethodDelete,
		fiber.MethodPatch,
	},
	AllowHeaders:        []string{},
	AllowCredentials:    false,
	ExposeHeaders:       []string{},
	MaxAge:              0,
	AllowPrivateNetwork: false,
}
