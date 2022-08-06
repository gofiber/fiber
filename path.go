// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was inspired by ucarion/urlpath (MIT License).
// 💖 Maintained and modified for Fiber by @renewerner87

package fiber

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2/internal/uuid"
	"github.com/gofiber/fiber/v2/utils"
)

// routeParser holds the path segments and param names
type routeParser struct {
	segs          []*routeSegment // the parsed segments of the route
	params        []string        // that parameter names the parsed route
	wildCardCount int             // number of wildcard parameters, used internally to give the wildcard parameter its number
	plusCount     int             // number of plus parameters, used internally to give the plus parameter its number
}

// paramsSeg holds the segment metadata
type routeSegment struct {
	// const information
	Const string // constant part of the route
	// parameter information
	IsParam     bool   // Truth value that indicates whether it is a parameter or a constant part
	ParamName   string // name of the parameter for access to it, for wildcards and plus parameters access iterators starting with 1 are added
	ComparePart string // search part to find the end of the parameter
	PartCount   int    // how often is the search part contained in the non-param segments? -> necessary for greedy search
	IsGreedy    bool   // indicates whether the parameter is greedy or not, is used with wildcard and plus
	IsOptional  bool   // indicates whether the parameter is optional or not
	// common information
	IsLast           bool        // shows if the segment is the last one for the route
	HasOptionalSlash bool        // segment has the possibility of an optional slash
	Constraint       *Constraint // Constraint type if segment is a parameter, if not it will be set to noConstraint by default
	Length           int         // length of the parameter for segment, when its 0 then the length is undetermined
	// future TODO: add support for optional groups "/abc(/def)?"
}

// different special routing signs
const (
	wildcardParam        byte = '*'  // indicates a optional greedy parameter
	plusParam            byte = '+'  // indicates a required greedy parameter
	optionalParam        byte = '?'  // concludes a parameter by name and makes it optional
	paramStarterChar     byte = ':'  // start character for a parameter with name
	slashDelimiter       byte = '/'  // separator for the route, unlike the other delimiters this character at the end can be optional
	escapeChar           byte = '\\' // escape character
	paramConstraintStart byte = '<'  // start of type constraint for a parameter
	paramConstraintEnd   byte = '>'  // end of type constraint for a parameter

)

// parameter constraint types
type TypeConstraint int16

type Constraint struct {
	ID   TypeConstraint
	Data []string
}

const (
	noConstraint TypeConstraint = iota
	intConstraint
	boolConstraint
	floatConstraint
	alphaConstraint
	datetimeConstraint
	guidConstraint
	minLengthConstraint
	maxLengthConstraint
	exactLengthConstraint
	BetweenLengthConstraint
	minConstraint
	maxConstraint
	rangeConstraint
	regexConstraint
)

// list of possible parameter and segment delimiter
var (
	// slash has a special role, unlike the other parameters it must not be interpreted as a parameter
	routeDelimiter = []byte{slashDelimiter, '-', '.'}
	// list of chars for the parameter recognising
	parameterStartChars = []byte{wildcardParam, plusParam, paramStarterChar}
	// list of chars of delimiters and the starting parameter name char
	parameterDelimiterChars = append([]byte{paramStarterChar}, routeDelimiter...)
	// list of chars to find the end of a parameter
	parameterEndChars = append([]byte{optionalParam}, parameterDelimiterChars...)
	// list of parameter constraint start
	parameterConstraintStartChars = []byte{paramConstraintStart}
	// list of parameter constraint end
	parameterConstraintEndChars = []byte{paramConstraintEnd}
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

	// loop from begin to end
	for i := 0; i < segLen; i++ {
		// check how often the compare part is in the following const parts
		if segs[i].IsParam {
			// check if parameter segments are directly after each other and if one of them is greedy
			// in case the next parameter or the current parameter is not a wildcard its not greedy, we only want one character
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
			// check if the end of the segment is a optional slash and then if the segement is optional or the last one
		} else if segs[i].Const[len(segs[i].Const)-1] == slashDelimiter && (segs[i].IsLast || (segLen > i+1 && segs[i+1].IsOptional)) {
			segs[i].HasOptionalSlash = true
		}
	}

	return segs
}

