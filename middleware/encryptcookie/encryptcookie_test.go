package encryptcookie

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"
)

func Test_Middleware_Panics(t *testing.T) {
	t.Parallel()

	t.Run("Empty Key", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		require.Panics(t, func() {
			app.Use(New(Config{
				Key: "",
			}))
		})
	})

	t.Run("Invalid Key", func(t *testing.T) {
		t.Parallel()
		require.Panics(t, func() {
			GenerateKey(11)
		})
	})
}

func Test_Middleware_InvalidKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		length int
	}{
		{length: 11},
		{length: 25},
		{length: 60},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.length)+"_length_encrypt", func(t *testing.T) {
			t.Parallel()
			key := make([]byte, tt.length)
			_, err := rand.Read(key)
			require.NoError(t, err)
			keyString := base64.StdEncoding.EncodeToString(key)

			_, err = EncryptCookie("test", "SomeThing", keyString)
			require.Error(t, err)
		})

		t.Run(strconv.Itoa(tt.length)+"_length_decrypt", func(t *testing.T) {
			t.Parallel()
			key := make([]byte, tt.length)
			_, err := rand.Read(key)
			require.NoError(t, err)
			keyString := base64.StdEncoding.EncodeToString(key)

			_, err = DecryptCookie("test", "SomeThing", keyString)
			require.Error(t, err)
		})
	}
}

func Test_Middleware_InvalidBase64(t *testing.T) {
	t.Parallel()
	invalidBase64 := "invalid-base64-string-!@#"

	t.Run("encryptor", func(t *testing.T) {
		t.Parallel()
		_, err := EncryptCookie("test", "SomeText", invalidBase64)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to base64-decode key")
	})

	t.Run("decryptor_key", func(t *testing.T) {
		t.Parallel()
		_, err := DecryptCookie("test", "SomeText", invalidBase64)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to base64-decode key")
	})

	t.Run("decryptor_value", func(t *testing.T) {
		t.Parallel()
		_, err := DecryptCookie("test", invalidBase64, GenerateKey(32))
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to base64-decode value")
	})
}

func Test_DecryptCookie_InvalidEncryptedValue(t *testing.T) {
	t.Parallel()

	key := GenerateKey(32)
	// the decoded value is shorter than the GCM nonce size, so decryption should fail immediately
	shortValue := base64.StdEncoding.EncodeToString([]byte("short"))

	_, err := DecryptCookie("session", shortValue, key)
	require.ErrorIs(t, err, ErrInvalidEncryptedValue)
}

func Test_Middleware_EncryptionErrorPropagates(t *testing.T) {
	t.Parallel()

	testKey := GenerateKey(32)
	expected := errors.New("encrypt failed")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusTeapot).SendString("encryption error")
		},
	})

	app.Use(New(Config{
		Key: testKey,
		Encryptor: func(name, value, _ string) (string, error) {
			if name == "test" {
				return "", expected
			}
			return value, nil
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "value",
		})
		c.Cookie(&fiber.Cookie{
			Name:  "test2",
			Value: "value2",
		})
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.ErrorIs(t, captured, expected)
	require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie))
}

func Test_Middleware_EncryptionErrorDoesNotMaskNextError(t *testing.T) {
	t.Parallel()

	testKey := GenerateKey(32)
	encryptErr := errors.New("encrypt failed")
	downstreamErr := errors.New("downstream failed")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusTeapot).SendString("combined error")
		},
	})

	app.Use(New(Config{
		Key: testKey,
		Encryptor: func(name, value, _ string) (string, error) {
			if name == "test" {
				return "", encryptErr
			}
			return value, nil
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "value",
		})
		return downstreamErr
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.ErrorIs(t, captured, downstreamErr)
	require.ErrorIs(t, captured, encryptErr)
}

func Test_Middleware_Encrypt_Cookie(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	h := app.Handler()

	// Test empty cookie
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "value=", string(ctx.Response.Body()))

	// Test invalid cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("test", "Invalid")
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "value=", string(ctx.Response.Body()))
	ctx.Request.Header.SetCookie("test", "ixQURE2XOyZUs0WAOh2ehjWcP7oZb07JvnhWOsmeNUhPsj4+RyI=")
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "value=", string(ctx.Response.Body()))

	// Test valid cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	encryptedCookie := fasthttp.Cookie{}
	encryptedCookie.SetKey("test")
	require.True(t, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")
	decryptedCookieValue, err := DecryptCookie("test", string(encryptedCookie.Value()), testKey)
	require.NoError(t, err)
	require.Equal(t, "SomeThing", decryptedCookieValue)

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("test", string(encryptedCookie.Value()))
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "value=SomeThing", string(ctx.Response.Body()))
}

