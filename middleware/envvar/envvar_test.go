package envvar

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"io"
	"net/http"
	"testing"
)

func TestEnvVarStructWithExportVarsExcludeVars(t *testing.T) {
	t.Setenv("testKey", "testEnvValue")
	t.Setenv("anotherEnvKey", "anotherEnvVal")
	t.Setenv("excludeKey", "excludeEnvValue")

	vars := newEnvVar(Config{
		ExportVars:  map[string]string{"testKey": "", "testDefaultKey": "testDefaultVal"},
		ExcludeVars: map[string]string{"excludeKey": ""}})

	utils.AssertEqual(t, vars.get("testKey"), "testEnvValue")
	utils.AssertEqual(t, vars.get("testDefaultKey"), "testDefaultVal")
	utils.AssertEqual(t, vars.get("excludeKey"), "")
	utils.AssertEqual(t, vars.get("anotherEnvKey"), "")
}

func TestEnvVarHandler(t *testing.T) {
	t.Setenv("testKey", "testVal")

	expectedEnvVarResponse, _ := json.Marshal(
		struct {
			Vars map[string]string `json:"vars"`
		}{
			map[string]string{"testKey": "testVal"},
		})

	app := fiber.New()
	app.Use(New(Config{
		Path:       "/envvars",
		ExportVars: map[string]string{"testKey": ""}}))

	req, _ := http.NewRequest("GET", "http://localhost/envvars", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, expectedEnvVarResponse, respBody)
}

func TestEnvVarHandlerNotMatched(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Path:       "/envvars",
		ExportVars: map[string]string{"testKey": ""}}))

	app.Get("/another-path", func(ctx *fiber.Ctx) error {
		ctx.SendString("OK")
		return nil
	})

	req, _ := http.NewRequest("GET", "http://localhost/another-path", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, []byte("OK"), respBody)
}

func TestEnvVarHandlerDefaultConfig(t *testing.T) {
	t.Setenv("testEnvKey", "testEnvVal")

	app := fiber.New()
	app.Use(New())

	req, _ := http.NewRequest("GET", "http://localhost/envvars", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	respBody, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	var envVars EnvVar
	utils.AssertEqual(t, nil, json.Unmarshal(respBody, &envVars))
	val := envVars.get("testEnvKey")
	utils.AssertEqual(t, "testEnvVal", val)
}
