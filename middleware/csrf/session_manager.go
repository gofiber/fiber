package csrf

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/session"
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
func (m *sessionManager) getRaw(c *fiber.Ctx, key string, raw []byte) []byte {
	sess, err := m.session.Get(c)
	if err != nil {
		return nil
	}
	token, ok := sess.Get(m.key).(Token)
	if ok {
		if token.Expiration.Before(time.Now()) || key != token.Key || !compareTokens(raw, token.Raw) {
			return nil
		}
		return token.Raw
	}

	return nil
}

// set token in session
func (m *sessionManager) setRaw(c *fiber.Ctx, key string, raw []byte, exp time.Duration) {
	sess, err := m.session.Get(c)
	if err != nil {
		return
	}
	// the key is crucial in crsf and sometimes a reference to another value which can be reused later(pool/unsafe values concept), so a copy is made here
	sess.Set(m.key, &Token{key, raw, time.Now().Add(exp)})
	if err := sess.Save(); err != nil {
		log.Warn("csrf: failed to save session: ", err)
	}
}

// delete token from session
func (m *sessionManager) delRaw(c *fiber.Ctx) {
	sess, err := m.session.Get(c)
	if err != nil {
		return
	}
	sess.Delete(m.key)
	if err := sess.Save(); err != nil {
		log.Warn("csrf: failed to save session: ", err)
	}
}
