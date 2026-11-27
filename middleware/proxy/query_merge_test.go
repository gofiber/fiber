package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_JoinUpstreamPath_QueryMerge(t *testing.T) {
	t.Parallel()

	baseWithQuery, err := url.Parse("http://upstream.example/api?apiKey=secret")
	require.NoError(t, err)

	baseWithoutQuery, err := url.Parse("http://upstream.example/api")
	require.NoError(t, err)

	t.Run("default merges upstream base query with client query", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "http://upstream.example/api/items?apiKey=secret", joinUpstreamPath(baseWithQuery, "/items"))
		require.Equal(t, "http://upstream.example/api/items?apiKey=secret&filter=active", joinUpstreamPath(baseWithQuery, "/items?filter=active"))
		require.Equal(t, "http://upstream.example/api/items?apiKey=secret#section", joinUpstreamPath(baseWithQuery, "/items#section"))
		require.Equal(t, "http://upstream.example/api/items?apiKey=secret&filter=active#top", joinUpstreamPath(baseWithQuery, "/items?filter=active#top"))
		require.Equal(t, "http://upstream.example/api/%25zz?apiKey=secret", joinUpstreamPath(baseWithQuery, "/%zz"))
		require.Equal(t, "http://upstream.example/api/items?filter=active", joinUpstreamPath(baseWithoutQuery, "/items?filter=active"))
	})

	t.Run("disableQueryMerge keeps legacy client-replaces-base behavior", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "http://upstream.example/api/items", joinUpstreamPathWithPolicy(baseWithQuery, "/items", true))
		require.Equal(t, "http://upstream.example/api/items?filter=active", joinUpstreamPathWithPolicy(baseWithQuery, "/items?filter=active", true))
		require.Equal(t, "http://upstream.example/api/%25zz", joinUpstreamPathWithPolicy(baseWithQuery, "/%zz", true))
		require.Equal(t, "http://upstream.example/api/items?filter=active", joinUpstreamPathWithPolicy(baseWithoutQuery, "/items?filter=active", true))
	})
}

func Test_DomainForward_UpstreamQueryMerge(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.RequestURI()))
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL + "/api?apiKey=secret"

	t.Run("default preserves and merges upstream query", func(t *testing.T) {
		policy := DefaultSecurityPolicy()
		policy.AllowPrivateIPs = true
		prev := WithSecurityPolicy(policy)
		t.Cleanup(func() { WithSecurityPolicy(prev) })

		app := fiber.New()
		app.Use(DomainForward("api.example.com", baseURL))

		req := httptest.NewRequest(http.MethodGet, "http://api.example.com/items", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/api/items?apiKey=secret", string(body))

		reqWithQuery := httptest.NewRequest(http.MethodGet, "http://api.example.com/items?filter=active", http.NoBody)
		resp2, err := app.Test(reqWithQuery)
		require.NoError(t, err)
		body2, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		require.NoError(t, resp2.Body.Close())
		require.Equal(t, "/api/items?apiKey=secret&filter=active", string(body2))
	})

	t.Run("DisableQueryMerge replaces upstream query with client query", func(t *testing.T) {
		policy := DefaultSecurityPolicy()
		policy.AllowPrivateIPs = true
		policy.DisableQueryMerge = true
		prev := WithSecurityPolicy(policy)
		t.Cleanup(func() { WithSecurityPolicy(prev) })

		app := fiber.New()
		app.Use(DomainForward("api.example.com", baseURL))

		req := httptest.NewRequest(http.MethodGet, "http://api.example.com/items", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/api/items", string(body))

		reqWithQuery := httptest.NewRequest(http.MethodGet, "http://api.example.com/items?filter=active", http.NoBody)
		resp2, err := app.Test(reqWithQuery)
		require.NoError(t, err)
		body2, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		require.NoError(t, resp2.Body.Close())
		require.Equal(t, "/api/items?filter=active", string(body2))
	})
}

func Test_BalancerForward_UpstreamQueryMerge(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.RequestURI()))
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL + "/v1?tenant=acme"

	t.Run("default preserves and merges upstream query", func(t *testing.T) {
		policy := DefaultSecurityPolicy()
		policy.AllowPrivateIPs = true
		prev := WithSecurityPolicy(policy)
		t.Cleanup(func() { WithSecurityPolicy(prev) })

		app := fiber.New()
		app.Use(BalancerForward([]string{baseURL}))

		req := httptest.NewRequest(http.MethodGet, "/orders?status=open", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/v1/orders?tenant=acme&status=open", string(body))
	})

	t.Run("DisableQueryMerge replaces upstream query with client query", func(t *testing.T) {
		policy := DefaultSecurityPolicy()
		policy.AllowPrivateIPs = true
		policy.DisableQueryMerge = true
		prev := WithSecurityPolicy(policy)
		t.Cleanup(func() { WithSecurityPolicy(prev) })

		app := fiber.New()
		app.Use(BalancerForward([]string{baseURL}))

		req := httptest.NewRequest(http.MethodGet, "/orders?status=open", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "/v1/orders?status=open", string(body))
	})
}
