package minify

import (
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

func Test_Minify_cssMinify(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		css          []byte
		expectedData string
	}{
		{
			name:         "Minify css",
			css:          filedata.css,
			expectedData: expectedData.CSSMinify,
		},
	}
	// loop through test cases
	for _, tc := range testCases {
		// run the test case with the given name
		t.Run(tc.name, func(t *testing.T) {
			// minify css
			minifiedCss := cssMinify(tc.css)
			// check if minified html equals expectedData
			utils.AssertEqual(t, []byte(tc.expectedData), minifiedCss)
		})
	}
}
