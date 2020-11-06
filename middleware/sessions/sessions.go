package sessions

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

type Config struct {
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	// - "cookie:<name>"
	//
	// Optional. Default value "cookie:_csrf".
	// TODO: When to override Cookie.Value?
	KeyLookup string

	// Optional. Session ID generator function.
	//
	// Default: utils.UUID
	KeyGenerator func() string

	// Optional. Cookie to set values on
	//
	// NOTE: Value, MaxAge and Expires will be overriden by the session ID and expiration
	// TODO: Should this be a pointer, if yes why?
	Cookie fiber.Cookie

	// Allowed session duration
	//
	// Optional. Default: 24 hours
	Expiration time.Duration

	// Store interface
	// Optional. Default: memory.New
	Store fiber.Storage
}

var ConfigDefault = Config{
	Cookie: fiber.Cookie{
		Value: "session_id",
	},
	Expiration:   30 * time.Minute,
	KeyGenerator: utils.UUID,
}

type Sessions struct {
	cfg Config
}

func New(config ...Config) *Sessions {
	cfg := ConfigDefault

	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.Store == nil {
		cfg.Store = memoryStorage()
	}

	return &Sessions{
		cfg: cfg,
	}
}

func (s *Sessions) Get(c *fiber.Ctx) *Session {
	var fresh bool

	// Get ID from cookie
	id := c.Cookies(s.cfg.Cookie.Name)

	// If no ID exist, create new one
	if len(id) == 0 {
		id = s.cfg.KeyGenerator()
		fresh = true
	}

	// Create session object
	sess := &Session{
		ctx:      c,
		sessions: s,
		fresh:    fresh,
		db:       acquireDB(),
		id:       id,
	}

	// Fetch existing data
	if !fresh {
		raw, err := s.cfg.Store.Get(id)

		// Set data
		if err == nil {
			_, err := sess.db.UnmarshalMsg(raw)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	// Return session object
	return sess
}

func (s *Sessions) Reset() error {
	return s.cfg.Store.Reset()
}
