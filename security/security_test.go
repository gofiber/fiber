package security

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func setupApp(handler fiber.Handler) *fiber.App {
	app := fiber.New()
	app.Get("/", handler)
	return app
}

func Test_APIKeyCookie(t *testing.T) {
	t.Parallel()

	app := setupApp(func(c fiber.Ctx) error {
		key, err := APIKeyCookie(c, "api")
		if err != nil {
			return err
		}
		return c.SendString(key)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "api", Value: "secret"})
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "secret", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	badApp := setupApp(func(c fiber.Ctx) error {
		_, err := APIKeyCookie(c, "")
		return err
	})
	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = badApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_APIKeyHeader(t *testing.T) {
	t.Parallel()

	app := setupApp(func(c fiber.Ctx) error {
		key, err := APIKeyHeader(c, "X-API-Key")
		if err != nil {
			return err
		}
		return c.SendString(key)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "secret", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	badApp := setupApp(func(c fiber.Ctx) error {
		_, err := APIKeyHeader(c, "")
		return err
	})
	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = badApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_APIKeyQuery(t *testing.T) {
	t.Parallel()

	app := setupApp(func(c fiber.Ctx) error {
		key, err := APIKeyQuery(c, "key")
		if err != nil {
			return err
		}
		return c.SendString(key)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/?key=secret", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "secret", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	badApp := setupApp(func(c fiber.Ctx) error {
		_, err := APIKeyQuery(c, "")
		return err
	})
	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = badApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_GetAuthorizationCredentials(t *testing.T) {
	t.Parallel()

	app := setupApp(func(c fiber.Ctx) error {
		cred, err := GetAuthorizationCredentials(c)
		if err != nil {
			return err
		}
		return c.JSON(cred)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "Bearer")
	require.Contains(t, string(body), "token")

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "badheader")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func Test_HTTPBearer(t *testing.T) {
	t.Parallel()

	app := setupApp(func(c fiber.Ctx) error {
		token, err := HTTPBearer(c)
		if err != nil {
			return err
		}
		return c.SendString(token)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "tok", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic foo")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func Test_HTTPBasic(t *testing.T) {
	t.Parallel()

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	app := setupApp(func(c fiber.Ctx) error {
		cred, err := HTTPBasic(c)
		if err != nil {
			return err
		}
		return c.JSON(cred)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "john")
	require.Contains(t, string(body), "doe")

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	bad := setupApp(func(c fiber.Ctx) error {
		_, err := HTTPBasic(c)
		return err
	})
	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic !!")
	resp, err = bad.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func Test_HTTPDigest(t *testing.T) {
	t.Parallel()

	app := setupApp(func(c fiber.Ctx) error {
		token, err := HTTPDigest(c)
		if err != nil {
			return err
		}
		return c.SendString(token)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Digest abc")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "abc", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer xyz")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}
