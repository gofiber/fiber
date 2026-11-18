package openapi

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type nestedLocationInner struct {
	Name string `json:"name"`
	Page int    `query:"page"`
}

type nestedLocationOuter struct {
	Items []nestedLocationInner `json:"items"`
}

func Test_OpenAPI_LocationOnlyFieldsAreFoundInNestedStructs(t *testing.T) {
	t.Parallel()
	require.True(t, hasLocationOnlyFields(reflect.TypeFor[nestedLocationOuter](), "json"))
	require.False(t, hasLocationOnlyFields(reflect.TypeFor[userResponse](), "json"))
}
