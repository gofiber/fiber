// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"bytes"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"text/tabwriter"
)

// AssertEqual checks if values are equal
func AssertEqual(tb testing.TB, expected, actual interface{}, description ...string) { //nolint:thelper // TODO: Verify if tb can be nil
	if tb != nil {
		tb.Helper()
	}

	if reflect.DeepEqual(expected, actual) {
		return
	}

	aType := "<nil>"
	bType := "<nil>"

	if expected != nil {
		aType = reflect.TypeOf(expected).String()
	}
	if actual != nil {
		bType = reflect.TypeOf(actual).String()
	}

	testName := "AssertEqual"
	if tb != nil {
		testName = tb.Name()
	}

	_, file, line, _ := runtime.Caller(1)

	var buf bytes.Buffer
	const pad = 5
	w := tabwriter.NewWriter(&buf, 0, 0, pad, ' ', 0)
	_, _ = fmt.Fprintf(w, "\nTest:\t%s", testName)
	_, _ = fmt.Fprintf(w, "\nTrace:\t%s:%d", filepath.Base(file), line)
	if len(description) > 0 {
		_, _ = fmt.Fprintf(w, "\nDescription:\t%s", description[0])
	}
	_, _ = fmt.Fprintf(w, "\nExpect:\t%v\t(%s)", expected, aType)
	_, _ = fmt.Fprintf(w, "\nResult:\t%v\t(%s)", actual, bType)

	var result string
	if err := w.Flush(); err != nil {
		result = err.Error()
	} else {
		result = buf.String()
	}

	if tb != nil {
		tb.Fatal(result)
	} else {
		log.Fatal(result) //nolint:revive // tb might be nil, so we need a fallback
	}
}
