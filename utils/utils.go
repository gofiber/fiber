// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"text/tabwriter"
	"unsafe"
)

const toLowerTable = "\x00\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-./0123456789:;<=>?@abcdefghijklmnopqrstuvwxyz[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~\u007f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"
const toUpperTable = "\x00\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`ABCDEFGHIJKLMNOPQRSTUVWXYZ{|}~\u007f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"

// Copyright © 2014, Roger Peppe
// github.com/rogpeppe/fastuuid
// All rights reserved.

var uuidSeed [24]byte
var uuidCounter uint64

func UUID() string {
	// Setup seed & counter once
	if uuidCounter <= 0 {
		if _, err := rand.Read(uuidSeed[:]); err != nil {
			return "00000000-0000-0000-0000-000000000000"
		}
		uuidCounter = binary.LittleEndian.Uint64(uuidSeed[:8])
	}
	// first 8 bytes differ, taking a slice of the first 16 bytes
	x := atomic.AddUint64(&uuidCounter, 1)
	uuid := uuidSeed
	binary.LittleEndian.PutUint64(uuid[:8], x)
	uuid[6], uuid[9] = uuid[9], uuid[6]

	// RFC4122 v4
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = uuid[8]&0x3f | 0x80

	// create UUID representation of the first 128 bits
	b := make([]byte, 36)
	hex.Encode(b[0:8], uuid[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], uuid[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], uuid[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], uuid[8:10])
	b[23] = '-'
	hex.Encode(b[24:], uuid[10:16])

	return GetString(b)
}

const (
	uByte = 1 << (10 * iota)
	uKilobyte
	uMegabyte
	uGigabyte
	uTerabyte
	uPetabyte
	uExabyte
)

// Copyright github.com/cloudfoundry/bytefmt
// ByteSize returns a human-readable byte string of the form 10M, 12.5K, and so forth.
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
func ByteSize(bytes uint64) string {
	unit := ""
	value := float64(bytes)
	switch {
	case bytes >= uExabyte:
		unit = "E"
		value = value / uExabyte
	case bytes >= uPetabyte:
		unit = "P"
		value = value / uPetabyte
	case bytes >= uTerabyte:
		unit = "T"
		value = value / uTerabyte
	case bytes >= uGigabyte:
		unit = "G"
		value = value / uGigabyte
	case bytes >= uMegabyte:
		unit = "M"
		value = value / uMegabyte
	case bytes >= uKilobyte:
		unit = "K"
		value = value / uKilobyte
	case bytes >= uByte:
		unit = "B"
	default:
		return "0B"
	}
	result := strconv.FormatFloat(value, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

// Returns function name
func FunctionName(fn interface{}) string {
	t := reflect.ValueOf(fn).Type()
	if t.Kind() == reflect.Func {
		return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	}
	return t.String()
}

// ToLower is the equivalent of strings.ToLower
func ToLower(b string) string {
	var res = make([]byte, len(b))
	copy(res, b)
	for i := 0; i < len(res); i++ {
		res[i] = toLowerTable[res[i]]
	}

	return GetString(res)
}

// ToLowerBytes is the equivalent of bytes.ToLower
func ToLowerBytes(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		b[i] = toLowerTable[b[i]]
	}
	return b
}

// ToUpper is the equivalent of strings.ToUpper
func ToUpper(b string) string {
	var res = make([]byte, len(b))
	copy(res, b)
	for i := 0; i < len(res); i++ {
		res[i] = toUpperTable[res[i]]
	}

	return GetString(res)
}

// ToUpperBytes is the equivalent of bytes.ToUpper
func ToUpperBytes(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		b[i] = toUpperTable[b[i]]
	}
	return b
}

// TrimRight is the equivalent of strings.TrimRight
func TrimRight(s string, cutset byte) string {
	lenStr := len(s)
	for lenStr > 0 && s[lenStr-1] == cutset {
		lenStr--
	}
	return s[:lenStr]
}

