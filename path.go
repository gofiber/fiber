// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was inspired by ucarion/urlpath (MIT License).
// 💖 Maintained and modified for Fiber by @renewerner87

package fiber

import (
	"strings"
	"sync/atomic"

	utils "github.com/gofiber/utils"
)

// routeParser  holds the path segments and param names
type routeParser struct {
	segs   []routeSegment
	params []string
}

// paramsSeg holds the segment metadata
type routeSegment struct {
	Param      string
	Const      string
	IsParam    bool
	IsWildcard bool
	IsOptional bool
	IsLast     bool
}

const (
	wildcardParam    byte = '*'
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
	parameterStartChars = []byte{wildcardParam, paramStarterChar}
	// list of chars at the end of the parameter
	parameterDelimiterChars = append([]byte{paramStarterChar}, routeDelimiter...)
	// list of chars to find the end of a parameter
	parameterEndChars = append([]byte{optionalParam}, parameterDelimiterChars...)
)

// parseRoute analyzes the route and divides it into segments for constant areas and parameters,
// this information is needed later when assigning the requests to the declared routes
func parseRoute(pattern string) routeParser {
	var segList []routeSegment
	var params []string

	part := ""
	for len(pattern) > 0 {
		nextParamPosition := findNextParamPosition(pattern)
		// handle the parameter part
		if nextParamPosition == 0 {
			processedPart, seg := analyseParameterPart(pattern)
			params, segList, part = append(params, seg.Param), append(segList, seg), processedPart
		} else {
			processedPart, seg := analyseConstantPart(pattern, nextParamPosition)
			segList, part = append(segList, seg), processedPart
		}

		// reduce the pattern by the processed parts
		if len(part) == len(pattern) {
			break
		}
		pattern = pattern[len(part):]
	}
	// mark last segment
	if len(segList) > 0 {
		segList[len(segList)-1].IsLast = true
	}

	return routeParser{segs: segList, params: params}
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
func analyseConstantPart(pattern string, nextParamPosition int) (string, routeSegment) {
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
func analyseParameterPart(pattern string) (string, routeSegment) {
	isWildCard := pattern[0] == wildcardParam
	parameterEndPosition := findNextCharsetPosition(pattern[1:], parameterEndChars)
	// handle wildcard end
	if isWildCard {
		parameterEndPosition = 0
	} else if parameterEndPosition == -1 {
		parameterEndPosition = len(pattern) - 1
	} else if false == isInCharset(pattern[parameterEndPosition+1], parameterDelimiterChars) {
		parameterEndPosition = parameterEndPosition + 1
	}
	// cut params part
	processedPart := pattern[0 : parameterEndPosition+1]

	return processedPart, routeSegment{
		Param:      utils.GetTrimmedParam(processedPart),
		IsParam:    true,
		IsOptional: isWildCard || pattern[parameterEndPosition] == optionalParam,
		IsWildcard: isWildCard,
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
func (p *routeParser) getMatch(s string, partialCheck bool) ([][2]int, bool) {
	lenKeys := len(p.params)
	paramsPositions := getAllocFreeParamsPos(lenKeys)
	var i, paramsIterator, partLen, paramStart int
	for index, segment := range p.segs {
		partLen = len(s)
		// check parameter
		if segment.IsParam {
			// determine parameter length
			i = findParamLen(s, p.segs, index)
			if !segment.IsOptional && i == 0 {
				return nil, false
			}

			paramsPositions[paramsIterator][0], paramsPositions[paramsIterator][1] = paramStart, paramStart+i
			paramsIterator++
		} else {
			// check const segment
			optionalPart := false
			i = len(segment.Const)
			if i > 0 && partLen == i-1 && segment.Const[i-1] == slashDelimiter && s[:i-1] == segment.Const[:i-1] {
				if segment.IsLast || p.segs[index+1].IsOptional {
					i--
					optionalPart = true
				}
			}

			if optionalPart == false && (partLen < i || (i == 0 && partLen > 0) || s[:i] != segment.Const) {
				return nil, false
			}
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
func (p *routeParser) paramsForPos(path string, paramsPositions [][2]int) []string {
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
		if segments[currIndex].IsWildcard {
			return len(s)
		}
		if i := strings.IndexByte(s, slashDelimiter); i != -1 {
			return i
		}

		return len(s)
	}
	// "/api/*/:param" - "/api/joker/batman/robin/1" -> "joker/batman/robin", "1"
	// "/api/*/:param" - "/api/joker/batman"         -> "joker", "batman"
	// "/api/*/:param" - "/api/joker-batman-robin/1" -> "joker-batman-robin", "1"
	nextSeg := segments[currIndex+1]
	// check next segment
	if nextSeg.IsParam {
		if segments[currIndex].IsWildcard || nextSeg.IsWildcard {
			// greedy logic
			for i := currIndex + 1; i < len(segments); i++ {
				if false == segments[i].IsParam {
					nextSeg = segments[i]
					break
				}
			}
		} else if len(s) > 0 {
			// in case the next parameter or the current parameter is not a wildcard its not greedy, we only want one character
			return 1
		}
	}
	// get the length to the next constant part
	if false == nextSeg.IsParam {
		if constPosition := strings.Index(s, nextSeg.Const); constPosition != -1 {
			return constPosition
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

// TODO: replace it with bytebufferpool and release the parameter buffers in ctx release function
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
