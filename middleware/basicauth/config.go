package basicauth

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidSHA256PasswordLength = errors.New("decode SHA256 password: invalid length")

// fallbackDummySHA512 is SHA-512("fiber-basicauth-dummy"), used as a
// constant-time comparison target when no users are configured.
var fallbackDummySHA512 = [sha512.Size]byte{
	0x85, 0xc7, 0xd4, 0xbc, 0xec, 0x5f, 0xdf, 0xef, 0xe0, 0x4d, 0xd4, 0x3e, 0xd3, 0xac, 0x45, 0x7c,
	0x5e, 0x48, 0x60, 0x74, 0x12, 0x8e, 0xf8, 0xc0, 0xde, 0x39, 0x89, 0xf9, 0x84, 0x0c, 0x50, 0x24,
	0x1e, 0xa6, 0x1f, 0x2a, 0x11, 0x97, 0xb1, 0xb9, 0x67, 0xa9, 0xf7, 0x3b, 0x82, 0x8f, 0x95, 0xf5,
	0x58, 0xed, 0x3c, 0xab, 0x43, 0x22, 0xf6, 0xfa, 0x84, 0x1d, 0xbc, 0xeb, 0x87, 0xc4, 0x1c, 0x5a,
}

type passwordVerifier func(string) bool

type userVerifiers map[string]passwordVerifier

// Verifier strengths are ordered by expected verification work:
// bcrypt is strongest because it is adaptive and cost-based, SHA-512 follows
// as the larger fixed-cost digest, and SHA-256 is the lightest fixed-cost hash.
const (
	verifierStrengthSHA256 = iota + 1
	verifierStrengthSHA512
	verifierStrengthBcrypt
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

	// BadRequest defines the response body for malformed Authorization headers.
	// By default it will return with a 400 Bad Request without the WWW-Authenticate header.
	//
	// Optional. Default: nil
	BadRequest fiber.Handler

	// Realm is a string to define realm attribute of BasicAuth.
	// the realm identifies the system to authenticate against
	// and can be used by clients to save credentials
	//
	// Optional. Default: "Restricted".
	Realm string

	// Charset defines the value for the charset parameter in the
	// WWW-Authenticate header. According to RFC 7617 clients can use
	// this value to interpret credentials correctly. Only the value
	// "UTF-8" is allowed; any other value will panic.
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
	BadRequest:   nil,
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

	switch {
	case cfg.Charset == "":
		cfg.Charset = ConfigDefault.Charset
	case utils.EqualFold(cfg.Charset, "UTF-8"):
		cfg.Charset = "UTF-8"
	default:
		panic("basicauth: charset must be UTF-8")
	}

	if cfg.HeaderLimit <= 0 {
		cfg.HeaderLimit = ConfigDefault.HeaderLimit
	}

	if cfg.Authorizer == nil {
		verifiers, dummyVerify, err := buildVerifiers(cfg.Users)
		if err != nil {
			panic(err)
		}
		cfg.Authorizer = func(user, pass string, _ fiber.Ctx) bool {
			verify, ok := verifiers[user]
			if !ok {
				verify = dummyVerify
			}
			res := verify(pass)
			return ok && res
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

	if cfg.BadRequest == nil {
		cfg.BadRequest = func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusBadRequest)
		}
	}
	return cfg
}

type verifierStrength struct {
	algorithm int
	cost      int
}

// buildVerifiers parses each configured user hash, stores the verifier by user,
// and selects the strongest configured verifier for the dummy verification path.
// The dummy verifier is used for unknown-user requests to equalize timing.
//
// Note: in mixed-hash deployments (e.g. bcrypt + SHA-256), the dummy matches
// the strongest configured hash. Users with weaker hashes may still be
// distinguishable from unknown users by timing. This is an accepted trade-off
// since running all verifier types per request would be prohibitively expensive.
func buildVerifiers(users map[string]string) (userVerifiers, passwordVerifier, error) {
	verifiers := make(userVerifiers, len(users))
	dummyVerify := fallbackDummyVerify
	keys := make([]string, 0, len(users))
	for user := range users {
		keys = append(keys, user)
	}
	sort.Strings(keys)

	var dummyStrength verifierStrength
	for _, user := range keys {
		hashedPassword := users[user]
		verify, err := parseHashedPassword(hashedPassword)
		if err != nil {
			return nil, nil, err
		}
		verifiers[user] = verify

		strength := verifierStrengthForHash(hashedPassword)
		if strength.betterThan(dummyStrength) {
			dummyVerify = verify
			dummyStrength = strength
		}
	}

	return verifiers, dummyVerify, nil
}

// fallbackDummyVerify provides fixed verification work when no users are
// configured so missing-user requests still perform a constant-time hash check.
func fallbackDummyVerify(pass string) bool {
	sum := sha512.Sum512([]byte(pass))
	return subtle.ConstantTimeCompare(sum[:], fallbackDummySHA512[:]) == 1
}

// verifierStrengthForHash ranks a configured password hash by algorithm family
// and cost so the middleware can choose the strongest verifier for dummy work.
func verifierStrengthForHash(h string) verifierStrength {
	switch {
	case strings.HasPrefix(h, "$2"):
		cost, err := bcrypt.Cost([]byte(h))
		if err != nil {
			return verifierStrength{algorithm: verifierStrengthBcrypt}
		}
		return verifierStrength{algorithm: verifierStrengthBcrypt, cost: cost}
	case strings.HasPrefix(h, "{SHA512}"):
		return verifierStrength{algorithm: verifierStrengthSHA512}
	default:
		return verifierStrength{algorithm: verifierStrengthSHA256}
	}
}

// betterThan prefers stronger hash families first (bcrypt > SHA-512 > SHA-256)
// and uses the bcrypt cost as a tiebreaker within the same algorithm family.
func (s verifierStrength) betterThan(other verifierStrength) bool {
	if s.algorithm != other.algorithm {
		return s.algorithm > other.algorithm
	}

	return s.cost > other.cost
}

func parseHashedPassword(h string) (passwordVerifier, error) {
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
				return nil, ErrInvalidSHA256PasswordLength
			}
		}
		return func(p string) bool {
			sum := sha256.Sum256([]byte(p))
			return subtle.ConstantTimeCompare(sum[:], b) == 1
		}, nil
	}
}
