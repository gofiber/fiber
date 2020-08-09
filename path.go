// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was inspired by ucarion/urlpath (MIT License).
// 💖 Maintained and modified for Fiber by @renewerner87

package fiber

import (
	"strconv"
	"strings"
	"sync/atomic"

	utils "github.com/gofiber/utils"
)

// routeParser holds the path segments and param names
type routeParser struct {
	segs          []routeSegment
	params        []string
	wildCardCount int
	plusCount     int
}

// paramsSeg holds the segment metadata
type routeSegment struct {
	Const string

	IsParam     bool
	ParamName   string
	ComparePart string // search part to find the end of the parameter
	PartCount   int    // how often is the search part contained in the non-param segments? -> necessary for greedy search
	IsWildcard  bool
	IsGreedy    bool
	IsOptional  bool
	IsLast      bool
	// TODO: add support for optional groups ?
}

const (
	wildcardParam    byte = '*'
	plusParam        byte = '+'
	optionalParam    byte = '?'
	slashDelimiter   byte = '/'
	paramStarterChar byte = ':'
)

var (
	// list of possible parameter and segment delimiter
	// slash has a special role, unlike the other parameters it must not be interpreted as a parameter
	// TODO '(' ')' delimiters for regex patterns
	routeDelimiter = []byte{slashDelimiter, '-', '.'}
	// list of chars for the parameter recognising
	parameterStartChars = []byte{wildcardParam, plusParam, paramStarterChar}
	// list of chars at the end of the parameter
	parameterDelimiterChars = append([]byte{paramStarterChar}, routeDelimiter...)
	// list of chars to find the end of a parameter
	parameterEndChars = append([]byte{optionalParam}, parameterDelimiterChars...)
)

// parseRoute analyzes the route and divides it into segments for constant areas and parameters,
// this information is needed later when assigning the requests to the declared routes
func parseRoute(pattern string) routeParser {
	parser := routeParser{}

	part := ""
	for len(pattern) > 0 {
		nextParamPosition := findNextParamPosition(pattern)
		// handle the parameter part
		if nextParamPosition == 0 {
			processedPart, seg := parser.analyseParameterPart(pattern)
			parser.params, parser.segs, part = append(parser.params, seg.ParamName), append(parser.segs, seg), processedPart
		} else {
			processedPart, seg := parser.analyseConstantPart(pattern, nextParamPosition)
			parser.segs, part = append(parser.segs, seg), processedPart
		}

		// reduce the pattern by the processed parts
		if len(part) == len(pattern) {
			break
		}
		pattern = pattern[len(part):]
	}
	// mark last segment
	if len(parser.segs) > 0 {
		parser.segs[len(parser.segs)-1].IsLast = true
	}
	parser.segs = addParameterMetaInfo(parser.segs)

	return parser
}

// addParameterMetaInfo add important meta information to the parameter segments
// to simplify the search for the end of the parameter
func addParameterMetaInfo(segs []routeSegment) []routeSegment {
	comparePart := ""
	// loop from end to begin
	for i := len(segs) - 1; i >= 0; i-- {
		// set the compare part for the parameter
		if segs[i].IsParam {
			segs[i].ComparePart = comparePart
		} else {
			comparePart = segs[i].Const
			if len(comparePart) > 1 {
				comparePart = utils.TrimRight(comparePart, slashDelimiter)
			}
		}
	}

	// loop from begin to end
	for i := 0; i < len(segs); i++ {
		// check how often the compare part is in the following const parts
		if segs[i].IsParam && segs[i].ComparePart != "" {
			for j := i + 1; j < len(segs)-1; j++ {
				if !segs[j].IsParam {
					segs[i].PartCount += strings.Count(segs[j].Const, segs[i].ComparePart)
				}
			}
		}
	}

	return segs
}

// findNextParamPosition search for the next possible parameter start position
func findNextParamPosition(pattern string) int {
	nextParamPosition := findNextCharsetPosition(pattern, parameterStartChars)
	if nextParamPosition != -1 && len(pattern) > nextParamPosition && pattern[nextParamPosition] != wildcardParam {
		// search for parameter characters for the found parameter start,
		// if there are more, move the parameter start to the last parameter char
		for found := findNextCharsetPosition(pattern[nextParamPosition+1:], parameterStartChars); found == 0; {
			nextParamPosition++
			if len(pattern) > nextParamPosition {
				break
			}
		}
	}

	return nextParamPosition
}

