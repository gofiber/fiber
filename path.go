// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was inspired by https://github.com/ucarion/urlpath
// 💖 Maintained and modified for Fiber by @renewerner87

package fiber

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	utilsbytes "github.com/gofiber/utils/v2/bytes"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// routeParser holds the path segments and param names
type routeParser struct {
	segs          []*routeSegment // the parsed segments of the route
	params        []string        // that parameter names the parsed route
	wildCardCount int             // number of wildcard parameters, used internally to give the wildcard parameter its number
	plusCount     int             // number of plus parameters, used internally to give the plus parameter its number
}

var routerParserPool = &sync.Pool{
	New: func() any {
		return &routeParser{}
	},
}

// routeSegment holds the segment metadata
type routeSegment struct {
	// const information
	Const       string        // constant part of the route
	ParamName   string        // name of the parameter for access to it, for wildcards and plus parameters access iterators starting with 1 are added
	ComparePart string        // search part to find the end of the parameter
	Constraints []*Constraint // Constraint type if segment is a parameter, if not it will be set to noConstraint by default
	PartCount   int           // how often is the search part contained in the non-param segments? -> necessary for greedy search
	Length      int           // length of the parameter for segment, when its 0 then the length is undetermined
	// future TODO: add support for optional groups "/abc(/def)?"
	// parameter information
	IsParam    bool // Truth value that indicates whether it is a parameter or a constant part
	IsGreedy   bool // indicates whether the parameter is greedy or not, is used with wildcard and plus
	IsOptional bool // indicates whether the parameter is optional or not
	// common information
	IsLast           bool // shows if the segment is the last one for the route
	HasOptionalSlash bool // segment has the possibility of an optional slash
}

// different special routing signs
const (
	wildcardParam                byte = '*'  // indicates an optional greedy parameter
	plusParam                    byte = '+'  // indicates a required greedy parameter
	optionalParam                byte = '?'  // concludes a parameter by name and makes it optional
	paramStarterChar             byte = ':'  // start character for a parameter with name
	slashDelimiter               byte = '/'  // separator for the route, unlike the other delimiters this character at the end can be optional
	escapeChar                   byte = '\\' // escape character
	paramConstraintStart         byte = '<'  // start of type constraint for a parameter
	paramConstraintEnd           byte = '>'  // end of type constraint for a parameter
	paramConstraintSeparator     byte = ';'  // separator of type constraints for a parameter
	paramConstraintDataStart     byte = '('  // start of data of type constraint for a parameter
	paramConstraintDataEnd       byte = ')'  // end of data of type constraint for a parameter
	paramConstraintDataSeparator byte = ','  // separator of data of type constraint for a parameter
)

// TypeConstraint parameter constraint types.
//
// Deprecated: Use the ConstraintHandler interface instead. Retained for
// backward compatibility with external code that reads or compares IDs.
type TypeConstraint uint16

// Constraint describes the validation rules that apply to a dynamic route
// segment when matching incoming requests.
// See constraint.go for the ConstraintHandler and ConstraintAnalyzer interfaces.
type Constraint struct {
	handler   ConstraintHandler
	typedData []any

	// RegexCompiler is populated when the constraint is a regex and the
	// default regexp.Compile engine is used.
	//
	// Deprecated: Use the ConstraintHandler interface instead. Retained for
	// backward compatibility with external code that reads this field.
	RegexCompiler *regexp.Regexp

	// Name is the raw constraint name as it appeared in the route pattern
	// (e.g. "minlen", not the canonical "minLen").
	Name string

	// Data holds the raw parsed constraint arguments from the route pattern.
	Data []string

	// ID identifies the built-in constraint kind.
	//
	// Deprecated: Use the ConstraintHandler interface instead. Retained for
	// backward compatibility with external code that reads or compares IDs.
	ID TypeConstraint
}

