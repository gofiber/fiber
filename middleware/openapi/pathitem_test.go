package openapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_OpenAPISpec_MarshalsPathExtrasAndExtensions(t *testing.T) {
	t.Parallel()
	spec := openAPISpec{
		OpenAPI: "3.2.0",
		Info:    openAPIInfo{Title: "t", Version: "1"},
		Paths:   map[string]map[string]operation{"/a": {"get": {Summary: "get a"}}},
		pathExtras: map[string]pathExtra{
			"/a": {summary: "A", description: "all of A", parameters: []parameter{{Name: "id", In: "query"}}, additional: map[string]operation{"PURGE": {Summary: "purge"}}},
			"/b": {summary: "only extras"},
		},
		extensions: map[string]any{"x-team": "api", "openapi": "ignored: a generated key wins"},
	}

	data, err := json.Marshal(spec)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, "api", got["x-team"])
	require.Equal(t, "3.2.0", got["openapi"])

	paths, ok := got["paths"].(map[string]any)
	require.True(t, ok)
	a, ok := paths["/a"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "A", a["summary"])
	require.Equal(t, "all of A", a["description"])
	require.Contains(t, a, "get")
	require.Contains(t, a, "parameters")
	require.Contains(t, a["additionalOperations"], "PURGE")
	require.Equal(t, map[string]any{"summary": "only extras"}, paths["/b"])
}

func Test_OpenAPISpec_MarshalsWithoutExtras(t *testing.T) {
	t.Parallel()
	spec := openAPISpec{OpenAPI: "3.1.0", Info: openAPIInfo{Title: "t", Version: "1"}, Paths: map[string]map[string]operation{}}
	data, err := json.Marshal(spec)
	require.NoError(t, err)
	require.JSONEq(t, `{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":{}}`, string(data))
}
