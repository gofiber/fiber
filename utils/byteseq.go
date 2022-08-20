package utils

type byteSeq interface {
	~string | ~[]byte
}

// EqualFold tests ascii strings or bytes for equality case-insensitively
func EqualFold[S byteSeq](b, s S) bool {
	if len(b) != len(s) {
		return false
	}
	for i := len(b) - 1; i >= 0; i-- {
		if toUpperTable[b[i]] != toUpperTable[s[i]] {
			return false
		}
	}
	return true
}