// Deprecated: Use the ConstraintHandler interface instead.
const (
	noConstraint TypeConstraint = 1 << iota
	intConstraint
	boolConstraint
	floatConstraint
	alphaConstraint
	datetimeConstraint
	guidConstraint
	minLenConstraint
	maxLenConstraint
	lenConstraint
	betweenLenConstraint
	minConstraint
	maxConstraint
	rangeConstraint
	regexConstraint
)

// constraintNameToID maps canonical constraint names to their TypeConstraint ID.
//
// Deprecated: retained for populating Constraint.ID for backward compatibility.
var constraintNameToID = map[string]TypeConstraint{
	ConstraintInt:        intConstraint,
	ConstraintBool:       boolConstraint,
	ConstraintFloat:      floatConstraint,
	ConstraintAlpha:      alphaConstraint,
	ConstraintDatetime:   datetimeConstraint,
	ConstraintGUID:       guidConstraint,
	ConstraintMinLen:     minLenConstraint,
	ConstraintMaxLen:     maxLenConstraint,
	ConstraintLen:        lenConstraint,
	ConstraintBetweenLen: betweenLenConstraint,
	ConstraintMin:        minConstraint,
	ConstraintMax:        maxConstraint,
	ConstraintRange:      rangeConstraint,
	ConstraintRegex:      regexConstraint,
}

// list of possible parameter and segment delimiter
var (
	// slash has a special role, unlike the other parameters it must not be interpreted as a parameter
	routeDelimiter = []byte{slashDelimiter, '-', '.'}
	// list of chars for the parameter recognizing
	parameterStartChars = [256]bool{
		wildcardParam:    true,
		plusParam:        true,
		paramStarterChar: true,
	}
	// list of chars of delimiters and the starting parameter name char
	parameterDelimiterChars = append([]byte{paramStarterChar, escapeChar}, routeDelimiter...)
	// list of chars to find the end of a parameter
	parameterEndChars = [256]bool{
		optionalParam:    true,
		paramStarterChar: true,
		escapeChar:       true,
		slashDelimiter:   true,
		'-':              true,
		'.':              true,
	}
)

// RoutePatternMatch reports whether path matches the provided Fiber route pattern.
//
// Patterns use the same syntax as routes registered on an App, including
// parameters (for example `:id`), wildcards (`*`, `+`), and optional segments.
// The optional Config argument can be used to control case sensitivity and
// strict routing behavior. This helper allows checking potential matches
// without registering a route.
func RoutePatternMatch(path, pattern string, cfg ...Config) bool {
	// See logic in (*Route).match and (*App).register
	var ctxParams [maxParams]string

	config := Config{}
	if len(cfg) > 0 {
		config = cfg[0]
	}
	config.RegexHandler = validateRegexHandler(config.RegexHandler)

	if path == "" {
		path = "/"
	}

	// Cannot have an empty pattern
	if pattern == "" {
		pattern = "/"
	}
	// Pattern always start with a '/'
	if pattern[0] != '/' {
		pattern = "/" + pattern
	}

	patternPretty := []byte(pattern)

	// Case-sensitive routing, all to lowercase
	if !config.CaseSensitive {
		patternPretty = utilsbytes.UnsafeToLower(patternPretty)
		path = utilsstrings.ToLower(path)
	}
	// Strict routing, remove trailing slashes
	if !config.StrictRouting && len(patternPretty) > 1 {
		patternPretty = utils.TrimRight(patternPretty, '/')
	}

	parser, _ := routerParserPool.Get().(*routeParser) //nolint:errcheck // only contains routeParser
	parser.reset()
	patternStr := string(patternPretty)
	parser.parseRoute(patternStr, config.RegexHandler)
	defer routerParserPool.Put(parser)

	// '*' wildcard matches any path
	if (patternStr == "/" && path == "/") || patternStr == "/*" {
		return true
	}

	// Does this route have parameters
	if len(parser.params) > 0 {
		if match := parser.getMatch(path, path, &ctxParams, false); match {
			return true
		}
	}
	// Check for a simple match
	patternPretty = RemoveEscapeCharBytes(patternPretty)

	return string(patternPretty) == path
}

