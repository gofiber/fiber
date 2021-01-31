package utils

// JSONMarshal is the standard definition of representing a Go structure in
// json format
type JSONMarshal func(interface{}) ([]byte, error)
