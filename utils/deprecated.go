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

// DEPRECATED, please use EqualFoldBytes
func EqualsFold(b, s []byte) (equals bool) {
	return EqualFoldBytes(b, s)
}

// DEPRECATED, Please use CopyString instead
func SafeString(s string) string {
	return CopyString(s)
}

// DEPRECATED, Please use CopyBytes instead
func SafeBytes(b []byte) []byte {
	return CopyBytes(b)
}
