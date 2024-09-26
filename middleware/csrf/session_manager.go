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

// get token from session
func (m *sessionManager) getRaw(c fiber.Ctx, key string, raw []byte) []byte {
	sess := session.FromContext(c)
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

// set token in session
func (m *sessionManager) setRaw(c fiber.Ctx, key string, raw []byte, exp time.Duration) {
	sess := session.FromContext(c)
	if sess != nil {
		// the key is crucial in crsf and sometimes a reference to another value which can be reused later(pool/unsafe values concept), so a copy is made here
		sess.Set(sessionKey, Token{Key: key, Raw: raw, Expiration: time.Now().Add(exp)})
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return
		}
		storeSess.Set(sessionKey, Token{Key: key, Raw: raw, Expiration: time.Now().Add(exp)})
		if err := storeSess.Save(); err != nil {
			log.Warn("csrf: failed to save session: ", err)
		}
	}
}

// delete token from session
func (m *sessionManager) delRaw(c fiber.Ctx) {
	sess := session.FromContext(c)
	if sess != nil {
		sess.Delete(sessionKey)
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return
		}
		storeSess.Delete(sessionKey)
		if err := storeSess.Save(); err != nil {
			log.Warn("csrf: failed to save session: ", err)
		}
	}
}