func (parser *routeParser) reset() {
	parser.segs = parser.segs[:0]
	parser.params = parser.params[:0]
	parser.wildCardCount = 0
	parser.plusCount = 0
}

// parseRoute analyzes the route and divides it into segments for constant areas and parameters,
// this information is needed later when assigning the requests to the declared routes
func (parser *routeParser) parseRoute(pattern string, regexHandler any, customConstraints ...CustomConstraint) {
	var n int
	var seg *routeSegment
	for pattern != "" {
		nextParamPosition := findNextParamPosition(pattern)
		// handle the parameter part
		if nextParamPosition == 0 {
			n, seg = parser.analyseParameterPart(pattern, regexHandler, customConstraints...)
			parser.params, parser.segs = append(parser.params, seg.ParamName), append(parser.segs, seg)
		} else {
			n, seg = parser.analyseConstantPart(pattern, nextParamPosition)
			parser.segs = append(parser.segs, seg)
		}
		pattern = pattern[n:]
	}
	// mark last segment
	if len(parser.segs) > 0 {
		parser.segs[len(parser.segs)-1].IsLast = true
	}
	parser.segs = addParameterMetaInfo(parser.segs)
}

// parseRoute analyzes the route and divides it into segments for constant areas and parameters,
// this information is needed later when assigning the requests to the declared routes
func parseRoute(pattern string, regexHandler any, customConstraints ...CustomConstraint) routeParser {
	parser := routeParser{}
	parser.parseRoute(pattern, regexHandler, customConstraints...)

	// Check if the route has too many parameters
	if len(parser.params) > maxParams {
		panic(fmt.Sprintf("Route '%s' has %d parameters, which exceeds the maximum of %d",
			pattern, len(parser.params), maxParams))
	}

	return parser
}

// addParameterMetaInfo add important meta information to the parameter segments
// to simplify the search for the end of the parameter
func addParameterMetaInfo(segs []*routeSegment) []*routeSegment {
	var comparePart string
	segLen := len(segs)
	// loop from end to begin
	for i := segLen - 1; i >= 0; i-- {
		// set the compare part for the parameter
		if segs[i].IsParam {
			// important for finding the end of the parameter
			segs[i].ComparePart = RemoveEscapeChar(comparePart)
		} else {
			comparePart = segs[i].Const
			if len(comparePart) > 1 {
				comparePart = utils.TrimRight(comparePart, slashDelimiter)
			}
		}
	}

	// loop from beginning to end
	for i := range segLen {
		// check how often the compare part is in the following const parts
		if segs[i].IsParam {
			// check if parameter segments are directly after each other;
			// when neither this parameter nor the next parameter are greedy, we only want one character
			if segLen > i+1 && !segs[i].IsGreedy && segs[i+1].IsParam && !segs[i+1].IsGreedy {
				segs[i].Length = 1
			}
			if segs[i].ComparePart == "" {
				continue
			}
			for j := i + 1; j <= len(segs)-1; j++ {
				if !segs[j].IsParam {
					// count is important for the greedy match
					segs[i].PartCount += strings.Count(segs[j].Const, segs[i].ComparePart)
				}
			}
			// check if the end of the segment is an optional slash and then if the segment is optional or the last one
		} else if segs[i].Const[len(segs[i].Const)-1] == slashDelimiter && (segs[i].IsLast || (segLen > i+1 && segs[i+1].IsOptional)) {
			segs[i].HasOptionalSlash = true
		}
	}

	return segs
}

// findNextParamPosition search for the next possible parameter start position
func findNextParamPosition(pattern string) int {
	// Find the first parameter position
	next := -1
	for i := range pattern {
		if parameterStartChars[pattern[i]] && (i == 0 || pattern[i-1] != escapeChar) {
			next = i
			break
		}
	}
	if next > 0 && pattern[next] != wildcardParam {
		// checking the found parameterStartChar is a cluster
		for i := next + 1; i < len(pattern); i++ {
			if !parameterStartChars[pattern[i]] {
				return i - 1
			}
		}
		return len(pattern) - 1
	}
	return next
}

