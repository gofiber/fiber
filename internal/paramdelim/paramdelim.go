// Package paramdelim holds the byte set that ends a ":name" placeholder in a
// path. The route parser and the HTTP client both need the same grammar; keep
// it here so they cannot drift (#4635).
package paramdelim

// PathEndChars are the bytes that terminate a :name placeholder in a URL path
// or route pattern: '/', '-', '.', ':', '\\', and '?'.
func PathEndChars() [256]bool {
	var s [256]bool
	s['/'] = true
	s['-'] = true
	s['.'] = true
	s[':'] = true
	s['\\'] = true
	s['?'] = true
	return s
}