// analyseConstantPart find the end of the constant part and create the route segment
func (routeParser *routeParser) analyseConstantPart(pattern string, nextParamPosition int) (string, routeSegment) {
	// handle the constant part
	processedPart := pattern
	if nextParamPosition != -1 {
		// remove the constant part until the parameter
		processedPart = pattern[:nextParamPosition]
	}
	return processedPart, routeSegment{
		Const: processedPart,
	}
}

// analyseParameterPart find the parameter end and create the route segment
func (routeParser *routeParser) analyseParameterPart(pattern string) (string, routeSegment) {
	isWildCard := pattern[0] == wildcardParam
	isPlusParam := pattern[0] == plusParam
	parameterEndPosition := findNextCharsetPosition(pattern[1:], parameterEndChars)
	// handle wildcard end
	if isWildCard || isPlusParam {
		parameterEndPosition = 0
	} else if parameterEndPosition == -1 {
		parameterEndPosition = len(pattern) - 1
	} else if !isInCharset(pattern[parameterEndPosition+1], parameterDelimiterChars) {
		parameterEndPosition = parameterEndPosition + 1
	}
	// cut params part
	processedPart := pattern[0 : parameterEndPosition+1]

	paramName := processedPart
	// add access iterator to wildcard and plus
	if isWildCard {
		routeParser.wildCardCount++
		paramName += strconv.Itoa(routeParser.wildCardCount)
	} else if isPlusParam {
		routeParser.plusCount++
		paramName += strconv.Itoa(routeParser.plusCount)
	} else {
		paramName = utils.GetTrimmedParam(paramName)
	}

	return processedPart, routeSegment{
		ParamName:  paramName,
		IsParam:    true,
		IsOptional: isWildCard || pattern[parameterEndPosition] == optionalParam,
		IsGreedy:   isWildCard || isPlusParam,
	}
}

// isInCharset check is the given character in the charset list
func isInCharset(searchChar byte, charset []byte) bool {
	for _, char := range charset {
		if char == searchChar {
			return true
		}
	}
	return false
}

// findNextCharsetPosition search the next char position from the charset
func findNextCharsetPosition(search string, charset []byte) int {
	nextPosition := -1
	for _, char := range charset {
		if pos := strings.IndexByte(search, char); pos != -1 && (pos < nextPosition || nextPosition == -1) {
			nextPosition = pos
		}
	}

	return nextPosition
}

// getMatch parses the passed url and tries to match it against the route segments and determine the parameter positions
func (routeParser *routeParser) getMatch(s string, partialCheck bool) ([][2]int, bool) {
	lenKeys := len(routeParser.params)
	paramsPositions := getAllocFreeParamsPos(lenKeys)
	var i, paramsIterator, partLen, paramStart int
	for index, segment := range routeParser.segs {
		partLen = len(s)
		// check parameter
		if !segment.IsParam {
			// check const segment
			optionalPart := false
			i = len(segment.Const)
			// check if the end of the segment is a optional slash and then if the segement is optional or the last one
			if i > 0 && partLen == i-1 && segment.Const[i-1] == slashDelimiter && s[:i-1] == segment.Const[:i-1] {
				if segment.IsLast || routeParser.segs[index+1].IsOptional {
					i--
					optionalPart = true
				}
			}
			// is optional part or the const part must match with the given string
			if !optionalPart && (partLen < i || (i == 0 && partLen > 0) || s[:i] != segment.Const) {
				return nil, false
			}
		} else {
			// determine parameter length
			i = findParamLen(s, routeParser.segs, index)
			if !segment.IsOptional && i == 0 {
				return nil, false
			}
			// take over the params positions
			paramsPositions[paramsIterator][0], paramsPositions[paramsIterator][1] = paramStart, paramStart+i
			paramsIterator++
		}

		// reduce founded part from the string
		if partLen > 0 {
			if partLen < i {
				i = partLen
			}
			paramStart += i

			s = s[i:]
		}
	}
	if len(s) != 0 && !partialCheck {
		return nil, false
	}

	return paramsPositions, true
}