func Test_EncryptCookie_Rejects_Swapped_Names(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)

	encryptedValue, err := EncryptCookie("cookieA", "ValueA", testKey)
	require.NoError(t, err)

	decryptedValue, err := DecryptCookie("cookieA", encryptedValue, testKey)
	require.NoError(t, err)
	require.Equal(t, "ValueA", decryptedValue)

	_, err = DecryptCookie("cookieB", encryptedValue, testKey)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to decrypt ciphertext")
}

func Test_Encrypt_Cookie_Next(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "SomeThing", resp.Cookies()[0].Value)
}

func Test_Encrypt_Cookie_Except(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Except: []string{
			"test1",
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test1",
			Value: "SomeThing",
		})
		c.Cookie(&fiber.Cookie{
			Name:  "test2",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	rawCookie := fasthttp.Cookie{}
	rawCookie.SetKey("test1")
	require.True(t, ctx.Response.Header.Cookie(&rawCookie), "Get cookie value")
	require.Equal(t, "SomeThing", string(rawCookie.Value()))

	encryptedCookie := fasthttp.Cookie{}
	encryptedCookie.SetKey("test2")
	require.True(t, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")
	decryptedCookieValue, err := DecryptCookie("test2", string(encryptedCookie.Value()), testKey)
	require.NoError(t, err)
	require.Equal(t, "SomeThing", decryptedCookieValue)
}

func Test_Encrypt_Cookie_Custom_Encryptor(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Encryptor: func(_, decryptedString, _ string) (string, error) {
			return base64.StdEncoding.EncodeToString([]byte(decryptedString)), nil
		},
		Decryptor: func(_, encryptedString, _ string) (string, error) {
			decodedBytes, err := base64.StdEncoding.DecodeString(encryptedString)
			return string(decodedBytes), err
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	encryptedCookie := fasthttp.Cookie{}
	encryptedCookie.SetKey("test")
	require.True(t, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")
	decodedBytes, err := base64.StdEncoding.DecodeString(string(encryptedCookie.Value()))
	require.NoError(t, err)
	require.Equal(t, "SomeThing", string(decodedBytes))

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("test", string(encryptedCookie.Value()))
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "value=SomeThing", string(ctx.Response.Body()))
}

func Test_GenerateKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		length int
	}{
		{length: 16},
		{length: 24},
		{length: 32},
	}

	decodeBase64 := func(t *testing.T, s string) []byte {
		t.Helper()
		data, err := base64.StdEncoding.DecodeString(s)
		require.NoError(t, err)
		return data
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.length)+"_length", func(t *testing.T) {
			t.Parallel()
			key := GenerateKey(tt.length)
			decodedKey := decodeBase64(t, key)
			require.Len(t, decodedKey, tt.length)
		})
	}

	t.Run("Invalid Length", func(t *testing.T) {
		require.Panics(t, func() { GenerateKey(10) })
		require.Panics(t, func() { GenerateKey(20) })
	})
}

func Benchmark_Middleware_Encrypt_Cookie(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	h := app.Handler()

	b.Run("Empty Cookie", func(b *testing.B) {
		for b.Loop() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			h(ctx)
		}
	})

	b.Run("Invalid Cookie", func(b *testing.B) {
		for b.Loop() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.Header.SetCookie("test", "Invalid")
			h(ctx)
		}
	})

	b.Run("Valid Cookie", func(b *testing.B) {
		for b.Loop() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodPost)
			h(ctx)
		}
	})
}

func Benchmark_Encrypt_Cookie_Next(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	h := app.Handler()

	b.Run("Encrypt Cookie Next", func(b *testing.B) {
		for b.Loop() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.SetRequestURI("/")
			h(ctx)
		}
	})
}

func Benchmark_Encrypt_Cookie_Except(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Except: []string{
			"test1",
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test1",
			Value: "SomeThing",
		})
		c.Cookie(&fiber.Cookie{
			Name:  "test2",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()

	b.Run("Encrypt Cookie Except", func(b *testing.B) {
		for b.Loop() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			h(ctx)
		}
	})
}

