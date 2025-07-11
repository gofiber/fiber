package main

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/stretchr/testify/require"
)

func TestNewAppHealthEndpoints(t *testing.T) {
	t.Parallel()
	opts := options{Dir: t.TempDir(), Path: "/", Health: true}
	app := newApp(opts)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, healthcheck.LivenessEndpoint, nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestNewAppServeIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("hello"), 0o644)
	require.NoError(t, err)

	opts := options{Dir: dir, Path: "/", Index: "index.html", Cache: time.Second}
	app := newApp(opts)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "hello")
}