// paramsForPos get parameters for the given positions from the given path
func (routeParser *routeParser) paramsForPos(path string, paramsPositions [][2]int) []string {
	size := len(paramsPositions)
	params := getAllocFreeParams(size)
	for i, positions := range paramsPositions {
		if positions[0] != positions[1] && len(path) >= positions[1] {
			params[i] = path[positions[0]:positions[1]]
		} else {
			params[i] = ""
		}
	}

	return params
}

// findParamLen for the expressjs wildcard behavior (right to left greedy)
// look at the other segments and take what is left for the wildcard from right to left
func findParamLen(s string, segments []routeSegment, currIndex int) int {
	if segments[currIndex].IsLast {
		return findParamLenForLastSegment(s, segments[currIndex])
	}

	compareSeg := segments[currIndex+1]
	// check if parameter segments are directly after each other
	if compareSeg.IsParam {
		// and if one of them is greedy
		if !segments[currIndex].IsGreedy && !compareSeg.IsGreedy && len(s) > 0 {
			// in case the next parameter or the current parameter is not a wildcard its not greedy, we only want one character
			return 1
		}
	}

	return findParamLenUntilNextConstSeg(s, segments[currIndex])
}

// findParamLenUntilNextConstSeg Search the parameters until the next constant part
func findParamLenUntilNextConstSeg(s string, segment routeSegment) int {
	// special logic for greedy params
	if segment.IsGreedy {
		searchCount := strings.Count(s, segment.ComparePart)
		if searchCount > 1 {
			return findGreedyParamLen(s, searchCount, segment)
		}
	}

	if constPosition := strings.Index(s, segment.ComparePart); constPosition != -1 {
		return constPosition
	}

	return len(s)
}

// findParamLenForLastSegment get the length of the parameter if it is the last segment
func findParamLenForLastSegment(s string, seg routeSegment) int {
	if seg.IsGreedy {
		return len(s)
	}
	if i := strings.IndexByte(s, slashDelimiter); i != -1 {
		return i
	}

	return len(s)
}

// findGreedyParamLen get the length of the parameter for greedy segments from right to left
func findGreedyParamLen(s string, searchCount int, segment routeSegment) int {
	// check all from right to left segments
	for i := segment.PartCount; i > 0 && searchCount > 0; i-- {
		searchCount--
		if constPosition := strings.LastIndex(s, segment.ComparePart); constPosition != -1 {
			s = s[:constPosition]
		} else {
			break
		}
	}

	return len(s)
}

// performance tricks
// creates predefined arrays that are used to match the request routes so that no allocations need to be made
var paramsDummy, paramsPosDummy = make([]string, 100000), make([][2]int, 100000)

// positions parameter that moves further and further to the right and remains atomic over all simultaneous requests
// to assign a separate range to each request
var startParamList, startParamPosList uint32 = 0, 0

// getAllocFreeParamsPos fetches a slice area from the predefined slice, which is currently not in use
func getAllocFreeParamsPos(allocLen int) [][2]int {
	size := uint32(allocLen)
	start := atomic.AddUint32(&startParamPosList, size)
	if (start + 10) >= uint32(len(paramsPosDummy)) {
		atomic.StoreUint32(&startParamPosList, 0)
		return getAllocFreeParamsPos(allocLen)
	}
	start -= size
	allocLen += int(start)
	paramsPositions := paramsPosDummy[start:allocLen:allocLen]
	return paramsPositions
}

// getAllocFreeParams fetches a slice area from the predefined slice, which is currently not in use
func getAllocFreeParams(allocLen int) []string {
	size := uint32(allocLen)
	start := atomic.AddUint32(&startParamList, size)
	if (start + 10) >= uint32(len(paramsDummy)) {
		atomic.StoreUint32(&startParamList, 0)
		return getAllocFreeParams(allocLen)
	}
	start -= size
	allocLen += int(start)
	params := paramsDummy[start:allocLen:allocLen]
	return params
}
