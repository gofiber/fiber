package encryptcookie

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
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

func Test_Middleware_Decrypt_Invalid_Cookie_Does_Not_Panic(t *testing.T) {
	t.Parallel()

	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("value=" + c.Cookies("test"))
	})

	// Send a request with an unencrypted/invalid cookie value
	// This should not panic and should clear the cookie value
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{
		Name:  "test",
		Value: "plaintext-unencrypted-value",
	})

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// The cookie value should be empty since decryption failed
	body := make([]byte, 64)
	n, err := resp.Body.Read(body)
	require.NoError(t, err)
	require.Equal(t, "value=", string(body[:n]))
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
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.ErrorIs(t, captured, expected)
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
