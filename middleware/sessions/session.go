package sessions

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

type Session struct {
	ctx      *fiber.Ctx
	sessions *Sessions
	db       *db
	id       string
	fresh    bool
}

// Fresh is true if the current session is new or existing
func (s *Session) Fresh() bool {
	return s.fresh
}

// ID returns the session id
func (s *Session) ID() string {
	return s.id
}

// Get will return the value
func (s *Session) Get(key string) interface{} {
	return s.db.Get(key)
}

// Set will update or create a new key value
func (s *Session) Set(key string, val interface{}) {
	s.db.Set(key, val)
}

// Delete will delete the value
func (s *Session) Delete(key string) {
	s.db.Delete(key)
}

// Reset will clear the session and remove from storage
func (s *Session) Reset() error {
	s.db.Reset()
	return s.sessions.cfg.Store.Delete(s.id)
}

// Save will update the storage and client cookie
func (s *Session) Save() error {
	// Expire session if no data is present ( aka reset )
	if s.db.Len() <= 0 {
		// Delete cookie
		s.deleteCookie()
		return nil
	}

	// Convert book to bytes
	data, err := s.db.MarshalMsg(nil)
	if err != nil {
		return err
	}

	// pass raw bytes with session id to provider
	if err := s.sessions.cfg.Store.Set(s.id, data, s.sessions.cfg.Expiration); err != nil {
		return err
	}

	// release db back to pool to be re-used on next request
	releaseDB(s.db)

	// Create cookie with the session ID
	s.setCookie()

	return nil
}

func (s *Session) setCookie() {
	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(s.sessions.cfg.Cookie.Name)
	fcookie.SetValue(s.id)
	fcookie.SetPath(s.sessions.cfg.Cookie.Path)
	fcookie.SetDomain(s.sessions.cfg.Cookie.Domain)
	fcookie.SetMaxAge(int(s.sessions.cfg.Expiration))
	fcookie.SetExpire(time.Now().Add(s.sessions.cfg.Expiration))
	fcookie.SetSecure(s.sessions.cfg.Cookie.Secure)
	fcookie.SetHTTPOnly(s.sessions.cfg.Cookie.HTTPOnly)

	switch utils.ToLower(s.sessions.cfg.Cookie.SameSite) {
	case "strict":
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case "none":
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	}

	s.ctx.Response().Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}

func (s *Session) deleteCookie() {
	s.ctx.Request().Header.DelCookie(s.sessions.cfg.Cookie.Name)
	s.ctx.Response().Header.DelCookie(s.sessions.cfg.Cookie.Name)

	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(s.sessions.cfg.Cookie.Name)
	fcookie.SetValue(s.id)
	fcookie.SetPath(s.sessions.cfg.Cookie.Path)
	fcookie.SetDomain(s.sessions.cfg.Cookie.Domain)
	fcookie.SetMaxAge(int(s.sessions.cfg.Expiration))
	fcookie.SetExpire(time.Now().Add(-1 * time.Minute))
	fcookie.SetSecure(s.sessions.cfg.Cookie.Secure)
	fcookie.SetHTTPOnly(s.sessions.cfg.Cookie.HTTPOnly)

	switch utils.ToLower(s.sessions.cfg.Cookie.SameSite) {
	case "strict":
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case "none":
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	}

	s.ctx.Response().Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}
