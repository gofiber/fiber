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
func AssertEqual(t testing.TB, expected, actual interface{}, description ...string) {
	if reflect.DeepEqual(expected, actual) {
		return
	}

	var aType = "<nil>"
	var bType = "<nil>"

	if expected != nil {
		aType = fmt.Sprintf("%s", reflect.TypeOf(expected))
	}
	if actual != nil {
		bType = fmt.Sprintf("%s", reflect.TypeOf(actual))
	}

	testName := "AssertEqual"
	if t != nil {
		testName = t.Name()
	}

	_, file, line, _ := runtime.Caller(1)

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 5, ' ', 0)
	fmt.Fprintf(w, "\nTest:\t%s", testName)
	fmt.Fprintf(w, "\nTrace:\t%s:%d", filepath.Base(file), line)
	if len(description) > 0 {
		fmt.Fprintf(w, "\nDescription:\t%s", description[0])
	}
	fmt.Fprintf(w, "\nExpect:\t%v\t(%s)", expected, aType)
	fmt.Fprintf(w, "\nResult:\t%v\t(%s)", actual, bType)

	result := ""
	if err := w.Flush(); err != nil {
		result = err.Error()
	} else {
		result = buf.String()
	}

	if t != nil {
		t.Fatal(result)
	} else {
		log.Fatal(result)
	}
}
