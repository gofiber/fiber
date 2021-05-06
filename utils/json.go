package utils

// JSONMarshal returns the JSON encoding of v.
type JSONMarshal func(v interface{}) ([]byte, error)

// JSONUnmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// Unmarshal returns an InvalidUnmarshalError.
type JSONUnmarshal func(data []byte, v interface{}) error