func Benchmark_Encrypt_Cookie_Custom_Encryptor(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Encryptor: func(_, decryptedString, _ string) (string, error) {
			return base64.StdEncoding.EncodeToString([]byte(decryptedString)), nil
		},
		Decryptor: func(_, encryptedString, _ string) (string, error) {
			decodedBytes, err := base64.StdEncoding.DecodeString(encryptedString)
			return string(decodedBytes), err
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()

	b.Run("Custom Encryptor Post", func(b *testing.B) {
		for b.Loop() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodPost)
			h(ctx)
		}
	})

	b.Run("Custom Encryptor Get", func(b *testing.B) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		h(ctx)
		encryptedCookie := fasthttp.Cookie{}
		encryptedCookie.SetKey("test")
		require.True(b, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")

		for b.Loop() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.Header.SetCookie("test", string(encryptedCookie.Value()))
			h(ctx)
		}
	})
}

func Benchmark_Middleware_Encrypt_Cookie_Parallel(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	h := app.Handler()

	b.Run("Empty Cookie Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				ctx := &fasthttp.RequestCtx{}
				ctx.Request.Header.SetMethod(fiber.MethodGet)
				h(ctx)
			}
		})
	})

	b.Run("Invalid Cookie Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				ctx := &fasthttp.RequestCtx{}
				ctx.Request.Header.SetMethod(fiber.MethodGet)
				ctx.Request.Header.SetCookie("test", "Invalid")
				h(ctx)
			}
		})
	})

	b.Run("Valid Cookie Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				ctx := &fasthttp.RequestCtx{}
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				h(ctx)
			}
		})
	})
}

func Benchmark_Encrypt_Cookie_Next_Parallel(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})
		return nil
	})

	h := app.Handler()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.SetRequestURI("/")
			h(ctx)
		}
	})
}

func Benchmark_Encrypt_Cookie_Except_Parallel(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Except: []string{
			"test1",
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test1",
			Value: "SomeThing",
		})
		c.Cookie(&fiber.Cookie{
			Name:  "test2",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			h(ctx)
		}
	})
}

func Benchmark_Encrypt_Cookie_Custom_Encryptor_Parallel(b *testing.B) {
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Encryptor: func(_, decryptedString, _ string) (string, error) {
			return base64.StdEncoding.EncodeToString([]byte(decryptedString)), nil
		},
		Decryptor: func(_, encryptedString, _ string) (string, error) {
			decodedBytes, err := base64.StdEncoding.DecodeString(encryptedString)
			return string(decodedBytes), err
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "test",
			Value: "SomeThing",
		})

		return nil
	})

	h := app.Handler()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		h(ctx)
		encryptedCookie := fasthttp.Cookie{}
		encryptedCookie.SetKey("test")
		require.True(b, ctx.Response.Header.Cookie(&encryptedCookie), "Get cookie value")

		b.ResetTimer()
		for pb.Next() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.Header.SetCookie("test", string(encryptedCookie.Value()))
			h(ctx)
		}
	})
}

func Benchmark_GenerateKey(b *testing.B) {
	tests := []struct {
		length int
	}{
		{length: 16},
		{length: 24},
		{length: 32},
	}

	for _, tt := range tests {
		b.Run(strconv.Itoa(tt.length), func(b *testing.B) {
			for b.Loop() {
				GenerateKey(tt.length)
			}
		})
	}
}

func Benchmark_GenerateKey_Parallel(b *testing.B) {
	tests := []struct {
		length int
	}{
		{length: 16},
		{length: 24},
		{length: 32},
	}

	for _, tt := range tests {
		b.Run(strconv.Itoa(tt.length), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					GenerateKey(tt.length)
				}
			})
		})
	}
}

// Test_Middleware_Mixed_Valid_Invalid_Cookies tests that the middleware correctly handles
// a mix of valid and invalid cookies during iteration
func Test_Middleware_Mixed_Valid_Invalid_Cookies(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		valid1 := c.Cookies("valid1")
		valid2 := c.Cookies("valid2")
		invalid := c.Cookies("invalid")
		return c.SendString("valid1=" + valid1 + ",valid2=" + valid2 + ",invalid=" + invalid)
	})

	h := app.Handler()

	// First, create some valid encrypted cookies
	encryptedValue1, err := EncryptCookie("valid1", "value1", testKey)
	require.NoError(t, err)
	encryptedValue2, err := EncryptCookie("valid2", "value2", testKey)
	require.NoError(t, err)

	// Test with a mix of valid and invalid cookies
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("valid1", encryptedValue1)
	ctx.Request.Header.SetCookie("invalid", "thisisnotvalid")
	ctx.Request.Header.SetCookie("valid2", encryptedValue2)

	h(ctx)

	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "valid1=value1,valid2=value2,invalid=", string(ctx.Response.Body()))

	// Verify the invalid cookie was deleted but valid ones remain
	require.NotEmpty(t, ctx.Request.Header.Cookie("valid1"))
	require.Empty(t, ctx.Request.Header.Cookie("invalid"))
	require.NotEmpty(t, ctx.Request.Header.Cookie("valid2"))
}

