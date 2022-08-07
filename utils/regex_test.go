package utils

import (
	"regexp"
	"testing"
)

func Test_Golang_Regex_Match(t *testing.T) {
	var matcher RegexMatch = regexp.MatchString

	matched, err := matcher(`^(\+\d{1,2}\s)?\(?\d{3}\)?[\s.-]\d{3}[\s.-]\d{4}$`, `(555) 555-5555`)
	AssertEqual(t, nil, err)
	AssertEqual(t, true, matched)
}
