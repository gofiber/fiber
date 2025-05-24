package security

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// APIKeyCookie retrieves an API key from the named cookie.
// It returns ErrBadRequest if the cookie name is empty
// and ErrUnauthorized when the cookie does not exist.
func APIKeyCookie(c fiber.Ctx, name string) (string, error) {
	if name == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "name is empty")
	}
	key := c.Cookies(name)
	if key == "" {
		return "", fiber.ErrUnauthorized
	}
	return key, nil
}

// APIKeyHeader retrieves an API key from the named header.
// It returns ErrBadRequest if the header name is empty
// and ErrUnauthorized when the header is missing.
func APIKeyHeader(c fiber.Ctx, header string) (string, error) {
	if header == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "header is empty")
	}
	key := c.Get(header)
	if key == "" {
		return "", fiber.ErrUnauthorized
	}
	return key, nil
}

// APIKeyQuery retrieves an API key from the given query parameter.
// It returns ErrBadRequest if the query name is empty
// and ErrUnauthorized when the parameter is missing.
func APIKeyQuery(c fiber.Ctx, name string) (string, error) {
	if name == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "name is empty")
	}
	key := fiber.Query[string](c, name)
	if key == "" {
		return "", fiber.ErrUnauthorized
	}
	return key, nil
}

// HTTPAuthorizationCredentials represents the Authorization header parts.
type HTTPAuthorizationCredentials struct {
	Scheme string
	Token  string
}

// GetAuthorizationCredentials parses the Authorization header.
func GetAuthorizationCredentials(c fiber.Ctx) (HTTPAuthorizationCredentials, error) {
	auth := c.Get(fiber.HeaderAuthorization)
	if auth == "" {
		return HTTPAuthorizationCredentials{}, fiber.ErrUnauthorized
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return HTTPAuthorizationCredentials{}, fiber.ErrUnauthorized
	}
	return HTTPAuthorizationCredentials{Scheme: parts[0], Token: parts[1]}, nil
}

// HTTPBearer extracts a bearer token from the Authorization header.
func HTTPBearer(c fiber.Ctx) (string, error) {
	auth := c.Get(fiber.HeaderAuthorization)
	if auth == "" {
		return "", fiber.ErrUnauthorized
	}
	const prefix = "Bearer "
	if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return "", fiber.ErrUnauthorized
	}
	return auth[len(prefix):], nil
}

// HTTPBasicCredentials holds parsed HTTP basic auth credentials.
type HTTPBasicCredentials struct {
	Username string
	Password string
}

// HTTPBasic parses the Authorization header for basic auth credentials.
func HTTPBasic(c fiber.Ctx) (HTTPBasicCredentials, error) {
	auth := c.Get(fiber.HeaderAuthorization)
	if len(auth) <= 6 || !strings.EqualFold(auth[:6], "Basic ") {
		return HTTPBasicCredentials{}, fiber.ErrUnauthorized
	}
	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return HTTPBasicCredentials{}, fiber.ErrUnauthorized
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return HTTPBasicCredentials{}, fiber.ErrUnauthorized
	}
	return HTTPBasicCredentials{Username: parts[0], Password: parts[1]}, nil
}

// HTTPDigest retrieves the digest value from the Authorization header.
func HTTPDigest(c fiber.Ctx) (string, error) {
	auth := c.Get(fiber.HeaderAuthorization)
	const prefix = "Digest "
	if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return "", fiber.ErrUnauthorized
	}
	return auth[len(prefix):], nil
}