// TrimRightBytes is the equivalent of bytes.TrimRight
func TrimRightBytes(b []byte, cutset byte) []byte {
	lenStr := len(b)
	for lenStr > 0 && b[lenStr-1] == cutset {
		lenStr--
	}
	return b[:lenStr]
}

// TrimLeft is the equivalent of strings.TrimLeft
func TrimLeft(s string, cutset byte) string {
	lenStr, start := len(s), 0
	for start < lenStr && s[start] == cutset {
		start++
	}
	return s[start:]
}

// TrimLeftBytes is the equivalent of bytes.TrimLeft
func TrimLeftBytes(b []byte, cutset byte) []byte {
	lenStr, start := len(b), 0
	for start < lenStr && b[start] == cutset {
		start++
	}
	return b[start:]
}

// Trim is the equivalent of strings.Trim
func Trim(s string, cutset byte) string {
	i, j := 0, len(s)-1
	for ; i < j; i++ {
		if s[i] != cutset {
			break
		}
	}
	for ; i < j; j-- {
		if s[j] != cutset {
			break
		}
	}

	return s[i : j+1]
}

// TrimBytes is the equivalent of bytes.Trim
func TrimBytes(b []byte, cutset byte) []byte {
	i, j := 0, len(b)-1
	for ; i < j; i++ {
		if b[i] != cutset {
			break
		}
	}
	for ; i < j; j-- {
		if b[j] != cutset {
			break
		}
	}

	return b[i : j+1]
}

// EqualFold the equivalent of bytes.EqualFold
func EqualsFold(b, s []byte) (equals bool) {
	n := len(b)
	equals = n == len(s)
	if equals {
		for i := 0; i < n; i++ {
			if equals = b[i]|0x20 == s[i]|0x20; !equals {
				break
			}
		}
	}
	return
}

// #nosec G103
// GetString returns a string pointer without allocation
func GetString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// #nosec G103
// GetBytes returns a byte pointer without allocation
func GetBytes(s string) (bs []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&bs))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return
}

// ImmutableString returns a immutable string with allocation
func ImmutableString(s string) string {
	return string(GetBytes(s))
}

// AssertEqual checks if values are equal
func AssertEqual(t testing.TB, expected interface{}, actual interface{}, description ...string) {
	if reflect.DeepEqual(expected, actual) {
		return
	}
	var aType = "<nil>"
	var bType = "<nil>"
	if reflect.ValueOf(expected).IsValid() {
		aType = reflect.TypeOf(expected).Name()
	}
	if reflect.ValueOf(actual).IsValid() {
		bType = reflect.TypeOf(actual).Name()
	}

	testName := "AssertEqual"
	if t != nil {
		testName = t.Name()
	}

	_, file, line, _ := runtime.Caller(1)

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 5, ' ', 0)
	fmt.Fprintf(w, "\nTest:\t%s", testName)
	fmt.Fprintf(w, "\nTrace:\t%s:%d", filepath.Base(file), line)
	fmt.Fprintf(w, "\nError:\tNot equal")
	fmt.Fprintf(w, "\nExpect:\t%v\t[%s]", expected, aType)
	fmt.Fprintf(w, "\nResult:\t%v\t[%s]", actual, bType)

	if len(description) > 0 {
		fmt.Fprintf(w, "\nDescription:\t%s", description[0])
	}

	result := ""
	if err := w.Flush(); err != nil {
		result = err.Error()
	} else {
		result = buf.String()
	}
	if t != nil {
		t.Fatal(result)
	} else {
		log.Fatal(result)
	}
}

const MIMEOctetStream = "application/octet-stream"

// GetMIME returns the content-type of a file extension
func GetMIME(extension string) (mime string) {
	if len(extension) == 0 {
		return mime
	}
	if extension[0] == '.' {
		mime = mimeExtensions[extension[1:]]
	} else {
		mime = mimeExtensions[extension]
	}
	if len(mime) == 0 {
		return MIMEOctetStream
	}
	return mime
}