// findNextParamPosition search for the next possible parameter start position
func findNextParamPosition(pattern string) int {
	nextParamPosition := findNextNonEscapedCharsetPosition(pattern, parameterStartChars)
	if nextParamPosition != -1 && len(pattern) > nextParamPosition && pattern[nextParamPosition] != wildcardParam {
		// search for parameter characters for the found parameter start,
		// if there are more, move the parameter start to the last parameter char
		for found := findNextNonEscapedCharsetPosition(pattern[nextParamPosition+1:], parameterStartChars); found == 0; {
			nextParamPosition++
			if len(pattern) > nextParamPosition {
				break
			}
		}
	}

	return nextParamPosition
}

// analyseConstantPart find the end of the constant part and create the route segment
func (routeParser *routeParser) analyseConstantPart(pattern string, nextParamPosition int) (string, *routeSegment) {
	// handle the constant part
	processedPart := pattern
	if nextParamPosition != -1 {
		// remove the constant part until the parameter
		processedPart = pattern[:nextParamPosition]
	}
	constPart := RemoveEscapeChar(processedPart)
	return processedPart, &routeSegment{
		Const:  constPart,
		Length: len(constPart),
	}
}

// analyseParameterPart find the parameter end and create the route segment
func (routeParser *routeParser) analyseParameterPart(pattern string) (string, *routeSegment) {
	isWildCard := pattern[0] == wildcardParam
	isPlusParam := pattern[0] == plusParam
	parameterEndPosition := findNextNonEscapedCharsetPosition(pattern[1:], parameterEndChars)
	parameterConstraintStart := -1
	parameterConstraintEnd := -1

	// handle wildcard end
	if isWildCard || isPlusParam {
		parameterEndPosition = 0
	} else if parameterEndPosition == -1 {
		parameterEndPosition = len(pattern) - 1
	} else if !isInCharset(pattern[parameterEndPosition+1], parameterDelimiterChars) {
		parameterEndPosition++
	}

	// find constraint part if exists in the parameter part and remove it
	if parameterEndPosition > 0 {
		parameterConstraintStart = findNextNonEscapedCharsetPosition(pattern[0:parameterEndPosition], parameterConstraintStartChars)
		parameterConstraintEnd = findNextNonEscapedCharsetPosition(pattern[0:parameterEndPosition+1], parameterConstraintEndChars)

	}

	// cut params part
	processedPart := pattern[0 : parameterEndPosition+1]
	paramName := RemoveEscapeChar(GetTrimmedParam(processedPart))

	// Check has constraint
	var constraint *Constraint

	if hasConstraint := (parameterConstraintStart != -1 && parameterConstraintEnd != -1); hasConstraint {
		constraintString := pattern[parameterConstraintStart+1 : parameterConstraintEnd]

		start := 0
		end := 0
		haveData := false
		for k, v := range constraintString {
			if v == rune('(') {
				start = k + 1
				continue
			}

			if v == rune(')') && start != 0 {
				end = k
				haveData = true
				break
			}
		}

		constraint = &Constraint{
			ID: getParamConstraintType(constraintString[:start-1]),
		}

		if haveData {
			constraint.Data = strings.Split(constraintString[start:end], ",")
		}

		paramName = RemoveEscapeChar(GetTrimmedParam(pattern[0:parameterConstraintStart]))
	}

	// add access iterator to wildcard and plus
	if isWildCard {
		routeParser.wildCardCount++
		paramName += strconv.Itoa(routeParser.wildCardCount)
	} else if isPlusParam {
		routeParser.plusCount++
		paramName += strconv.Itoa(routeParser.plusCount)
	}

	segment := &routeSegment{
		ParamName:  paramName,
		IsParam:    true,
		IsOptional: isWildCard || pattern[parameterEndPosition] == optionalParam,
		IsGreedy:   isWildCard || isPlusParam,
	}

	if constraint != nil {
		segment.Constraint = constraint
	}

	return processedPart, segment
}

