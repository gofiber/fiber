package openapi

import (
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type nestedLocationNamed struct {
	Page int `query:"page"`
}

type nestedLocationOuter struct {
	Items []struct {
		Name string `json:"name"`
		Page int    `query:"page"`
	} `json:"items"`
}

type nestedLocationHolder struct {
	Items []nestedLocationNamed `json:"items"`
}

type hiddenLocation struct {
	Name   string `json:"name"`
	Secret string `json:"-" query:"secret"`
	hidden string `query:"hidden"` //nolint:unused,revive // an unexported field is not in the schema
}

type customLocation struct {
	Page int `query:"page"`
}

func (customLocation) OpenAPISchema() map[string]any { return map[string]any{"type": "string"} }

type customHolder struct {
	Inner customLocation `json:"inner"`
}

func Test_OpenAPI_LocationOnlyFieldsAreFoundInNestedStructs(t *testing.T) {
	t.Parallel()
	reg := newSchemaRegistry(&Config{}, dialect{})
	require.True(t, reg.hasLocationOnlyFields(reflect.TypeFor[nestedLocationOuter]()))
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[nestedLocationHolder]()), "a named struct is its own component")
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[userResponse]()))
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[hiddenLocation]()), "a field the schema leaves out does not count")
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[customHolder]()))
}

func Test_OpenAPI_RequestComponentsAreEmittedOnlyWhenTheyDiffer(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Post("/a", listUsers).Accepts(nestedLocationHolder{}).Returns(fiber.StatusOK, nestedLocationHolder{})
	app.Post("/b", listUsers).Accepts(hiddenLocation{}).Returns(fiber.StatusOK, hiddenLocation{})
	app.Post("/c", listUsers).Accepts(userRequest{}).Returns(fiber.StatusOK, userRequest{})
	schemas := modelSchemas(t, modelSpec(t, app))
	require.Contains(t, schemas, "nestedLocationHolder")
	require.NotContains(t, schemas, "nestedLocationHolder_request")
	require.NotContains(t, schemas, "hiddenLocation_request")
	require.Contains(t, schemas, "userRequest_request", "a type with a query field differs as a body")
}