// GetTrimmedParam trims the ':' & '?' from a string
func GetTrimmedParam(param string) string {
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

// GetCharPos ...
func GetCharPos(s string, char byte, matchCount int) int {
	if matchCount == 0 {
		matchCount = 1
	}
	endPos, pos := 0, -2
	for matchCount > 0 && pos != -1 {
		if pos > -1 {
			s = s[pos+1:]
			endPos++
		}
		pos = strings.IndexByte(s, char)
		endPos += pos
		matchCount--
	}
	return endPos
}

// GetArgument check if key is in arguments
func GetArgument(arg string) bool {
	for i := range os.Args[1:] {
		if os.Args[1:][i] == arg {
			return true
		}
	}
	return false
}

// limit for the access
const (
	statusMessageMin = 100
	statusMessageMax = 511
)

// StatusMessage returns the correct message for the provided HTTP statuscode
func StatusMessage(status int) string {
	if status < statusMessageMin || status > statusMessageMax {
		return ""
	}
	return statusMessage[status]
}

// HTTP status codes were copied from net/http.
var statusMessage = []string{
	100: "Continue",
	101: "Switching Protocols",
	102: "Processing",
	103: "Early Hints",
	200: "OK",
	201: "Created",
	202: "Accepted",
	203: "Non-Authoritative Information",
	204: "No Content",
	205: "Reset Content",
	206: "Partial Content",
	207: "Multi-Status",
	208: "Already Reported",
	226: "IM Used",
	300: "Multiple Choices",
	301: "Moved Permanently",
	302: "Found",
	303: "See Other",
	304: "Not Modified",
	305: "Use Proxy",
	306: "Switch Proxy",
	307: "Temporary Redirect",
	308: "Permanent Redirect",
	400: "Bad Request",
	401: "Unauthorized",
	402: "Payment Required",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	406: "Not Acceptable",
	407: "Proxy Authentication Required",
	408: "Request Timeout",
	409: "Conflict",
	410: "Gone",
	411: "Length Required",
	412: "Precondition Failed",
	413: "Request Entity Too Large",
	414: "Request URI Too Long",
	415: "Unsupported Media Type",
	416: "Requested Range Not Satisfiable",
	417: "Expectation Failed",
	418: "I'm a teapot",
	421: "Misdirected Request",
	422: "Unprocessable Entity",
	423: "Locked",
	424: "Failed Dependency",
	426: "Upgrade Required",
	428: "Precondition Required",
	429: "Too Many Requests",
	431: "Request Header Fields Too Large",
	451: "Unavailable For Legal Reasons",
	500: "Internal Server Error",
	501: "Not Implemented",
	502: "Bad Gateway",
	503: "Service Unavailable",
	504: "Gateway Timeout",
	505: "HTTP Version Not Supported",
	506: "Variant Also Negotiates",
	507: "Insufficient Storage",
	508: "Loop Detected",
	510: "Not Extended",
	511: "Network Authentication Required",
}

// MIME types were copied from https://github.com/nginx/nginx/blob/master/conf/mime.types
var mimeExtensions = map[string]string{
	"html":    "text/html",
	"htm":     "text/html",
	"shtml":   "text/html",
	"css":     "text/css",
	"gif":     "image/gif",
	"jpeg":    "image/jpeg",
	"jpg":     "image/jpeg",
	"xml":     "application/xml",
	"js":      "application/javascript",
	"atom":    "application/atom+xml",
	"rss":     "application/rss+xml",
	"mml":     "text/mathml",
	"txt":     "text/plain",
	"jad":     "text/vnd.sun.j2me.app-descriptor",
	"wml":     "text/vnd.wap.wml",
	"htc":     "text/x-component",
	"png":     "image/png",
	"svg":     "image/svg+xml",
	"svgz":    "image/svg+xml",
	"tif":     "image/tiff",
	"tiff":    "image/tiff",
	"wbmp":    "image/vnd.wap.wbmp",
	"webp":    "image/webp",
	"ico":     "image/x-icon",
	"jng":     "image/x-jng",
	"bmp":     "image/x-ms-bmp",
	"woff":    "font/woff",
	"woff2":   "font/woff2",
	"jar":     "application/java-archive",
	"war":     "application/java-archive",
	"ear":     "application/java-archive",
	"json":    "application/json",
	"hqx":     "application/mac-binhex40",
	"doc":     "application/msword",
	"pdf":     "application/pdf",
	"ps":      "application/postscript",
	"eps":     "application/postscript",
	"ai":      "application/postscript",
	"rtf":     "application/rtf",
	"m3u8":    "application/vnd.apple.mpegurl",
	"kml":     "application/vnd.google-earth.kml+xml",
	"kmz":     "application/vnd.google-earth.kmz",
	"xls":     "application/vnd.ms-excel",
	"eot":     "application/vnd.ms-fontobject",
	"ppt":     "application/vnd.ms-powerpoint",
	"odg":     "application/vnd.oasis.opendocument.graphics",
	"odp":     "application/vnd.oasis.opendocument.presentation",
	"ods":     "application/vnd.oasis.opendocument.spreadsheet",
	"odt":     "application/vnd.oasis.opendocument.text",
	"wmlc":    "application/vnd.wap.wmlc",
	"7z":      "application/x-7z-compressed",
	"cco":     "application/x-cocoa",
	"jardiff": "application/x-java-archive-diff",
	"jnlp":    "application/x-java-jnlp-file",
	"run":     "application/x-makeself",
	"pl":      "application/x-perl",
	"pm":      "application/x-perl",
	"prc":     "application/x-pilot",
	"pdb":     "application/x-pilot",
	"rar":     "application/x-rar-compressed",
	"rpm":     "application/x-redhat-package-manager",
	"sea":     "application/x-sea",
	"swf":     "application/x-shockwave-flash",
	"sit":     "application/x-stuffit",
	"tcl":     "application/x-tcl",
	"tk":      "application/x-tcl",
	"der":     "application/x-x509-ca-cert",
	"pem":     "application/x-x509-ca-cert",
	"crt":     "application/x-x509-ca-cert",
	"xpi":     "application/x-xpinstall",
	"xhtml":   "application/xhtml+xml",
	"xspf":    "application/xspf+xml",
	"zip":     "application/zip",
	"bin":     "application/octet-stream",
	"exe":     "application/octet-stream",
	"dll":     "application/octet-stream",
	"deb":     "application/octet-stream",
	"dmg":     "application/octet-stream",
	"iso":     "application/octet-stream",
	"img":     "application/octet-stream",
	"msi":     "application/octet-stream",
	"msp":     "application/octet-stream",
	"msm":     "application/octet-stream",
	"mid":     "audio/midi",
	"midi":    "audio/midi",
	"kar":     "audio/midi",
	"mp3":     "audio/mpeg",
	"ogg":     "audio/ogg",
	"m4a":     "audio/x-m4a",
	"ra":      "audio/x-realaudio",
	"3gpp":    "video/3gpp",
	"3gp":     "video/3gpp",
	"ts":      "video/mp2t",
	"mp4":     "video/mp4",
	"mpeg":    "video/mpeg",
	"mpg":     "video/mpeg",
	"mov":     "video/quicktime",
	"webm":    "video/webm",
	"flv":     "video/x-flv",
	"m4v":     "video/x-m4v",
	"mng":     "video/x-mng",
	"asx":     "video/x-ms-asf",
	"asf":     "video/x-ms-asf",
	"wmv":     "video/x-ms-wmv",
	"avi":     "video/x-msvideo",
}
