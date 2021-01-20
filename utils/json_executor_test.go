package utils

import (
	"testing"
)

func TestDefaultJSONExecutor(t *testing.T) {
	type SampleStructure struct {
		ImportantString string `json:"important_string"`
	}

	var (
		sampleStructure = &SampleStructure{
			ImportantString: "Hello World",
		}
		importantString = `{"important_string":"Hello World"}`
	)

	jsonExecutor := DefaultJSONExecutor{}

	raw, err := jsonExecutor.Marshal(sampleStructure)
	AssertEqual(t, err, nil)

	AssertEqual(t, string(raw), importantString)
}
