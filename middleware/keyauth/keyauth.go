package keyauth

import (
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	utils "github.com/gofiber/utils/v2"
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// The keys for the values in context
const (
	tokenKey contextKey = iota
)

// ErrMissingOrMalformedAPIKey is returned when the API key is missing or invalid.
var ErrMissingOrMalformedAPIKey = errors.New("missing or invalid API Key")

const (
	challengeBufferDefaultCap = 128
	challengeBufferMaxCap     = 1024
)

var (
	challengeSlicePool = sync.Pool{
		New: func() any {
			s := make([]string, 0, 4)
			return &s
		},
	}

	challengeBufferPool = sync.Pool{
		New: func() any {
			buf := make([]byte, 0, challengeBufferDefaultCap)
			return &buf
		},
	}
)

func releaseChallengeBuffer(bufPtr *[]byte, used int) {
	if bufPtr == nil {
		return
	}

	buf := *bufPtr
	if used > len(buf) {
		used = len(buf)
	}
	for i := 0; i < used; i++ {
		buf[i] = 0
	}

	if cap(buf) > challengeBufferMaxCap {
		*bufPtr = make([]byte, 0, challengeBufferDefaultCap)
	} else {
		*bufPtr = buf[:0]
	}

	challengeBufferPool.Put(bufPtr)
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Init config
	cfg := configDefault(config...)

	// Determine the auth schemes from the extractor chain.
	authSchemes := getAuthSchemes(cfg.Extractor)

	// Return middleware handler
	return func(c fiber.Ctx) error {
		// Filter request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Extract and verify key
		key, err := cfg.Extractor.Extract(c)
		if errors.Is(err, extractors.ErrNotFound) {
			// Replace shared extractor not found error with a keyauth specific error
			err = ErrMissingOrMalformedAPIKey
		}
		// If there was no error extracting the key, validate it
		if err == nil {
			var valid bool
			valid, err = cfg.Validator(c, key)
			if err == nil && valid {
				c.Locals(tokenKey, key)
				return cfg.SuccessHandler(c)
			}
		}

		// Execute the error handler first
		handlerErr := cfg.ErrorHandler(c, err)

		status := c.Response().StatusCode()
		if status == fiber.StatusUnauthorized || status == fiber.StatusProxyAuthRequired {
			header := fiber.HeaderWWWAuthenticate
			if status == fiber.StatusProxyAuthRequired {
				header = fiber.HeaderProxyAuthenticate
			}
			if len(authSchemes) > 0 {
				challengesAny := challengeSlicePool.Get()
				challengesPtr, ok := challengesAny.(*[]string)
				if !ok {
					panic(errors.New("failed to type-assert to *[]string"))
				}
				challenges := (*challengesPtr)[:0]
				defer func() {
					for i := range challenges {
						challenges[i] = ""
					}
					*challengesPtr = challenges[:0]
					challengeSlicePool.Put(challengesPtr)
				}()

				for _, scheme := range authSchemes {
					bufPtr, ok := challengeBufferPool.Get().(*[]byte)
					if !ok {
						panic(errors.New("failed to type-assert to *[]byte"))
					}
					buf := (*bufPtr)[:0]

					buf = append(buf, scheme...)
					buf = append(buf, ' ')
					buf = append(buf, "realm="...)
					buf = strconv.AppendQuote(buf, cfg.Realm)

					if utils.EqualFold(scheme, "Bearer") && cfg.Error != "" {
						buf = append(buf, ", error="...)
						buf = strconv.AppendQuote(buf, cfg.Error)

						if cfg.ErrorDescription != "" {
							buf = append(buf, ", error_description="...)
							buf = strconv.AppendQuote(buf, cfg.ErrorDescription)
						}
						if cfg.ErrorURI != "" {
							buf = append(buf, ", error_uri="...)
							buf = strconv.AppendQuote(buf, cfg.ErrorURI)
						}
						if cfg.Error == ErrorInsufficientScope {
							buf = append(buf, ", scope="...)
							buf = strconv.AppendQuote(buf, cfg.Scope)
						}
					}

					challenge := string(buf)
					releaseChallengeBuffer(bufPtr, len(buf))
					challenges = append(challenges, challenge)
				}

				c.Set(header, strings.Join(challenges, ", "))
			} else if cfg.Challenge != "" {
				c.Set(header, cfg.Challenge)
			}
		}

		return handlerErr
	}
}

// TokenFromContext returns the bearer token from the request context.
// returns an empty string if the token does not exist
func TokenFromContext(c fiber.Ctx) string {
	token, ok := c.Locals(tokenKey).(string)
	if !ok {
		return ""
	}
	return token
}

// getAuthSchemes inspects an extractor and its chain to find all auth schemes
// used by FromAuthHeader. It returns a slice of schemes, or an empty slice if
// none are found.
func getAuthSchemes(e extractors.Extractor) []string {
	var schemes []string
	if e.Source == extractors.SourceAuthHeader && e.AuthScheme != "" {
		schemes = append(schemes, e.AuthScheme)
	}
	for _, ex := range e.Chain {
		schemes = append(schemes, getAuthSchemes(ex)...)
	}
	return schemes
}