// Test_Middleware_Duplicate_Cookie_Names covers a Cookie header that carries
// the same name more than once (RFC 6265 Section 5.4). The handler must see
// one entry per name holding the first value that decrypts, as c.Cookies
// returns the first plaintext cookie of a name, whatever else the header
// carries.
func Test_Middleware_Duplicate_Cookie_Names(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)

	encrypt := func(t *testing.T, name, value string) string {
		t.Helper()
		encrypted, err := EncryptCookie(name, value, testKey)
		require.NoError(t, err)
		return encrypted
	}

	newApp := func(except ...string) *fiber.App {
		app := fiber.New()
		app.Use(New(Config{
			Key:    testKey,
			Except: except,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("value=" + c.Cookies("test"))
		})
		return app
	}

	// request runs cookieHeader through app and returns the body and the
	// request cookies as the handler saw them, in header order.
	request := func(t *testing.T, app *fiber.App, cookieHeader string) (string, []string) {
		t.Helper()
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		ctx.Request.Header.Set(fiber.HeaderCookie, cookieHeader)
		app.Handler()(ctx)
		require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
		cookies := make([]string, 0, 4)
		for key, value := range ctx.Request.Header.Cookies() {
			cookies = append(cookies, string(key)+"="+string(value))
		}
		return string(ctx.Response.Body()), cookies
	}

	t.Run("without middleware the first cookie wins", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("value=" + c.Cookies("test"))
		})
		body, _ := request(t, app, "test=first; test=second")
		require.Equal(t, "value=first", body)
	})

	t.Run("both decrypt: first wins, one entry remains", func(t *testing.T) {
		t.Parallel()
		body, cookies := request(t, newApp(), "test="+encrypt(t, "test", "first")+"; test="+encrypt(t, "test", "second"))
		require.Equal(t, "value=first", body)
		require.Equal(t, []string{"test=first"}, cookies)
	})

	t.Run("first decrypts, second does not", func(t *testing.T) {
		t.Parallel()
		body, cookies := request(t, newApp(), "test="+encrypt(t, "test", "SomeThing")+"; test=not-encrypted")
		require.Equal(t, "value=SomeThing", body)
		require.Equal(t, []string{"test=SomeThing"}, cookies)
	})

	t.Run("first does not decrypt: next one that does wins", func(t *testing.T) {
		t.Parallel()
		body, cookies := request(t, newApp(), "test=garbage; test="+encrypt(t, "test", "second"))
		require.Equal(t, "value=second", body)
		require.Equal(t, []string{"test=second"}, cookies)
	})

	t.Run("first does not decrypt: the next one wins over a third", func(t *testing.T) {
		t.Parallel()
		body, cookies := request(t, newApp(), "test=garbage; test="+encrypt(t, "test", "second")+"; test="+encrypt(t, "test", "third"))
		require.Equal(t, "value=second", body)
		require.Equal(t, []string{"test=second"}, cookies)
	})

	t.Run("none decrypt: the name is removed", func(t *testing.T) {
		t.Parallel()
		body, cookies := request(t, newApp(), "test=junk; test=more-junk")
		require.Equal(t, "value=", body)
		require.Empty(t, cookies)
	})

	t.Run("deleting another name keeps the first value", func(t *testing.T) {
		t.Parallel()
		// DelCookie swaps the last entry into the hole, so a failed cookie of
		// another name used to move an unprocessed duplicate to the front.
		valid := encrypt(t, "test", "victim")
		for _, header := range []string{
			"junk=invalid; test=" + valid + "; test=attacker",
			"test=" + valid + "; junk=invalid; test=attacker",
			"junk=invalid; other=invalid; test=" + valid + "; test=attacker; test=more",
		} {
			body, cookies := request(t, newApp(), header)
			require.Equal(t, "value=victim", body, header)
			require.Equal(t, []string{"test=victim"}, cookies, header)
		}
	})

	t.Run("order of other names is preserved", func(t *testing.T) {
		t.Parallel()
		app := newApp()
		body, cookies := request(t, app, "a="+encrypt(t, "a", "1")+"; junk=invalid; test="+encrypt(t, "test", "first")+"; b="+encrypt(t, "b", "2")+"; test=junk")
		require.Equal(t, "value=first", body)
		require.Equal(t, []string{"a=1", "test=first", "b=2"}, cookies)
	})

	t.Run("excepted values survive a rebuild unchanged", func(t *testing.T) {
		t.Parallel()
		// A quoted value, or one an earlier handler set, is not expressible
		// through the wire parser; the rebuild must store it as it was.
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			c.Request().Header.SetCookie("set", `a"b\c`)
			return c.Next()
		})
		app.Use(New(Config{
			Key:    testKey,
			Except: []string{"set", "quoted"},
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString(c.Cookies("set") + "|" + c.Cookies("quoted"))
		})

		run := func(cookieHeader string) string {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.Header.Set(fiber.HeaderCookie, cookieHeader)
			app.Handler()(ctx)
			require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
			return string(ctx.Response.Body())
		}

		inPlace := run(`quoted=" a "`)
		require.True(t, strings.HasPrefix(inPlace, `a"b\c|`), inPlace)
		require.Equal(t, inPlace, run(`quoted=" a "; junk=invalid`))
		require.Equal(t, inPlace, run(`junk=invalid; quoted=" a "`))
	})

	t.Run("excepted duplicates are left alone", func(t *testing.T) {
		t.Parallel()
		body, cookies := request(t, newApp("test"), "test=first; test=second")
		require.Equal(t, "value=first", body)
		require.Equal(t, []string{"test=first", "test=second"}, cookies)

		// Also when another cookie forces the header to be rebuilt.
		body, cookies = request(t, newApp("test"), "test=first; junk=invalid; test=second")
		require.Equal(t, "value=first", body)
		require.Equal(t, []string{"test=first", "test=second"}, cookies)
	})
}

