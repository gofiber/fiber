package utils

// XMLMarshal returns the XML encoding of v.
type XMLMarshal func(v any) ([]byte, error)

// XMLUnmarshal parses the XML-encoded data and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// Unmarshal returns an InvalidUnmarshalError.
type XMLUnmarshal func(data []byte, v any) error
