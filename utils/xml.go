package utils

// XMLMarshal returns the XML encoding of v.
type XMLMarshal func(v any) ([]byte, error)