func Test_Middleware_Encrypts_Every_SetCookie_Line(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)

	type line struct{ value, attributes string }
	testCases := []struct {
		write func(c fiber.Ctx)
		name  string
		cname string
		want  []line
	}{
		{
			name: "one name at two paths",
			write: func(c fiber.Ctx) {
				c.Cookie(&fiber.Cookie{Name: "token", Path: "/app", MaxAge: -1})
				c.Add(fiber.HeaderSetCookie, "token=SomeThing; Path=/; HttpOnly")
			},
			cname: "token",
			want:  []line{{"", "; max-age=0; path=/app; SameSite=Lax"}, {"SomeThing", "; Path=/; HttpOnly"}},
		},
		{
			name: "a quoted value with a backslash",
			write: func(c fiber.Ctx) {
				c.Add(fiber.HeaderSetCookie, `token="Some\054Thing"; HttpOnly`)
			},
			cname: "token",
			want:  []line{{`Some\054Thing`, "; HttpOnly"}},
		},
		{
			name: "a percent-encoded path",
			write: func(c fiber.Ctx) {
				c.Cookie(&fiber.Cookie{Name: "token", Value: "SomeThing", Path: "/caf%C3%A9", HTTPOnly: true, Secure: true})
			},
			cname: "token",
			want:  []line{{"SomeThing", "; path=/café; HttpOnly; secure; SameSite=Lax"}},
		},
		{
			name: "an attribute fasthttp does not know",
			write: func(c fiber.Ctx) {
				c.Add(fiber.HeaderSetCookie, "token=SomeThing; Path=/; Priority=High")
			},
			cname: "token",
			want:  []line{{"SomeThing", "; Path=/; Priority=High"}},
		},
		{
			name: "spaces around name and value",
			write: func(c fiber.Ctx) {
				c.Add(fiber.HeaderSetCookie, "token = SomeThing ; Path=/")
			},
			cname: "token",
			want:  []line{{"SomeThing", "; Path=/"}},
		},
		{
			name: "a value with no name",
			write: func(c fiber.Ctx) {
				c.Add(fiber.HeaderSetCookie, "SomeThing; Path=/")
			},
			cname: "",
			want:  []line{{"SomeThing", "; Path=/"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Key: testKey}))
			app.Get("/", func(c fiber.Ctx) error {
				tc.write(c)
				return nil
			})

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			app.Handler()(ctx)

			var lines []string
			for _, l := range ctx.Response.Header.Cookies() {
				lines = append(lines, string(l))
			}
			require.Len(t, lines, len(tc.want))

			for i, l := range lines {
				encrypted, found := strings.CutPrefix(l, tc.cname+"=")
				require.True(t, found, l)
				encrypted, found = strings.CutSuffix(encrypted, tc.want[i].attributes)
				require.True(t, found, l)

				value, err := DecryptCookie(tc.cname, encrypted, testKey)
				require.NoError(t, err, l)
				require.Equal(t, tc.want[i].value, value)
			}
		})
	}
}

