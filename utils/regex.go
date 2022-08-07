package utils

// RegexMatch returns the matching s by regex pattern.
type RegexMatch func(pattern string, s string) (matched bool, err error)
