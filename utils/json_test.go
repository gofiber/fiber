package utils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type sampleStructure struct {
	ImportantString string `json:"important_string"`
}

func Test_GolangJSONEncoder(t *testing.T) {
	t.Parallel()

	var (
		ss = &sampleStructure{
			ImportantString: "Hello World",
		}
		importantString             = `{"important_string":"Hello World"}`
		jsonEncoder     JSONMarshal = json.Marshal
	)

	raw, err := jsonEncoder(ss)
	require.NoError(t, err)

	require.Equal(t, string(raw), importantString)
}

func Test_DefaultJSONEncoder(t *testing.T) {
	t.Parallel()

	var (
		ss = &sampleStructure{
			ImportantString: "Hello World",
		}
		importantString             = `{"important_string":"Hello World"}`
		jsonEncoder     JSONMarshal = json.Marshal
	)

	raw, err := jsonEncoder(ss)
	require.NoError(t, err)

	require.Equal(t, string(raw), importantString)
}

func Test_DefaultJSONDecoder(t *testing.T) {
	t.Parallel()

	var (
		ss              sampleStructure
		importantString               = []byte(`{"important_string":"Hello World"}`)
		jsonDecoder     JSONUnmarshal = json.Unmarshal
	)

	err := jsonDecoder(importantString, &ss)
	require.NoError(t, err)
	require.Equal(t, "Hello World", ss.ImportantString)
}
