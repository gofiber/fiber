package basicauth

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Users defines the allowed credentials
	//
	// Required. Default: map[string]string{}
	Users map[string]string

	// Authorizer defines a function you can pass
	// to check the credentials however you want.
	// It will be called with a username, password and
	// the current fiber context and is expected to return
	// true or false to indicate that the credentials were
	// approved or not.
	//
	// Optional. Default: nil.
	Authorizer func(string, string, fiber.Ctx) bool

	// Unauthorized defines the response body for unauthorized responses.
	// By default it will return with a 401 Unauthorized and the correct WWW-Auth header
	//
	// Optional. Default: nil
	Unauthorized fiber.Handler

	// Realm is a string to define realm attribute of BasicAuth.
	// the realm identifies the system to authenticate against
	// and can be used by clients to save credentials
	//
	// Optional. Default: "Restricted".
	Realm string

	// Charset defines the value for the charset parameter in the
	// WWW-Authenticate header. According to RFC 7617 clients can use
	// this value to interpret credentials correctly.
	//
	// Optional. Default: "UTF-8".
	Charset string

	// HeaderLimit specifies the maximum allowed length of the
	// Authorization header. Requests exceeding this limit will
	// be rejected.
	//
	// Optional. Default: 8192.
	HeaderLimit int
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	Users:        map[string]string{},
	Realm:        "Restricted",
	Charset:      "UTF-8",
	HeaderLimit:  8192,
	Authorizer:   nil,
	Unauthorized: nil,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Users == nil {
		cfg.Users = ConfigDefault.Users
	}
	if cfg.Realm == "" {
		cfg.Realm = ConfigDefault.Realm
	}
	if cfg.Charset == "" {
		cfg.Charset = ConfigDefault.Charset
	}
	if cfg.HeaderLimit <= 0 {
		cfg.HeaderLimit = ConfigDefault.HeaderLimit
	}
	if cfg.Authorizer == nil {
		verifiers := make(map[string]func(string) bool, len(cfg.Users))
		for u, hpw := range cfg.Users {
			v, err := parseHashedPassword(hpw)
			if err != nil {
				panic(err)
			}
			verifiers[u] = v
		}
		cfg.Authorizer = func(user, pass string, _ fiber.Ctx) bool {
			verify, ok := verifiers[user]
			return ok && verify(pass)
		}
	}
	if cfg.Unauthorized == nil {
		cfg.Unauthorized = func(c fiber.Ctx) error {
			header := "Basic realm=" + strconv.Quote(cfg.Realm)
			if cfg.Charset != "" {
				header += ", charset=" + strconv.Quote(cfg.Charset)
			}
			c.Set(fiber.HeaderWWWAuthenticate, header)
			c.Set(fiber.HeaderCacheControl, "no-store")
			c.Set(fiber.HeaderVary, fiber.HeaderAuthorization)
			return c.SendStatus(fiber.StatusUnauthorized)
		}
	}
	return cfg
}

func parseHashedPassword(h string) (func(string) bool, error) {
	switch {
	case strings.HasPrefix(h, "$2"):
		hash := []byte(h)
		return func(p string) bool {
			return bcrypt.CompareHashAndPassword(hash, []byte(p)) == nil
		}, nil
	case strings.HasPrefix(h, "{SHA512}"):
		b, err := base64.StdEncoding.DecodeString(h[len("{SHA512}"):])
		if err != nil {
			return nil, fmt.Errorf("decode SHA512 password: %w", err)
		}
		return func(p string) bool {
			sum := sha512.Sum512([]byte(p))
			return subtle.ConstantTimeCompare(sum[:], b) == 1
		}, nil
	case strings.HasPrefix(h, "{SHA256}"):
		b, err := base64.StdEncoding.DecodeString(h[len("{SHA256}"):])
		if err != nil {
			return nil, fmt.Errorf("decode SHA256 password: %w", err)
		}
		return func(p string) bool {
			sum := sha256.Sum256([]byte(p))
			return subtle.ConstantTimeCompare(sum[:], b) == 1
		}, nil
	default:
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != sha256.Size {
			if b, err = base64.StdEncoding.DecodeString(h); err != nil {
				return nil, fmt.Errorf("decode SHA256 password: %w", err)
			}
			if len(b) != sha256.Size {
				return nil, errors.New("decode SHA256 password: invalid length")
			}
		}
		return func(p string) bool {
			sum := sha256.Sum256([]byte(p))
			return subtle.ConstantTimeCompare(sum[:], b) == 1
		}, nil
	}
}
