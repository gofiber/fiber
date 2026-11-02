package csrf

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/session"
)

type sessionManager struct {
	session *session.Store
}

type sessionKeyType int

const (
	sessionKey sessionKeyType = 0
)

func newSessionManager(s *session.Store) *sessionManager {
	// Create new storage handler
	sessionManager := new(sessionManager)
	if s != nil {
		// Use provided storage if provided
		sessionManager.session = s

		// Register the sessionKeyType and Token type
		s.RegisterType(sessionKeyType(0))
		s.RegisterType(Token{})
	}
	return sessionManager
}

// middleware returns the session middleware the token is kept in: the one of the store csrf
// was given and, when that store has none in the request, the innermost one, which is what
// csrf used before it could tell them apart. It is nil when no session middleware runs.
func (m *sessionManager) middleware(c fiber.Ctx) *session.Middleware {
	if sess := m.session.FromContext(c); sess != nil {
		return sess
	}
	return session.FromContext(c)
}

// get token from session
func (m *sessionManager) getRaw(c fiber.Ctx, key string, raw []byte) []byte {
	sess := m.middleware(c)
	var token Token
	var ok bool

	if sess != nil {
		token, ok = sess.Get(sessionKey).(Token)
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return nil
		}
		// token.Raw stays valid after Release only because session Reset() does
		// not scrub values; copy it before Release if that ever changes.
		defer storeSess.Release()
		token, ok = storeSess.Get(sessionKey).(Token)
	}

	if ok {
		if token.Expiration.Before(time.Now()) || key != token.Key || !compareTokens(raw, token.Raw) {
			return nil
		}
		return token.Raw
	}

	return nil
}

// setRaw stores the token in the session, or renews it, and returns when it expires.
func (m *sessionManager) setRaw(c fiber.Ctx, key string, raw []byte, exp time.Duration) time.Time {
	expires := time.Now().Add(exp)
	sess := m.middleware(c)
	if sess != nil {
		// A token still stored with more than half its lifetime left is left as it is: a Set
		// would make this request a writer of the session, whose save then stands in for a
		// change another request stored while it ran, such as a role taken away. The cookie
		// expires with the token, so the browser does not keep sending one that is gone.
		if token, ok := sess.Get(sessionKey).(Token); ok && token.Key == key && compareTokens(raw, token.Raw) &&
			time.Until(token.Expiration) > exp/2 {
			return token.Expiration
		}
		// the key is crucial in crsf and sometimes a reference to another value which can be reused later(pool/unsafe values concept), so a copy is made here
		sess.Set(sessionKey, Token{Key: key, Raw: raw, Expiration: expires})
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return expires
		}
		defer storeSess.Release()
		storeSess.Set(sessionKey, Token{Key: key, Raw: raw, Expiration: expires})
		if err := storeSess.Save(); err != nil {
			log.Warn("csrf: failed to save session: ", err)
		}
	}
	return expires
}

// delete token from session
func (m *sessionManager) delRaw(c fiber.Ctx) {
	sess := m.middleware(c)
	if sess != nil {
		sess.Delete(sessionKey)
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return
		}
		defer storeSess.Release()
		storeSess.Delete(sessionKey)
		if err := storeSess.Save(); err != nil {
			log.Warn("csrf: failed to save session: ", err)
		}
	}
}
