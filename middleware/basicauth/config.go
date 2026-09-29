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
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/quotedstring"
	"github.com/gofiber/utils/v2"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidSHA256PasswordLength = errors.New("decode SHA256 password: invalid length")
	ErrInvalidSHA512PasswordLength = errors.New("decode SHA512 password: invalid length")
)

// fallbackDummySHA512 is SHA-512("fiber-basicauth-dummy"), used as a
// constant-time comparison target when no users are configured.
var fallbackDummySHA512 = [sha512.Size]byte{
	0x85, 0xc7, 0xd4, 0xbc, 0xec, 0x5f, 0xdf, 0xef, 0xe0, 0x4d, 0xd4, 0x3e, 0xd3, 0xac, 0x45, 0x7c,
	0x5e, 0x48, 0x60, 0x74, 0x12, 0x8e, 0xf8, 0xc0, 0xde, 0x39, 0x89, 0xf9, 0x84, 0x0c, 0x50, 0x24,
	0x1e, 0xa6, 0x1f, 0x2a, 0x11, 0x97, 0xb1, 0xb9, 0x67, 0xa9, 0xf7, 0x3b, 0x82, 0x8f, 0x95, 0xf5,
	0x58, 0xed, 0x3c, 0xab, 0x43, 0x22, 0xf6, 0xfa, 0x84, 0x1d, 0xbc, 0xeb, 0x87, 0xc4, 0x1c, 0x5a,
}

type passwordVerifier func(string) bool

// Hash algorithms recognized in Users, see parseHashedPassword.
const (
	hashAlgorithmSHA256 = iota + 1
	hashAlgorithmSHA512
	hashAlgorithmBcrypt
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
		verifiers, err := buildVerifiers(cfg.Users)
		if err != nil {
			panic(err)
		}
		cfg.Authorizer = func(user, pass string, _ fiber.Ctx) bool {
			return verifiers.verify(user, pass)
		}
	}

	if cfg.Unauthorized == nil {
		cfg.Unauthorized = func(c fiber.Ctx) error {
			header := `Basic realm="` + quotedstring.Escape(cfg.Realm) + `"`
			if cfg.Charset != "" {
				header += `, charset="` + quotedstring.Escape(cfg.Charset) + `"`
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

// hashClass groups password hashes that take the same work to verify: the
// same algorithm and, for bcrypt, the same cost. All SHA-256 encodings share
// one class.
type hashClass struct {
	algorithm int
	cost      int
}

// userVerifier holds a user's password verifier and the index of its hash
// class in credentialVerifier.dummies.
type userVerifier struct {
	verify passwordVerifier
	class  int
}

// credentialVerifier checks credentials against the configured Users.
type credentialVerifier struct {
	users map[string]userVerifier
	// dummies holds one verifier per hash class present in Users. They stand
	// in for the classes a request's user does not use, and for every class
	// when the user is unknown.
	dummies []passwordVerifier
}

// verify reports whether pass is the password of user. Every call runs
// exactly one verification per hash class: the user's own verifier for their
// class and that class's dummy for every other class, or only dummies when
// the user is unknown. The work therefore depends on the configuration alone,
// so response timing reveals neither whether the user exists nor which hash
// their password uses. Dummy results are discarded.
func (v *credentialVerifier) verify(user, pass string) bool {
	u, ok := v.users[user]
	matched := false
	for i, dummy := range v.dummies {
		if ok && i == u.class {
			matched = u.verify(pass)
			continue
		}
		dummy(pass)
	}
	return matched
}

// buildVerifiers parses each configured user hash and groups the hashes into
// classes of equal verification work. The first verifier seen for a class, in
// sorted username order, becomes that class's dummy. With no users configured,
// a fixed SHA-512 check is the only class, so unknown-user requests still do
// hashing work.
func buildVerifiers(users map[string]string) (*credentialVerifier, error) {
	keys := make([]string, 0, len(users))
	for user := range users {
		keys = append(keys, user)
	}
	sort.Strings(keys)

	v := &credentialVerifier{users: make(map[string]userVerifier, len(users))}
	classIndex := make(map[hashClass]int)
	for _, user := range keys {
		hashedPassword := users[user]
		verify, err := parseHashedPassword(hashedPassword)
		if err != nil {
			return nil, err
		}

		class := hashClassOf(hashedPassword)
		idx, seen := classIndex[class]
		if !seen {
			idx = len(v.dummies)
			classIndex[class] = idx
			v.dummies = append(v.dummies, verify)
		}
		v.users[user] = userVerifier{verify: verify, class: idx}
	}

	if len(v.dummies) == 0 {
		v.dummies = append(v.dummies, fallbackDummyVerify)
	}

	return v, nil
}

// fallbackDummyVerify provides fixed verification work when no users are
// configured so missing-user requests still perform a constant-time hash check.
func fallbackDummyVerify(pass string) bool {
	sum := sha512.Sum512([]byte(pass))
	return subtle.ConstantTimeCompare(sum[:], fallbackDummySHA512[:]) == 1
}

// hashClassOf returns the hash class of a configured password hash. It must
// follow the same prefix rules as parseHashedPassword. A bcrypt hash whose
// cost cannot be parsed fails verification before any hashing, so all such
// hashes share the zero-cost class.
func hashClassOf(h string) hashClass {
	switch {
	case strings.HasPrefix(h, "$2"):
		cost, err := bcrypt.Cost([]byte(h))
		if err != nil {
			return hashClass{algorithm: hashAlgorithmBcrypt}
		}
		return hashClass{algorithm: hashAlgorithmBcrypt, cost: cost}
	case strings.HasPrefix(h, "{SHA512}"):
		return hashClass{algorithm: hashAlgorithmSHA512}
	default:
		return hashClass{algorithm: hashAlgorithmSHA256}
	}
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
		// A digest of the wrong size can never equal a SHA-512 sum, so
		// accepting it would silently reject every password for this user.
		// Report it instead, which surfaces as a panic at startup.
		if len(b) != sha512.Size {
			return nil, ErrInvalidSHA512PasswordLength
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
		if len(b) != sha256.Size {
			return nil, ErrInvalidSHA256PasswordLength
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
