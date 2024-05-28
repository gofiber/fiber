package csrf

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

type sessionManager struct {
	key     string
	session *session.Store
}

func newSessionManager(s *session.Store, k string) *sessionManager {
	// Create new storage handler
	sessionManager := &sessionManager{
		key: k,
	}
	if s != nil {
		// Use provided storage if provided
		sessionManager.session = s
	}
	return sessionManager
}

// get token from session
func (m *sessionManager) getRaw(c fiber.Ctx, key string, raw []byte) []byte {
	sess := session.FromContext(c)
	var token Token
	var ok bool

	if sess != nil {
		token, ok = sess.Get(m.key).(Token)
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return nil
		}
		token, ok = storeSess.Get(m.key).(Token)
	}

	if ok {
		if token.Expiration.Before(time.Now()) || key != token.Key || !compareTokens(raw, token.Raw) {
			return nil
		}
		return token.Raw
	}

	return nil
}

// set token in session
func (m *sessionManager) setRaw(c fiber.Ctx, key string, raw []byte, exp time.Duration) {
	sess := session.FromContext(c)
	if sess != nil {
		// the key is crucial in crsf and sometimes a reference to another value which can be reused later(pool/unsafe values concept), so a copy is made here
		sess.Set(m.key, &Token{key, raw, time.Now().Add(exp)})
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return
		}
		storeSess.Set(m.key, &Token{key, raw, time.Now().Add(exp)})
	}
}

// delete token from session
func (m *sessionManager) delRaw(c fiber.Ctx) {
	sess := session.FromContext(c)
	if sess != nil {
		sess.Delete(m.key)
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return
		}
		storeSess.Delete(m.key)
	}
}
