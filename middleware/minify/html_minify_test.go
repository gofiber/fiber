package minify

import (
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

func Test_Minify_htmlMinify(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		html         []byte
		opt          *Options
		expectedData string
	}{
		{
			name: "Minify html",
			html: filedata.html,
			opt: &Options{
				MinifyScripts: false,
				MinifyStyles:  false,
			},
			expectedData: expectedData.HtmlMinify,
		},
		{
			name: "Minify html with css and js",
			html: filedata.htmlWithCssAndJs,
			opt: &Options{
				MinifyScripts: true,
				MinifyStyles:  true,
			},
			expectedData: expectedData.HTMLConfigOptionsCssJSTrue,
		},
		{
			name: "Minify html except js, css",
			html: filedata.htmlWithCssAndJs,
			opt: &Options{
				MinifyScripts: false,
				MinifyStyles:  false,
			},
			expectedData: expectedData.HTMLConfigOptionsCssJSFalse,
		},
		{
			name: "Minify html except js",
			html: filedata.htmlWithCssAndJs,
			opt: &Options{
				MinifyScripts: false,
				MinifyStyles:  true,
			},
			expectedData: expectedData.HTMLConfigOptionsJSFalse,
		},
		{
			name: "Minify html except css",
			html: filedata.htmlWithCssAndJs,
			opt: &Options{
				MinifyScripts: true,
				MinifyStyles:  false,
			},
			expectedData: expectedData.HTMLConfigOptionsStylesFalse,
		},
	}
	// loop through test cases
	for _, tc := range testCases {
		// run the test case with the given name
		t.Run(tc.name, func(t *testing.T) {
			// minify html
			minifiedHTML, err := htmlMinify(tc.html, tc.opt)
			// check if there is an error
			utils.AssertEqual(t, nil, err)

			// check if minified html equals expectedData
			utils.AssertEqual(t, []byte(tc.expectedData), minifiedHTML)
		})
	}
}
