package utils

// XMLMarshal returns the XML encoding of v.
type XMLMarshal func(v interface{}) ([]byte, error)