// analyseConstantPart find the end of the constant part and create the route segment
func (*routeParser) analyseConstantPart(pattern string, nextParamPosition int) (int, *routeSegment) {
	// handle the constant part
	processedPart := pattern
	if nextParamPosition != -1 {
		// remove the constant part until the parameter
		processedPart = pattern[:nextParamPosition]
	}
	constPart := RemoveEscapeChar(processedPart)
	return len(processedPart), &routeSegment{
		Const:  constPart,
		Length: len(constPart),
	}
}

// analyseParameterPart find the parameter end and create the route segment
func (parser *routeParser) analyseParameterPart(pattern string, regexHandler any, customConstraints ...CustomConstraint) (int, *routeSegment) {
	isWildCard := pattern[0] == wildcardParam
	isPlusParam := pattern[0] == plusParam

	paramEndPosition := 0
	paramConstraintStartPosition := -1
	paramConstraintEndPosition := -1

	// handle wildcard end
	if !isWildCard && !isPlusParam {
		paramEndPosition = -1
		search := pattern[1:]
		for i := range search {
			if paramConstraintStartPosition == -1 && search[i] == paramConstraintStart && (i == 0 || search[i-1] != escapeChar) {
				paramConstraintStartPosition = i + 1
				continue
			}
			if paramConstraintStartPosition != -1 && search[i] == paramConstraintEnd && (i == 0 || search[i-1] != escapeChar) {
				paramConstraintEndPosition = i + 1
				continue
			}
			if parameterEndChars[search[i]] {
				if (paramConstraintStartPosition == -1 && paramConstraintEndPosition == -1) ||
					(paramConstraintStartPosition != -1 && paramConstraintEndPosition != -1) {
					paramEndPosition = i
					break
				}
			}
		}

		switch {
		case paramEndPosition == -1:
			paramEndPosition = len(pattern) - 1
		case bytes.IndexByte(parameterDelimiterChars, pattern[paramEndPosition+1]) == -1:
			paramEndPosition++
		default:
			// do nothing
		}
	}

	// cut params part
	processedPart := pattern[0 : paramEndPosition+1]
	n := paramEndPosition + 1
	paramName := RemoveEscapeChar(GetTrimmedParam(processedPart))

	// Check has constraint
	var constraints []*Constraint

	if hasConstraint := paramConstraintStartPosition != -1 && paramConstraintEndPosition != -1; hasConstraint {
		constraintString := pattern[paramConstraintStartPosition+1 : paramConstraintEndPosition]
		userConstraints := splitNonEscaped(constraintString, paramConstraintSeparator)
		constraints = make([]*Constraint, 0, len(userConstraints))

		for _, c := range userConstraints {
			start := findNextNonEscapedCharPosition(c, paramConstraintDataStart)
			end := strings.LastIndexByte(c, paramConstraintDataEnd)

			var rawName string
			var data []string

			if start != -1 && end != -1 {
				rawName = c[:start]
				data = []string{c[start+1 : end]}
			} else {
				rawName = c
				data = []string{}
			}

			handler := findConstraintHandler(rawName, regexHandler, customConstraints)
			if handler == nil {
				handler = findConstraintHandler(resolveConstraintName(rawName), regexHandler, customConstraints)
			}
			if handler == nil {
				continue
			}

			constraint := newConstraint(handler, rawName, data)
			constraints = append(constraints, constraint)
		}

		paramName = RemoveEscapeChar(GetTrimmedParam(pattern[0:paramConstraintStartPosition]))
	}

	if isWildCard {
		parser.wildCardCount++
		paramName += strconv.Itoa(parser.wildCardCount)
	} else if isPlusParam {
		parser.plusCount++
		paramName += strconv.Itoa(parser.plusCount)
	}

	segment := &routeSegment{
		ParamName:  paramName,
		IsParam:    true,
		IsOptional: isWildCard || pattern[paramEndPosition] == optionalParam,
		IsGreedy:   isWildCard || isPlusParam,
	}

	if len(constraints) > 0 {
		segment.Constraints = constraints
	}

	return n, segment
}