func getParamConstraintType(constraintPart string) TypeConstraint {
	switch constraintPart {

	case "int":
		return intConstraint
	case "bool":
		return boolConstraint
	case "float":
		return floatConstraint
	case "datetime":
		return datetimeConstraint
	case "alpha":
		return alphaConstraint
	case "guid":
		return guidConstraint
	case "minlength":
		return minLengthConstraint
	case "maxlength":
		return maxLengthConstraint
	case "exactlength":
		return exactLengthConstraint
	case "betweenlength":
		return BetweenLengthConstraint
	case "min":
		return minConstraint
	case "max":
		return maxConstraint
	case "range":
		return rangeConstraint
	case "regex":
		return regexConstraint
	default:
		return noConstraint

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

// findNextNonEscapedCharsetPosition search the next char position from the charset and skip the escaped characters
func findNextNonEscapedCharsetPosition(search string, charset []byte) int {
	pos := findNextCharsetPosition(search, charset)
	for pos > 0 && search[pos-1] == escapeChar {
		if len(search) == pos+1 {
			// escaped character is at the end
			return -1
		}
		nextPossiblePos := findNextCharsetPosition(search[pos+1:], charset)
		if nextPossiblePos == -1 {
			return -1
		}
		// the previous character is taken into consideration
		pos = nextPossiblePos + pos + 1
	}

	return pos
}

// getMatch parses the passed url and tries to match it against the route segments and determine the parameter positions
func (routeParser *routeParser) getMatch(detectionPath, path string, params *[maxParams]string, partialCheck bool) bool {
	var i, paramsIterator, partLen int
	for _, segment := range routeParser.segs {
		partLen = len(detectionPath)
		// check const segment
		if !segment.IsParam {
			i = segment.Length
			// is optional part or the const part must match with the given string
			// check if the end of the segment is a optional slash
			if segment.HasOptionalSlash && partLen == i-1 && detectionPath == segment.Const[:i-1] {
				i--
			} else if !(i <= partLen && detectionPath[:i] == segment.Const) {
				return false
			}
		} else {
			// determine parameter length
			i = findParamLen(detectionPath, segment)
			if !segment.IsOptional && i == 0 {
				return false
			}
			// take over the params positions
			params[paramsIterator] = path[:i]

			// check constraint
			if matched := segment.Constraint.CheckConstraint(params[paramsIterator]); !matched {
				return false
			}

			paramsIterator++
		}

		// reduce founded part from the string
		if partLen > 0 {
			detectionPath, path = detectionPath[i:], path[i:]
		}
	}
	if detectionPath != "" && !partialCheck {
		return false
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
		if constPosition := strings.LastIndex(s, segment.ComparePart); constPosition != -1 {
			s = s[:constPosition]
		} else {
			break
		}
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

// RemoveEscapeChar remove escape characters
func RemoveEscapeChar(word string) string {
	if strings.IndexByte(word, escapeChar) != -1 {
		return strings.ReplaceAll(word, string(escapeChar), "")
	}
	return word
}

func (c *Constraint) CheckConstraint(param string) bool {
	var err error
	var num int

	switch c.ID {
	case intConstraint:
		_, err = strconv.Atoi(param)
	case boolConstraint:
		_, err = strconv.ParseBool(param)
	case floatConstraint:
		_, err = strconv.ParseFloat(param, 32)
	case alphaConstraint:
		for _, r := range param {
			if !unicode.IsLetter(r) {
				err = errors.New("")
			}
		}
	case guidConstraint:
		_, err = uuid.Parse(param)
	case minLengthConstraint:
		data, _ := strconv.Atoi(c.Data[0])

		if len(param) < data {
			err = errors.New("")
		}
	case maxLengthConstraint:
		data, _ := strconv.Atoi(c.Data[0])

		if len(param) > data {
			err = errors.New("")
		}
	case exactLengthConstraint:
		data, _ := strconv.Atoi(c.Data[0])

		if len(param) != data {
			err = errors.New("")
		}
	case BetweenLengthConstraint:
		data, _ := strconv.Atoi(c.Data[0])
		data2, _ := strconv.Atoi(c.Data[1])
		length := len(param)

		if !(length >= data && length <= data2) {
			err = errors.New("")
		}
	case minConstraint:
		data, _ := strconv.Atoi(c.Data[0])
		num, err = strconv.Atoi(param)

		if num < data {
			err = errors.New("")
		}
	case maxConstraint:
		data, _ := strconv.Atoi(c.Data[0])
		num, err = strconv.Atoi(param)

		if num > data {
			err = errors.New("")
		}
	case rangeConstraint:
		data, _ := strconv.Atoi(c.Data[0])
		data2, _ := strconv.Atoi(c.Data[1])
		num, err = strconv.Atoi(param)

		if !(num >= data && num <= data2) {
			err = errors.New("")
		}
	case datetimeConstraint:
		_, err = time.Parse(param, c.Data[0])
	case regexConstraint:
		match, _ := regexp.MatchString(c.Data[0], param)
		if !match {
			err = errors.New("")
		}
	}

	return err == nil
}
