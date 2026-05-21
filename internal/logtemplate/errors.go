package logtemplate

import (
	"errors"
	"strconv"
)

// ErrUnknownTag indicates that the template references a tag that has no
// registered renderer. The format may be a bare ${tag} or a parametric
// ${tag:param}; in both cases the unmatched name is reported via
// UnknownTagError so callers can extract it programmatically.
var ErrUnknownTag = errors.New("logtemplate: unknown tag")

// UnknownTagError is the typed error returned when a template references an
// unknown tag. Tag is the offending tag including any parametric suffix
// (without the surrounding "${" / "}"). Param is the parameter portion when
// the tag was parametric, or the empty string for bare tags. Hint is an
// optional human-readable suggestion — currently set when a bare tag was
// referenced but a parametric base of the same name is registered.
type UnknownTagError struct {
	Tag   string
	Param string
	Hint  string
}

func (e *UnknownTagError) Error() string {
	msg := ErrUnknownTag.Error() + ": " + strconv.Quote(e.Tag)
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

func (*UnknownTagError) Unwrap() error {
	return ErrUnknownTag
}