// findNextNonEscapedCharPosition searches the next char position and skips the escaped characters
func findNextNonEscapedCharPosition(search string, char byte) int {
	for i := 0; i < len(search); i++ {
		if search[i] == char && (i == 0 || search[i-1] != escapeChar) {
			return i
		}
	}
	return -1
}

// splitNonEscaped slices s into all substrings separated by sep and returns a slice of the substrings between those separators
// This function also takes a care of escape char when splitting.
func splitNonEscaped(s string, sep byte) []string {
	var result []string
	i := findNextNonEscapedCharPosition(s, sep)

	for i > -1 {
		result = append(result, s[:i])
		s = s[i+1:]
		i = findNextNonEscapedCharPosition(s, sep)
	}

	return append(result, s)
}

func hasPartialMatchBoundary(path string, matchedLength int) bool {
	if matchedLength < 0 || matchedLength > len(path) {
		return false
	}
	if matchedLength == len(path) {
		return true
	}
	if matchedLength == 0 {
		return false
	}
	if path[matchedLength-1] == slashDelimiter {
		return true
	}
	if matchedLength < len(path) && path[matchedLength] == slashDelimiter {
		return true
	}

	return false
}

// getMatch parses the passed url and tries to match it against the route segments and determine the parameter positions
func (parser *routeParser) getMatch(detectionPath, path string, params *[maxParams]string, partialCheck bool) bool { //nolint:revive // Accepting a bool param is fine here
	originalDetectionPath := detectionPath
	// offset tracks how many bytes of the original path have been consumed. path
	// itself is never re-sliced: only param segments read from it, as
	// path[offset:offset+i]. detectionPath and path advance in lockstep by the same
	// i, and detectionPath is at most one (trailing-slash) byte shorter than path, so
	// offset+i never exceeds len(path). This avoids a slice-header write per segment.
	var i, paramsIterator, partLen, offset int
	for _, segment := range parser.segs {
		partLen = len(detectionPath)
		// check const segment
		if !segment.IsParam {
			i = segment.Length
			// is optional part or the const part must match with the given string
			// check if the end of the segment is an optional slash
			if segment.HasOptionalSlash && partLen == i-1 && detectionPath == segment.Const[:i-1] {
				i--
			} else if i > partLen || detectionPath[:i] != segment.Const {
				return false
			}
		} else {
			// determine parameter length
			i = findParamLen(detectionPath, segment)
			if !segment.IsOptional && i == 0 {
				return false
			}
			// take over the params positions
			params[paramsIterator] = path[offset : offset+i]

			if !segment.IsOptional || i != 0 {
				// check constraint
				for _, c := range segment.Constraints {
					if matched := c.matchConstraint(params[paramsIterator]); !matched {
						return false
					}
				}
			}

			paramsIterator++
		}

		// reduce founded part from the string
		if partLen > 0 {
			detectionPath = detectionPath[i:]
			offset += i
		}
	}
	if detectionPath != "" {
		if !partialCheck {
			return false
		}
		consumedLength := len(originalDetectionPath) - len(detectionPath)
		if !hasPartialMatchBoundary(originalDetectionPath, consumedLength) {
			return false
		}
	}

	return true
}

// findParamLen for the expressjs wildcard behavior (right to left greedy)
// look at the other segments and take what is left for the wildcard from right to left
func findParamLen(s string, segment *routeSegment) int {
	if segment.IsLast {
		return findParamLenForLastSegment(s, segment)
	}

	if segment.Length != 0 && len(s) >= segment.Length {
		return segment.Length
	} else if segment.IsGreedy {
		// Search the parameters until the next constant part
		// special logic for greedy params
		searchCount := strings.Count(s, segment.ComparePart)
		if searchCount > 1 {
			return findGreedyParamLen(s, searchCount, segment)
		}
	}

	if len(segment.ComparePart) == 1 {
		if constPosition := strings.IndexByte(s, segment.ComparePart[0]); constPosition != -1 {
			return constPosition
		}
	} else if constPosition := strings.Index(s, segment.ComparePart); constPosition != -1 {
		// if the compare part was found, but contains a slash although this part is not greedy, then it must not match
		// example: /api/:param/fixedEnd -> path: /api/123/456/fixedEnd = no match , /api/123/fixedEnd = match
		if !segment.IsGreedy && strings.IndexByte(s[:constPosition], slashDelimiter) != -1 {
			return 0
		}
		return constPosition
	}

	return len(s)
}

