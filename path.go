// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was inspired by ucarion/urlpath (MIT License).
// 💖 Maintained and modified for Fiber by @renewerner87

package fiber

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	utils "github.com/gofiber/utils"
)

// routeParser  holds the path segments and param names
type routeParser struct {
	segs   []paramSeg
	params []string
}

// paramsSeg holds the segment metadata
type paramSeg struct {
	Param          string
	Const          string
	IsParam        bool
	IsOptional     bool
	IsLast         bool
	IsRegexp       bool           // for `Const` segment, it might be a regular expression
	CompiledRegexp *regexp.Regexp // compiled regular expression if `IsRegexp = true`
	EndChar        byte
}

// list of possible parameter and segment delimiter
// slash has a special role, unlike the other parameters it must not be interpreted as a parameter
var routeDelimiters string = "/-."
var regexpCharacters string = "?+*()"

const wildcardParam string = "*"

// parseRoute analyzes the route and divides it into segments for constant areas and parameters,
// this information is needed later when assigning the requests to the declared routes
func parseRoute(pattern string) (p routeParser) {
	var out []paramSeg
	var params []string

	part, delimiterPos := "", 0
	for len(pattern) > 0 && delimiterPos != -1 {
		delimiterPos = findNextRouteSegmentEnd(pattern)
		if delimiterPos != -1 {
			part = pattern[:delimiterPos]
		} else {
			part = pattern
		}

		partLen, lastSeg := len(part), len(out)-1
		if partLen == 0 { // skip empty parts
			if len(pattern) > 0 {
				// remove first char
				pattern = pattern[1:]
			}
			continue
		}
		// is parameter ?
		if part[0] == '*' || part[0] == ':' {
			out = append(out, paramSeg{
				Param:      utils.GetTrimmedParam(part),
				IsParam:    true,
				IsOptional: part == wildcardParam || part[partLen-1] == '?',
			})
			lastSeg = len(out) - 1
			params = append(params, out[lastSeg].Param)
			// combine const segments
		} else if lastSeg >= 0 && !out[lastSeg].IsParam {
			out[lastSeg].Const += string(out[lastSeg].EndChar) + part
			if !out[lastSeg].IsRegexp && isSegRegexp(part) {
				out[lastSeg].IsRegexp = true
			}
			// create new const segment
		} else {
			out = append(out, paramSeg{
				Const: part,
			})
			lastSeg = len(out) - 1
			if isSegRegexp(part) {
				out[lastSeg].IsRegexp = true
			}
		}

		// only check if the segment is regexp for non param segments
		if !out[lastSeg].IsParam && out[lastSeg].IsRegexp {
			re, err := buildSegRegexp(out[lastSeg].Const)
			if err != nil {
				out[lastSeg].IsRegexp = false
				// If there is an error when compiling the regexp, we log an
				// error and treat it like a normal Const segment instead of
				// panicing.
				fmt.Printf("invalid regexp route path: %v, %v\n", out[lastSeg].Const, err)
			} else {
				out[lastSeg].CompiledRegexp = re
			}
		}

		if delimiterPos != -1 && len(pattern) >= delimiterPos+1 {
			out[lastSeg].EndChar = pattern[delimiterPos]
			pattern = pattern[delimiterPos+1:]
		} else {
			// last default char
			out[lastSeg].EndChar = '/'
		}
	}
	if len(out) > 0 {
		out[len(out)-1].IsLast = true
	}

	p = routeParser{segs: out, params: params}
	return
}

// findNextRouteSegmentEnd searches in the route for the next end position for a segment
func findNextRouteSegmentEnd(search string) int {
	return strings.IndexAny(search, routeDelimiters)
}

// isSegRegexp checks if the segment contains regexp chars in `regexpCharacters`.
// Note that `regexpCharacters` also includes `wildcardParam`, but only `Constant`
// segment (isParam != true) uses this method. So as long as it has `*`, it will
// always be part of the regular expression but not the wildcard.
func isSegRegexp(seg string) bool {
	return strings.ContainsAny(seg, regexpCharacters)
}

