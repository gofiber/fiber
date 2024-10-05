package utils

// CBORMarshal returns the CBOR encoding of v.
type CBORMarshal func(v any) ([]byte, error)

// CBORUnmarshal parses the CBOR-encoded data and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// Unmarshal returns an InvalidUnmarshalError.
type CBORUnmarshal func(data []byte, v any) error
