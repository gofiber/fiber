//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package envvar

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

//nolint:paralleltest // Test is using t.Setenv which can't be run in parallel
func TestEnvVarStructWithExportVarsExcludeVars(t *testing.T) {
	t.Setenv("testKey", "testEnvValue")
	t.Setenv("anotherEnvKey", "anotherEnvVal")
	t.Setenv("excludeKey", "excludeEnvValue")

	vars := newEnvVar(Config{
		ExportVars:  map[string]string{"testKey": "", "testDefaultKey": "testDefaultVal"},
		ExcludeVars: map[string]string{"excludeKey": ""},
	})

	utils.AssertEqual(t, vars.Vars["testKey"], "testEnvValue")
	utils.AssertEqual(t, vars.Vars["testDefaultKey"], "testDefaultVal")
	utils.AssertEqual(t, vars.Vars["excludeKey"], "")
	utils.AssertEqual(t, vars.Vars["anotherEnvKey"], "")
}

//nolint:paralleltest // Test is using t.Setenv which can't be run in parallel
func TestEnvVarHandler(t *testing.T) {
	t.Setenv("testKey", "testVal")

	expectedEnvVarResponse, err := json.Marshal(
		struct {
			Vars map[string]string `json:"vars"`
		}{
			map[string]string{"testKey": "testVal"},
		})
	utils.AssertEqual(t, nil, err)

	app := fiber.New()
	app.Use("/envvars", New(Config{
		ExportVars: map[string]string{"testKey": ""},
	}))

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars", http.NoBody)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, expectedEnvVarResponse, respBody)
}

func TestEnvVarHandlerNotMatched(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use("/envvars", New(Config{
		ExportVars: map[string]string{"testKey": ""},
	}))

	app.Get("/another-path", func(ctx *fiber.Ctx) error {
		utils.AssertEqual(t, nil, ctx.SendString("OK"))
		return nil
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/another-path", http.NoBody)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, []byte("OK"), respBody)
}

//nolint:paralleltest // Test is using t.Setenv which can't be run in parallel
func TestEnvVarHandlerDefaultConfig(t *testing.T) {
	t.Setenv("testEnvKey", "testEnvVal")

	app := fiber.New()
	app.Use("/envvars", New())

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars", http.NoBody)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	var envVars EnvVar
	utils.AssertEqual(t, nil, json.Unmarshal(respBody, &envVars))
	val := envVars.Vars["testEnvKey"]
	utils.AssertEqual(t, "testEnvVal", val)
}

func TestEnvVarHandlerMethod(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use("/envvars", New())

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "http://localhost/envvars", http.NoBody)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusMethodNotAllowed, resp.StatusCode)
}

//nolint:paralleltest // Test is using t.Setenv which can't be run in parallel
func TestEnvVarHandlerSpecialValue(t *testing.T) {
	const (
		testEnvKey = "testEnvKey"
		fakeBase64 = "testBase64:TQ=="
	)

	t.Setenv(testEnvKey, fakeBase64)

	app := fiber.New()
	app.Use("/envvars", New())
	app.Use("/envvars/export", New(Config{ExportVars: map[string]string{testEnvKey: ""}}))

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars", http.NoBody)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	var envVars EnvVar
	utils.AssertEqual(t, nil, json.Unmarshal(respBody, &envVars))
	val := envVars.Vars[testEnvKey]
	utils.AssertEqual(t, fakeBase64, val)

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars/export", http.NoBody)
	utils.AssertEqual(t, nil, err)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	var envVarsExport EnvVar
	utils.AssertEqual(t, nil, json.Unmarshal(respBody, &envVarsExport))
	val = envVarsExport.Vars[testEnvKey]
	utils.AssertEqual(t, fakeBase64, val)
}