/// buildSegRegexp builds the Regexp based on the segment string.
func buildSegRegexp(seg string) (*regexp.Regexp, error) {
	// `\A` is added because we need the match from the beginning
	//  `*` is replaced with `.*` because we need to turn the wildcard to a regular expression.
	return regexp.Compile("\\A" + strings.Replace(seg, "*", ".*", -1))
}

// getMatch parses the passed url and tries to match it against the route segments and determine the parameter positions
func (p *routeParser) getMatch(s string, partialCheck bool) ([][2]int, bool) {
	lenKeys := len(p.params)
	paramsPositions := getAllocFreeParamsPos(lenKeys)
	var i, j, paramsIterator, partLen, paramStart int
	if len(s) > 0 {
		s = s[1:]
		paramStart++
	}
	for index, segment := range p.segs {
		partLen = len(s)
		// check parameter
		if segment.IsParam {
			// determine parameter length
			if segment.Param == wildcardParam {
				if segment.IsLast {
					i = partLen
				} else {
					i = findWildcardParamLen(s, p.segs, index)
				}
			} else {
				i = strings.IndexByte(s, segment.EndChar)
			}
			if i == -1 {
				i = partLen
			}

			if !segment.IsOptional && i == 0 {
				return nil, false
				// special case for not slash end character
			} else if i > 0 && partLen >= i && segment.EndChar != '/' && s[i-1] == '/' {
				return nil, false
			}

			paramsPositions[paramsIterator][0], paramsPositions[paramsIterator][1] = paramStart, paramStart+i
			paramsIterator++
		} else {
			// check const segment
			if !segment.IsRegexp {
				i = len(segment.Const)
				if partLen < i || (i == 0 && partLen > 0) || s[:i] != segment.Const || (partLen > i && s[i] != segment.EndChar) {
					return nil, false
				}
			} else {
				loc := segment.CompiledRegexp.FindStringIndex(s)
				if loc == nil || loc[0] != 0 {
					return nil, false
				}

				if partLen >= loc[1] {
					if segment.IsLast {
						// remaining of `s` after regexp match has to be `/` when
						// the current segment is the last one.
						if segment.EndChar != '/' || (partLen > loc[1] && s[loc[1]:] != "/") {
							return nil, false
						}

						i = partLen
						// segment is not the last, but there is no `s` left.
					} else if partLen == loc[1] {
						return nil, false
						// partLen > loc[1] and the current segment is not last
					} else {
						if s[loc[1]] != segment.EndChar {
							return nil, false
						}
						i = loc[1]
					}
				} else {
					if s[loc[1]] != segment.EndChar {
						return nil, false
					}

					i = loc[1]
				}
			}
		}

		// reduce founded part from the string
		if partLen > 0 {
			j = i + 1
			if segment.IsLast || partLen < j {
				j = i
			}
			paramStart += j

			s = s[j:]
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

// findWildcardParamLen for the expressjs wildcard behavior (right to left greedy)
// look at the other segments and take what is left for the wildcard from right to left
func findWildcardParamLen(s string, segments []paramSeg, currIndex int) int {
	// "/api/*/:param" - "/api/joker/batman/robin/1" -> "joker/batman/robin", "1"
	// "/api/*/:param" - "/api/joker/batman"         -> "joker", "batman"
	// "/api/*/:param" - "/api/joker-batman-robin/1" -> "joker-batman-robin", "1"
	endChar := segments[currIndex].EndChar
	neededEndChars := 0
	// count the needed chars for the other segments
	for i := currIndex + 1; i < len(segments); i++ {
		if segments[i].EndChar == endChar {
			neededEndChars++
		}
	}
	// remove the part the other segments still need
	for {
		pos := strings.LastIndexByte(s, endChar)
		if pos != -1 {
			s = s[:pos]
		}
		neededEndChars--
		if neededEndChars <= 0 || pos == -1 {
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
