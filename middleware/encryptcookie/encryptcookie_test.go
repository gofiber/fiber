package encryptcookie

import (
	"crypto/rand"
	"encoding/base64"
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
		tt := tt
		t.Run(strconv.Itoa(tt.length)+"_length_encrypt", func(t *testing.T) {
			t.Parallel()
			key := make([]byte, tt.length)
			_, err := rand.Read(key)
			require.NoError(t, err)
			keyString := base64.StdEncoding.EncodeToString(key)

			_, err = EncryptCookie("SomeThing", keyString)
			require.Error(t, err)
		})

		t.Run(strconv.Itoa(tt.length)+"_length_decrypt", func(t *testing.T) {
			t.Parallel()
			key := make([]byte, tt.length)
			_, err := rand.Read(key)
			require.NoError(t, err)
			keyString := base64.StdEncoding.EncodeToString(key)

			_, err = DecryptCookie("SomeThing", keyString)
			require.Error(t, err)
		})
	}
}

func Test_Middleware_InvalidBase64(t *testing.T) {
	t.Parallel()
	invalidBase64 := "invalid-base64-string-!@#"

	t.Run("encryptor", func(t *testing.T) {
		t.Parallel()
		_, err := EncryptCookie("SomeText", invalidBase64)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to base64-decode key")
	})

	t.Run("decryptor_key", func(t *testing.T) {
		t.Parallel()
		_, err := DecryptCookie("SomeText", invalidBase64)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to base64-decode key")
	})

	t.Run("decryptor_value", func(t *testing.T) {
		t.Parallel()
		_, err := DecryptCookie(invalidBase64, GenerateKey(32))
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to base64-decode value")
	})
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
	decryptedCookieValue, err := DecryptCookie(string(encryptedCookie.Value()), testKey)
	require.NoError(t, err)
	require.Equal(t, "SomeThing", decryptedCookieValue)

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("test", string(encryptedCookie.Value()))
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "value=SomeThing", string(ctx.Response.Body()))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
	decryptedCookieValue, err := DecryptCookie(string(encryptedCookie.Value()), testKey)
	require.NoError(t, err)
	require.Equal(t, "SomeThing", decryptedCookieValue)
}

func Test_Encrypt_Cookie_Custom_Encryptor(t *testing.T) {
	t.Parallel()
	testKey := GenerateKey(32)
	app := fiber.New()

	app.Use(New(Config{
		Key: testKey,
		Encryptor: func(decryptedString, _ string) (string, error) {
			return base64.StdEncoding.EncodeToString([]byte(decryptedString)), nil
		},
		Decryptor: func(encryptedString, _ string) (string, error) {
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
		tt := tt
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
		for i := 0; i < b.N; i++ {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			h(ctx)
		}
	})

	b.Run("Invalid Cookie", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.Header.SetCookie("test", "Invalid")
			h(ctx)
		}
	})

	b.Run("Valid Cookie", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
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
		for i := 0; i < b.N; i++ {
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
		for i := 0; i < b.N; i++ {
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
		Encryptor: func(decryptedString, _ string) (string, error) {
			return base64.StdEncoding.EncodeToString([]byte(decryptedString)), nil
		},
		Decryptor: func(encryptedString, _ string) (string, error) {
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
		for i := 0; i < b.N; i++ {
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

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
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
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				ctx := &fasthttp.RequestCtx{}
				ctx.Request.Header.SetMethod(fiber.MethodGet)
				h(ctx)
			}
		})
	})

	b.Run("Invalid Cookie Parallel", func(b *testing.B) {
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
		Encryptor: func(decryptedString, _ string) (string, error) {
			return base64.StdEncoding.EncodeToString([]byte(decryptedString)), nil
		},
		Decryptor: func(encryptedString, _ string) (string, error) {
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
			for i := 0; i < b.N; i++ {
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
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					GenerateKey(tt.length)
				}
			})
		})
	}
}
