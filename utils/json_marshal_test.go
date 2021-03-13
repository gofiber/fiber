package utils

import (
	goJson "encoding/json"
	"testing"

	internalJson "github.com/gofiber/fiber/v2/internal/encoding/json"
)

func TestInternalJSONEncoder(t *testing.T) {
	type SampleStructure struct {
		ImportantString string `json:"important_string"`
	}

	var (
		sampleStructure = &SampleStructure{
			ImportantString: "Hello World",
		}
		importantString = `{"important_string":"Hello World"}`

		jsonEncoder JSONMarshal = internalJson.Marshal
	)

	raw, err := jsonEncoder(sampleStructure)
	AssertEqual(t, err, nil)

	AssertEqual(t, string(raw), importantString)
}

func TestDefaultJSONEncoder(t *testing.T) {
	type SampleStructure struct {
		ImportantString string `json:"important_string"`
	}

	var (
		sampleStructure = &SampleStructure{
			ImportantString: "Hello World",
		}
		importantString = `{"important_string":"Hello World"}`

		jsonEncoder JSONMarshal = goJson.Marshal
	)

	raw, err := jsonEncoder(sampleStructure)
	AssertEqual(t, err, nil)

	AssertEqual(t, string(raw), importantString)
}
