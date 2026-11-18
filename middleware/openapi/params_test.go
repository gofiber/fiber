package openapi

import (
	"reflect"
	"testing"

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

func Test_OpenAPI_LocationOnlyFieldsAreFoundInNestedStructs(t *testing.T) {
	t.Parallel()
	require.True(t, hasLocationOnlyFields(reflect.TypeFor[nestedLocationOuter](), "json"))
	require.False(t, hasLocationOnlyFields(reflect.TypeFor[nestedLocationHolder](), "json"), "a named struct is its own component")
	require.False(t, hasLocationOnlyFields(reflect.TypeFor[userResponse](), "json"))
}
