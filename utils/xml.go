package utils

// XMLMarshal returns the XML encoding of v.
type XMLMarshal func(v any) ([]byte, error)

// XMLUnmarshal parses the XML-encoded data and stores the result in
// the value pointed to by v, which must be an arbitrary struct,
// slice, or string. Well-formed data that does not fit into v is
// discarded.
type XMLUnmarshal func([]byte, any) error
