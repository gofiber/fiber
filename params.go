// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was based on urlpath by @ucarion (MIT License).
// 💖 Modified for the Fiber router by @renanbastos93 & @renewerner87
// 🤖 ucarion/urlpath - renanbastos93/fastpath - renewerner87/fastpath

package fiber

import (
	"strings"
)

// paramsParser holds the path segments and param names
type parsedParams struct {
	Segs []paramSeg
	Keys []string
}

// paramsSeg holds the segment metadata
type paramSeg struct {
	Param      string
	Const      string
	IsParam    bool
	IsOptional bool
	IsLast     bool
}

var paramsDummy = make([]string, 100, 100)

// New ...
func parseParams(pattern string) (p parsedParams) {
<<<<<<< HEAD
	if pattern[0] != '/' {
		pattern = "/" + pattern
	}
=======
>>>>>>> upstream/master
	var patternCount int
	aPattern := []string{""}
	if pattern != "" {
		aPattern = strings.Split(pattern, "/")[1:] // every route starts with an "/"
	}
	patternCount = len(aPattern)

	var out = make([]paramSeg, patternCount)
	var params []string
	var segIndex int
	for i := 0; i < patternCount; i++ {
		partLen := len(aPattern[i])
		if partLen == 0 { // skip empty parts
			continue
		}
		// is parameter ?
		if aPattern[i][0] == '*' || aPattern[i][0] == ':' {
			out[segIndex] = paramSeg{
				Param:      paramTrimmer(aPattern[i]),
				IsParam:    true,
				IsOptional: aPattern[i] == "*" || aPattern[i][partLen-1] == '?',
			}
			params = append(params, out[segIndex].Param)
		} else {
			// combine const segments
			if segIndex > 0 && out[segIndex-1].IsParam == false {
				segIndex--
				out[segIndex].Const += "/" + aPattern[i]
				// create new const segment
			} else {
				out[segIndex] = paramSeg{
					Const: aPattern[i],
				}
			}
		}
		segIndex++
	}
	if segIndex == 0 {
		segIndex++
	}
	out[segIndex-1].IsLast = true

	p = parsedParams{Segs: out[:segIndex:segIndex], Keys: params}
	return
}

// Match ...
func (p *parsedParams) matchParams(s string) ([]string, bool) {
	lenKeys := len(p.Keys)
	params := paramsDummy[0:lenKeys:lenKeys]
	var i, j, paramsIterator, partLen int
	if len(s) > 0 {
		s = s[1:]
	}
	for index, segment := range p.Segs {
		partLen = len(s)
		// check parameter
		if segment.IsParam {
			// determine parameter length
			if segment.IsLast {
				i = partLen
			} else if segment.Param == "*" {
				// for the expressjs behavior -> "/api/*/:param" - "/api/joker/batman/robin/1" -> "joker/batman/robin", "1"
				i = findCharPos(s, '/', strings.Count(s, "/")-(len(p.Segs)-(index+1))+1)
			} else {
				i = strings.IndexByte(s, '/')
			}
			if i == -1 {
				i = partLen
			}

			if false == segment.IsOptional && i == 0 {
				return nil, false
			}

			params[paramsIterator] = s[:i]
			paramsIterator++
		} else {
			// check const segment
			i = len(segment.Const)
			if partLen < i || (i == 0 && partLen > 0) || s[:i] != segment.Const || (partLen > i && s[i] != '/') {
				return nil, false
			}
		}

		// reduce founded part from the string
		if partLen > 0 {
			j = i + 1
			if segment.IsLast || partLen < j {
				j = i
			}

			s = s[j:]
		}
	}

	return params, true
}

func paramTrimmer(param string) string {
	start := 0
	end := len(param)

	if param[start] != ':' { // is not a param
		return param
	}
	start++
	if param[end-1] == '?' { // is ?
		end--
	}

	return param[start:end]
}
func findCharPos(s string, char byte, matchCount int) int {
	if matchCount == 0 {
		matchCount = 1
	}
	endPos, pos := 0, 0
	for matchCount > 0 && pos != -1 {
		if pos > 0 {
			s = s[pos+1:]
			endPos++
		}
		pos = strings.IndexByte(s, char)
		endPos += pos
		matchCount--
	}
	return endPos
}
