package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type EmbeddedStruct struct {
	Tags []string `query:"tags"`
}

type TestRequest struct {
	Name string `query:"name"`
	EmbeddedStruct
}

func Test_QueryBinding_EmbeddedStructSlice(t *testing.T) {
	b := &QueryBinding{
		EnableSplitting: true,
	}

	// Create fasthttp request with query parameters
	req := &fasthttp.Request{}
	req.URI().SetQueryString("name=john&tags=tag1,tag2,tag3")

	var result TestRequest
	err := b.Bind(req, &result)

	require.NoError(t, err)
	require.Equal(t, "john", result.Name)
	require.Equal(t, []string{"tag1", "tag2", "tag3"}, result.Tags)
}

func Test_QueryBinding_EmbeddedStructSlice_MultipleValues(t *testing.T) {
	b := &QueryBinding{
		EnableSplitting: true,
	}

	// Create fasthttp request with multiple query parameters
	req := &fasthttp.Request{}
	req.URI().SetQueryString("name=jane&tags=sport&tags=music,movies&tags=books")

	var result TestRequest
	err := b.Bind(req, &result)

	require.NoError(t, err)
	require.Equal(t, "jane", result.Name)
	// Should handle both comma-separated and multiple parameter instances
	require.Equal(t, []string{"sport", "music", "movies", "books"}, result.Tags)
}

// Test case that reproduces the exact issue from #2859
func Test_QueryBinding_Issue2859_Reproduction(t *testing.T) {
	// This reproduces the exact issue described in Fiber #2859
	// where embedded struct fields with slices don't get split correctly
	type EmbeddedWithSlice struct {
		Items []string `query:"items"`
	}

	type RequestStruct struct {
		ID string `query:"id"`
		EmbeddedWithSlice
	}

	b := &QueryBinding{
		EnableSplitting: true,
	}

	// Test the exact case mentioned in the issue
	req := &fasthttp.Request{}
	req.URI().SetQueryString("id=123&items=item1,item2,item3")

	var result RequestStruct
	err := b.Bind(req, &result)

	require.NoError(t, err)
	require.Equal(t, "123", result.ID)
	// Before the fix, this would fail because embedded struct slice fields
	// wouldn't be properly detected for comma splitting
	require.Equal(t, []string{"item1", "item2", "item3"}, result.Items)
}

// Test with nested embedded structs
func Test_QueryBinding_NestedEmbeddedStructs(t *testing.T) {
	type Level2 struct {
		Values []string `query:"values"`
	}

	type Level1 struct {
		Tags []string `query:"tags"`
		Level2
	}

	type MainStruct struct {
		Name string `query:"name"`
		Level1
	}

	b := &QueryBinding{
		EnableSplitting: true,
	}

	req := &fasthttp.Request{}
	req.URI().SetQueryString("name=test&tags=tag1,tag2&values=val1,val2,val3")

	var result MainStruct
	err := b.Bind(req, &result)

	require.NoError(t, err)
	require.Equal(t, "test", result.Name)
	require.Equal(t, []string{"tag1", "tag2"}, result.Tags)
	require.Equal(t, []string{"val1", "val2", "val3"}, result.Values)
}
