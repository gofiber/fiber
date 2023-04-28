package minify

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
)

// define regular expressions to match different patterns in CSS code
var (
	rcomments      = regexp.MustCompile(`\/\*[\s\S]*?\*\/`)
	rwhitespace    = regexp.MustCompile(`\s+`)
	runits         = regexp.MustCompile(`(?i)([\s:])([+-]?0)(?:%|em|ex|px|in|cm|mm|pt|pc)`)
	rfourzero      = regexp.MustCompile(`:(?:0 )+0;`)
	rleadzero      = regexp.MustCompile(`(:|\s)0+\.(\d+)`)
	rrgb           = regexp.MustCompile(`rgb\s*\(\s*([0-9,\s]+)\s*\)`)
	rbmh           = regexp.MustCompile(`"\\"\}\\""`)
	runspace1      = regexp.MustCompile(`(?:^|\})[^\{:]+\s+:+[^\{]*\{`)
	runspace2      = regexp.MustCompile(`\s+([!\{\};:>+\(\)\],])`)
	rcompresshex   = regexp.MustCompile(`(?i)([^"'=\s])(\s?)\s*#([0-9a-f]){6}`)
	rhexval        = regexp.MustCompile(`[0-9a-f]{2}`)
	remptyrules    = regexp.MustCompile(`[^\}]+\{;\}\n`)
	rmediaspace    = regexp.MustCompile(`\band\(`)
	rredsemicolons = regexp.MustCompile(`;+\}`)
	runspace3      = regexp.MustCompile(`([!\{\}:;>+\(\[,])\s+`)
	rsemicolons    = regexp.MustCompile(`([^;\}])\}`)
	rdigits        = regexp.MustCompile(`\d+`)
)

// cssMinify takes in a slice of bytes representing CSS code and returns a minified version of it.
func cssMinify(css []byte) (minified []byte) {
	// Remove // and /* */ CSS comments
	css = rcomments.ReplaceAll(css, []byte{})

	// replace whitespace with a single space character
	css = rwhitespace.ReplaceAll(css, []byte(" "))

	// Replace all occurrences of '}' with '___BMH___' (Block Marker Hash)
	css = rbmh.ReplaceAll(css, []byte("___BMH___"))

	// Replace all occurrences of ':' with '___PSEUDOCLASSCOLON___'
	css = runspace1.ReplaceAllFunc(css, func(match []byte) []byte {
		return bytes.Replace(match, []byte(":"), []byte("___PSEUDOCLASSCOLON___"), -1)
	})
	css = runspace2.ReplaceAll(css, []byte("$1"))
	css = bytes.Replace(css, []byte("___PSEUDOCLASSCOLON___"), []byte(":"), -1)

	// remove space after commas, colons, semicolons, brackets, etc.
	css = runspace3.ReplaceAll(css, []byte("$1"))

	// add missing semicolon
	css = rsemicolons.ReplaceAll(css, []byte("$1;}"))

	// remove leading zeros from integer values
	css = runits.ReplaceAll(css, []byte("$1$2"))

	// replace 0 0 0 0; with 0;
	css = rfourzero.ReplaceAll(css, []byte(":0;"))

	// replace background-position:0; with background-position:0 0;
	css = bytes.Replace(css, []byte("background-position:0;"), []byte("background-position:0 0;"), -1)

	// remove leading zeros from float values
	css = rleadzero.ReplaceAll(css, []byte("$1.$2"))

	// replace rgb(0,0,0) with #000
	css = rrgb.ReplaceAllFunc(css, func(match []byte) (out []byte) {
		out = []byte{'#'}
		for _, v := range rdigits.FindAll(match, -1) {
			d, err := strconv.Atoi(string(v))
			if err != nil {
				return match
			}
			out = append(out, []byte(fmt.Sprintf("%02x", d))...)
		}
		return out
	})
	// replace #aabbcc with #abc
	css = rcompresshex.ReplaceAllFunc(css, func(match []byte) (out []byte) {
		vals := rhexval.FindAll(match, -1)
		if len(vals) != 3 {
			return match
		}
		compressible := true
		for _, v := range vals {
			if v[0] != v[1] {
				compressible = false
			}
		}
		if !compressible {
			return match
		}
		out = append(out, match[:bytes.IndexByte(match, '#')+1]...)
		return append(out, vals[0][0], vals[1][0], vals[2][0])
	})

	// remove empty rules
	css = remptyrules.ReplaceAll(css, []byte{})

	// replace ___BMH___ with '}' (put back the removed closing brackets)
	css = bytes.Replace(css, []byte("___BMH___"), []byte(`"\"}\""`), -1)

	// replace 'and (' with 'and('
	css = rmediaspace.ReplaceAll(css, []byte("and ("))

	// remove trailing semicolons
	css = rredsemicolons.ReplaceAll(css, []byte("}"))

	return bytes.TrimSpace(css)
}