func Test_Middleware_EncryptionError_Removes_Every_SetCookie_Line(t *testing.T) {
	t.Parallel()
	expected := errors.New("encrypt failed")

	app := fiber.New()
	app.Use(New(Config{
		Key:    GenerateKey(32),
		Except: []string{"plain"},
		Encryptor: func(_, value, _ string) (string, error) {
			if value == "second" {
				return "", expected
			}
			return "encrypted", nil
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Add(fiber.HeaderSetCookie, "plain=one; Path=/a")
		c.Add(fiber.HeaderSetCookie, "token=first; Path=/a")
		c.Add(fiber.HeaderSetCookie, "token=second; Path=/b")
		c.Add(fiber.HeaderSetCookie, "token=third; Path=/c")
		c.Add(fiber.HeaderSetCookie, "plain=two; Path=/b")
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, []string{"plain=one; Path=/a", "plain=two; Path=/b"}, resp.Header.Values(fiber.HeaderSetCookie))
}

func Test_Middleware_Request_Cookies_Linear(t *testing.T) {
	t.Parallel()
	const n = 1 << 16
	errRefused := errors.New("refused")

	// A cheap Decryptor leaves the middleware's own bookkeeping as the cost
	app := fiber.New()
	app.Use(New(Config{
		Key:    GenerateKey(32),
		Except: []string{"plain"},
		Decryptor: func(name, value, _ string) (string, error) {
			if name == "token" {
				return value, nil
			}
			return "", errRefused
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Cookies("plain") + "|" + c.Cookies("token"))
	})
	h := app.Handler()

	parts := make([]string, n)
	for i := range parts {
		parts[i] = "c" + strconv.Itoa(i) + "="
	}
	refused := strings.Join(parts, "; ") + "; "

	testCases := []struct {
		name    string
		cookies string
		entries int
	}{
		{name: "refused names", cookies: refused + "plain=one; token=v", entries: 2},
		{name: "refused names before a repeated excepted name", cookies: refused + strings.Repeat("plain=one; ", n) + "token=v", entries: n + 1},
		{name: "repeated name behind others", cookies: strings.Repeat("plain=one; ", n) + strings.Repeat("token=v; ", n), entries: n + 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.Header.Set(fiber.HeaderCookie, tc.cookies)

			start := time.Now()
			h(ctx)
			// A walk of the list per cookie is n*n/2 steps, which takes many seconds
			require.Less(t, time.Since(start), 10*time.Second)

			require.Equal(t, "one|v", string(ctx.Response.Body()))
			entries := 0
			for range ctx.Request.Header.Cookies() {
				entries++
			}
			require.Equal(t, tc.entries, entries)
		})
	}
}

func Benchmark_Middleware_Request_Cookies_Refused(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{Key: GenerateKey(32)}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})
	h := app.Handler()

	// 1000 distinct names that do not decrypt fit the default 8 KB ReadBufferSize
	parts := make([]string, 1000)
	for i := range parts {
		parts[i] = "c" + strconv.Itoa(i) + "=x"
	}
	cookies := strings.Join(parts, "; ")

	b.ReportAllocs()
	b.SetBytes(int64(len(cookies)))
	for b.Loop() {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		ctx.Request.Header.Set(fiber.HeaderCookie, cookies)
		h(ctx)
	}
}

func Test_Middleware_LeavesTimedOutResponse(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Key: GenerateKey(32)}))
	ctxCh := make(chan fiber.Ctx, 1)
	release := make(chan struct{})
	defer close(release)
	app.Get("/", timeout.New(func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{Name: "late", Value: "plain"})
		<-c.Context().Done()
		ctxCh <- c
		<-release
		return nil
	}, timeout.Config{Timeout: 10 * time.Millisecond}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	c := <-ctxCh
	cookie := fasthttp.Cookie{}
	cookie.SetKey("late")
	require.True(t, c.Response().Header.Cookie(&cookie))
	require.Equal(t, "plain", string(cookie.Value()), "the timed-out handler still owns the response")
}
