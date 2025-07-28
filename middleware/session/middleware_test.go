package session

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Session_Middleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/get", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		value, ok := sess.Get("key").(string)
		if !ok {
			return c.Status(fiber.StatusNotFound).SendString("key not found")
		}
		return c.SendString("value=" + value)
	})

	app.Post("/set", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// get a value from the body
		value := c.FormValue("value")
		sess.Set("key", value)
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/delete", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		sess.Delete("key")
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/reset", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		// Set a value to ensure it is cleared after reset
		sess.Set("key", "value")

		if err := sess.Reset(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Ensure value is cleared
		value, ok := sess.Get("key").(string)
		if ok || value != "" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/regenerate", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Set a value to ensure it is preserved after regeneration
		sess.Set("key", "value")

		// Regenerate the session ID
		if err := sess.Regenerate(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Ensure the session ID has changed but session data is preserved
		newID := sess.ID()
		if newID == "" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Check if the session data is still accessible
		value, ok := sess.Get("key").(string)
		if !ok || value != "value" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/destroy", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if err := sess.Destroy(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/fresh", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Reset the session to make it fresh
		if err := sess.Reset(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if sess.Fresh() {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	app.Post("/keys", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// get a value from the body
		value := c.FormValue("keys")
		for rawKey := range strings.SplitSeq(value, ",") {
			key := strings.TrimSpace(rawKey)
			if key == "" {
				continue
			}
			// Set each key in the session
			sess.Set(key, "value_"+key)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/keys", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		keys := sess.Keys()
		if len(keys) == 0 {
			return c.SendStatus(fiber.StatusNotFound)
		}
		// Keys may be of any type, so convert to string for display
		strKeys := []string{}
		for _, key := range keys {
			strKeys = append(strKeys, fmt.Sprintf("%v", key))
		}
		return c.SendString("keys=" + strings.Join(strKeys, ","))
	})

	// Test GET, SET, DELETE, RESET, REGENERATE, DESTROY by sending requests to the respective routes
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	h := app.Handler()
	h(ctx)
	require.Equal(t, fiber.StatusNotFound, ctx.Response.StatusCode())
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "key not found", string(ctx.Response.Body()))

	// Test POST /set
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/set")
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded") // Set the Content-Type
	ctx.Request.SetBodyString("value=hello")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test GET /get to check if the value was set
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	require.Equal(t, "value=hello", string(ctx.Response.Body()))

	// Test POST /delete to delete the value
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/delete")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test GET /get to check if the value was deleted
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusNotFound, ctx.Response.StatusCode())
	require.Equal(t, "key not found", string(ctx.Response.Body()))

	// Test POST /reset to reset the session
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/reset")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// verify we have a new session token
	newToken := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	newTokenParts := strings.SplitN(strings.SplitN(newToken, ";", 2)[0], "=", 2)
	require.Len(t, newTokenParts, 2, "Expected Set-Cookie header to contain a token")
	newToken = newTokenParts[1]
	require.NotEqual(t, token, newToken)
	token = newToken

	// Test POST /regenerate to regenerate the session ID
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/regenerate")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// verify we have a new session token
	newToken = string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	newTokenParts = strings.SplitN(strings.SplitN(newToken, ";", 2)[0], "=", 2)
	require.Len(t, newTokenParts, 2, "Expected Set-Cookie header to contain a token")
	newToken = newTokenParts[1]
	require.NotEqual(t, token, newToken)
	token = newToken

	// Test POST /destroy to destroy the session
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/destroy")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Verify the session cookie has expired
	setCookieHeader := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.Contains(t, setCookieHeader, "max-age=0")

	// Sleep so that the session expires
	time.Sleep(1 * time.Second)

	// Test GET /get to check if the session was destroyed
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusNotFound, ctx.Response.StatusCode())
	// check that we have a new session token
	newToken = string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	parts := strings.Split(newToken, ";")
	require.Greater(t, len(parts), 1)
	valueParts := strings.Split(parts[0], "=")
	require.Greater(t, len(valueParts), 1)
	newToken = valueParts[1]
	require.NotEqual(t, token, newToken)
	token = newToken

	// Test POST /fresh to check if the session is fresh
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/fresh")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// check that we have a new session token
	newToken = string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	newTokenParts = strings.SplitN(strings.SplitN(newToken, ";", 2)[0], "=", 2)
	require.Len(t, newTokenParts, 2, "Expected Set-Cookie header to contain a token")
	newToken = newTokenParts[1]
	require.NotEqual(t, token, newToken)

	token = newToken

	// Test POST /keys to set multiple keys
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/keys")
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded") // Set the Content-Type
	ctx.Request.SetBodyString("keys=key1,key2")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test GET /keys to check if the session has the keys
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/keys")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	require.True(t, strings.HasPrefix(body, "keys="))
	parts = strings.Split(strings.TrimPrefix(body, "keys="), ",")
	require.Len(t, parts, 2, "Expected two keys in the session")
	sort.Strings(parts)
	require.Equal(t, []string{"key1", "key2"}, parts)
}

func Test_Session_NewWithStore(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		return c.SendString("value=" + id)
	})
	app.Post("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		c.Cookie(&fiber.Cookie{
			Name:  "session_id",
			Value: id,
		})
		return nil
	})

	h := app.Handler()

	// Test GET request without cookie
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// Get session cookie
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test GET request with cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	require.Equal(t, "value="+token, string(ctx.Response.Body()))
}

func Test_Session_FromSession(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	sess := FromContext(app.AcquireCtx(&fasthttp.RequestCtx{}))
	require.Nil(t, sess)

	app.Use(New())
}

func Test_Session_WithConfig(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Get("key") == "value"
		},
		IdleTimeout: 1 * time.Second,
		Extractor:   FromCookie("session_id_test"),
		KeyGenerator: func() string {
			return "test"
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		return c.SendString("value=" + id)
	})

	app.Get("/isFresh", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess.Fresh() {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	app.Post("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		c.Cookie(&fiber.Cookie{
			Name:  "session_id_test",
			Value: id,
		})
		return nil
	})

	h := app.Handler()

	// Test GET request without cookie
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// Get session cookie
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test GET request with cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id_test", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test POST request with cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie("session_id_test", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test POST request without cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test POST request with wrong key
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test POST request with wrong value
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie("session_id_test", "wrong")
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Check idle timeout not expired
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id_test", token)
	ctx.Request.SetRequestURI("/isFresh")
	h(ctx)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response.StatusCode())

	// Test idle timeout
	time.Sleep(1200 * time.Millisecond)
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id_test", token)
	ctx.Request.SetRequestURI("/isFresh")
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
}

func Test_Session_Next(t *testing.T) {
	t.Parallel()

	var (
		doNext bool
		muNext sync.RWMutex
	)

	app := fiber.New()

	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			muNext.RLock()
			defer muNext.RUnlock()
			return doNext
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		id := sess.ID()
		return c.SendString("value=" + id)
	})

	h := app.Handler()

	// Test with Next returning false
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// Get session cookie
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test with Next returning true
	muNext.Lock()
	doNext = true
	muNext.Unlock()

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response.StatusCode())
}

func Test_Session_Middleware_Store(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	handler, sessionStore := NewWithStore()

	app.Use(handler)

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		st := sess.Store()
		if st != sessionStore {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	// Test GET request
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
}
