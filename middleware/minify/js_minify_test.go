package minify

import (
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

func Test_Minify_jsMinify(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		js           []byte
		expectedData string
	}{
		{
			name:         "Minify js",
			js:           filedata.js,
			expectedData: expectedData.JsMinify,
		},
	}
	// loop through test cases
	for _, tc := range testCases {
		// run the test case with the given name
		t.Run(tc.name, func(t *testing.T) {
			// minify css
			minifiedJs, err := jsMinify(tc.js)
			utils.AssertEqual(t, nil, err)
			// check if minified html equals expectedData
			utils.AssertEqual(t, []byte(tc.expectedData), minifiedJs)
		})
	}
}
