//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package envvar

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

func TestEnvVarStructWithExportVarsExcludeVars(t *testing.T) {
	err := os.Setenv("testKey", "testEnvValue")
	utils.AssertEqual(t, nil, err)
	err = os.Setenv("anotherEnvKey", "anotherEnvVal")
	utils.AssertEqual(t, nil, err)
	err = os.Setenv("excludeKey", "excludeEnvValue")
	utils.AssertEqual(t, nil, err)
	defer func() {
		err := os.Unsetenv("testKey")
		utils.AssertEqual(t, nil, err)
		err = os.Unsetenv("anotherEnvKey")
		utils.AssertEqual(t, nil, err)
		err = os.Unsetenv("excludeKey")
		utils.AssertEqual(t, nil, err)
	}()

	vars := newEnvVar(Config{
		ExportVars:  map[string]string{"testKey": "", "testDefaultKey": "testDefaultVal"},
		ExcludeVars: map[string]string{"excludeKey": ""},
	})

	utils.AssertEqual(t, vars.Vars["testKey"], "testEnvValue")
	utils.AssertEqual(t, vars.Vars["testDefaultKey"], "testDefaultVal")
	utils.AssertEqual(t, vars.Vars["excludeKey"], "")
	utils.AssertEqual(t, vars.Vars["anotherEnvKey"], "")
}

func TestEnvVarHandler(t *testing.T) {
	err := os.Setenv("testKey", "testVal")
	utils.AssertEqual(t, nil, err)
	defer func() {
		err := os.Unsetenv("testKey")
		utils.AssertEqual(t, nil, err)
	}()

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

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars", nil)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, expectedEnvVarResponse, respBody)
}

func TestEnvVarHandlerNotMatched(t *testing.T) {
	app := fiber.New()
	app.Use("/envvars", New(Config{
		ExportVars: map[string]string{"testKey": ""},
	}))

	app.Get("/another-path", func(ctx *fiber.Ctx) error {
		utils.AssertEqual(t, nil, ctx.SendString("OK"))
		return nil
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/another-path", nil)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, []byte("OK"), respBody)
}

func TestEnvVarHandlerDefaultConfig(t *testing.T) {
	err := os.Setenv("testEnvKey", "testEnvVal")
	utils.AssertEqual(t, nil, err)
	defer func() {
		err := os.Unsetenv("testEnvKey")
		utils.AssertEqual(t, nil, err)
	}()

	app := fiber.New()
	app.Use("/envvars", New())

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars", nil)
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
	app := fiber.New()
	app.Use("/envvars", New())

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "http://localhost/envvars", nil)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusMethodNotAllowed, resp.StatusCode)
}

func TestEnvVarHandlerSpecialValue(t *testing.T) {
	testEnvKey := "testEnvKey"
	fakeBase64 := "testBase64:TQ=="
	err := os.Setenv(testEnvKey, fakeBase64)
	utils.AssertEqual(t, nil, err)
	defer func() {
		err := os.Unsetenv(testEnvKey)
		utils.AssertEqual(t, nil, err)
	}()

	app := fiber.New()
	app.Use("/envvars", New())
	app.Use("/envvars/export", New(Config{ExportVars: map[string]string{testEnvKey: ""}}))

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars", nil)
	utils.AssertEqual(t, nil, err)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	var envVars EnvVar
	utils.AssertEqual(t, nil, json.Unmarshal(respBody, &envVars))
	val := envVars.Vars[testEnvKey]
	utils.AssertEqual(t, fakeBase64, val)

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://localhost/envvars/export", nil)
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
