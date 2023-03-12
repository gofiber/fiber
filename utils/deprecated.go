package utils

// Deprecated: Please use UnsafeString instead
func GetString(b []byte) string {
	return UnsafeString(b)
}

// Deprecated: Please use UnsafeBytes instead
func GetBytes(s string) []byte {
	return UnsafeBytes(s)
}

// Deprecated: Please use CopyString instead
func ImmutableString(s string) string {
	return CopyString(s)
}
