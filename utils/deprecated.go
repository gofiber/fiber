package utils

// #nosec G103
// DEPRECATED, Please use UnsafeString instead
func GetString(b []byte) string {
	return UnsafeString(b)
}

// #nosec G103
// DEPRECATED, Please use UnsafeBytes instead
func GetBytes(s string) []byte {
	return UnsafeBytes(s)
}

// DEPRECATED, Please use CopyString instead
func ImmutableString(s string) string {
	return CopyString(s)
}