// findParamLenForLastSegment get the length of the parameter if it is the last segment
func findParamLenForLastSegment(s string, seg *routeSegment) int {
	if !seg.IsGreedy {
		if i := strings.IndexByte(s, slashDelimiter); i != -1 {
			return i
		}
	}

	return len(s)
}

// findGreedyParamLen get the length of the parameter for greedy segments from right to left
func findGreedyParamLen(s string, searchCount int, segment *routeSegment) int {
	// check all from right to left segments
	for i := segment.PartCount; i > 0 && searchCount > 0; i-- {
		searchCount--

		constPosition := strings.LastIndex(s, segment.ComparePart)
		if constPosition == -1 {
			break
		}
		s = s[:constPosition]
	}

	return len(s)
}

// GetTrimmedParam trims the ':' & '?' from a string
func GetTrimmedParam(param string) string {
	start := 0
	end := len(param)

	if end == 0 || param[start] != paramStarterChar { // is not a param
		return param
	}
	start++
	if param[end-1] == optionalParam { // is ?
		end--
	}

	return param[start:end]
}

// RemoveEscapeChar removes escape characters
func RemoveEscapeChar(word string) string {
	// Fast path: check if there are any escape characters first
	escapeIdx := strings.IndexByte(word, '\\')
	if escapeIdx == -1 {
		return word // No escape chars, return original string without allocation
	}

	// Slow path: copy and remove escape characters
	b := []byte(word)
	dst := escapeIdx
	for src := escapeIdx + 1; src < len(b); src++ {
		if b[src] != '\\' {
			b[dst] = b[src]
			dst++
		}
	}
	return string(b[:dst])
}

// RemoveEscapeCharBytes removes escape characters
func RemoveEscapeCharBytes(word []byte) []byte {
	dst := 0
	for src := range word {
		if word[src] != '\\' {
			word[dst] = word[src]
			dst++
		}
	}
	return word[:dst]
}

// CheckConstraint validates if a param matches the given constraint.
// Kept for backward compatibility with external callers.
func (c *Constraint) CheckConstraint(param string) bool {
	return c.matchConstraint(param)
}

// asciiLowerTable maps every byte to its ASCII-lowercase equivalent. Non-letter
// bytes and bytes >= 0x80 map to themselves, so only A-Z are folded. It backs
// appendLowerASCII, the fused copy+lowercase used on the request hot path.
var asciiLowerTable = func() [256]byte {
	var t [256]byte
	for i := 0; i < 256; i++ {
		c := byte(i)
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		t[i] = c
	}
	return t
}()

// appendLowerASCII writes the ASCII-lowercased bytes of src into dst[:0] in a
// single pass, reusing dst's backing array when it is large enough. It fuses the
// copy and the lowercasing that configDependentPaths would otherwise do as two
// separate passes (append + UnsafeToLower), saving one traversal of the path on
// every case-insensitive request.
func appendLowerASCII(dst, src []byte) []byte {
	n := len(src)
	if cap(dst) < n {
		dst = make([]byte, n)
	} else {
		dst = dst[:n]
	}

	table := &asciiLowerTable
	i := 0
	limit := n &^ 3
	for i < limit {
		dst[i+0] = table[src[i+0]]
		dst[i+1] = table[src[i+1]]
		dst[i+2] = table[src[i+2]]
		dst[i+3] = table[src[i+3]]
		i += 4
	}
	for ; i < n; i++ {
		dst[i] = table[src[i]]
	}
	return dst
}
