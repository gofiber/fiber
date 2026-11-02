package envvar

import (
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

// envvarTestVars sends a GET for target through app and decodes the JSON it is answered with.
func envvarTestVars(t *testing.T, app *fiber.App, target string) EnvVar {
	t.Helper()

	resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, target))

	var envVars EnvVar
	require.NoError(t, json.Unmarshal(apptest.BodyBytes(t, resp), &envVars))
	return envVars
}

func Test_EnvVarStructWithExportVars(t *testing.T) {
	t.Setenv("testKey", "testEnvValue")
	t.Setenv("anotherEnvKey", "anotherEnvVal")
	vars := newEnvVar(Config{ExportVars: map[string]string{"testKey": "", "testDefaultKey": "testDefaultVal"}})

	require.Equal(t, "testEnvValue", vars.Vars["testKey"])
	require.Equal(t, "testDefaultVal", vars.Vars["testDefaultKey"])
	require.Empty(t, vars.Vars["anotherEnvKey"])
}

func Test_EnvVarHandler(t *testing.T) {
	t.Setenv("testKey", "testVal")

	app := fiber.New()
	app.Use("/envvars", New(Config{ExportVars: map[string]string{"testKey": ""}}))

	req := apptest.Req(fiber.MethodGet, "http://localhost/envvars")
	apptest.ExpectBody(t, app, req, fiber.StatusOK, `{"vars":{"testKey":"testVal"}}`)
}

func Test_EnvVarHandlerNotMatched(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use("/envvars", New(Config{ExportVars: map[string]string{"testKey": ""}}))

	app.Get("/another-path", func(ctx fiber.Ctx) error {
		require.NoError(t, ctx.SendString("OK"))
		return nil
	})

	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodGet, "http://localhost/another-path"), fiber.StatusOK, "OK")
}

func Test_EnvVarHandlerDefaultConfig(t *testing.T) {
	t.Setenv("testEnvKey", "testEnvVal")

	app := fiber.New()
	app.Use("/envvars", New())

	_, exists := envvarTestVars(t, app, "http://localhost/envvars").Vars["testEnvKey"]
	require.False(t, exists)
}

func Test_EnvVarHandlerMethod(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use("/envvars", New())

	resp := apptest.Expect(t, app, apptest.Req(fiber.MethodPost, "http://localhost/envvars"), fiber.StatusMethodNotAllowed)
	require.Equal(t, hAllow, resp.Header.Get(fiber.HeaderAllow))
}

func Test_EnvVarHandlerHead(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use("/envvars", New())

	resp := apptest.Expect(t, app, apptest.Req(fiber.MethodHead, "http://localhost/envvars"), fiber.StatusOK)
	require.Empty(t, apptest.Body(t, resp))
}

func Test_EnvVarHandlerSpecialValue(t *testing.T) {
	testEnvKey := "testEnvKey"
	fakeBase64 := "testBase64:TQ=="
	t.Setenv(testEnvKey, fakeBase64)

	app := fiber.New()
	app.Use("/envvars/export", New(Config{ExportVars: map[string]string{testEnvKey: ""}}))
	app.Use("/envvars", New())

	_, exists := envvarTestVars(t, app, "http://localhost/envvars").Vars[testEnvKey]
	require.False(t, exists)
	require.Equal(t, fakeBase64, envvarTestVars(t, app, "http://localhost/envvars/export").Vars[testEnvKey])
}
