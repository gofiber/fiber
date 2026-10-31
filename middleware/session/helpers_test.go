package session

import (
	"github.com/gofiber/fiber/v3"
)

// getSession is loadSession for the tests that only want the session, as they did
// before the store learned to say whose session ID a request caches.
func (s *Store) getSession(c fiber.Ctx) (*Session, error) {
	sess, _, err := s.loadSession(c)
	return sess, err
}
