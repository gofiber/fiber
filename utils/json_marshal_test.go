package utils

import (
	"encoding/json"
	"testing"
)

func TestDefaultJSONEncoder(t *testing.T) {
	type SampleStructure struct {
		ImportantString string `json:"important_string"`
	}

	var (
		sampleStructure = &SampleStructure{
			ImportantString: "Hello World",
		}
		importantString = `{"important_string":"Hello World"}`

		jsonEncoder JSONMarshal = json.Marshal
	)

	raw, err := jsonEncoder(sampleStructure)
	AssertEqual(t, err, nil)

	AssertEqual(t, string(raw), importantString)
}
