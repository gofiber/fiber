package minify

import (
	"bytes"
	"io"

	"golang.org/x/net/html"
)

type Options struct {
	MinifyScripts bool
	MinifyStyles  bool
}

// Minify returns minified version of the given HTML data.
// If passed options is nil, uses default options.
func htmlMinify(data []byte, options *Options) (out []byte, err error) {

	var b bytes.Buffer
	z := html.NewTokenizer(bytes.NewReader(data))
	raw := 0
	javascript := false
	style := false
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				return b.Bytes(), nil
			}
			return nil, err
		case html.StartTagToken, html.SelfClosingTagToken:
			tagName, hasAttr := z.TagName()
			switch string(tagName) {
			case "script":
				javascript = true
				raw++
			case "style":
				style = true
				raw++
			case "pre", "code", "textarea":
				raw++
			}
			b.WriteByte('<')
			b.Write(tagName)
			var k, v []byte
			isFirst := true
			for hasAttr {
				k, v, hasAttr = z.TagAttr()
				if javascript && string(k) == "type" && string(v) != "text/javascript" {
					javascript = false
				}
				if string(k) == "style" && options.MinifyStyles {
					v = []byte("a{" + string(v) + "}") // simulate "full" CSS
					v = cssMinify(v)
					v = v[2 : len(v)-1] // strip simulation
				}
				if isFirst {
					b.WriteByte(' ')
					isFirst = false
				}
				b.Write(k)
				if len(v) > 0 || isAlt(k) {
					b.WriteByte('=')
					qv := html.EscapeString(string(v))
					// If the value is quoted with single quotes, replace them with double quotes.
					b.WriteByte('"')
					b.WriteString(qv)
					b.WriteByte('"')
				}
				if hasAttr {
					b.WriteByte(' ')
				}
			}
			b.WriteByte('>')
		case html.EndTagToken:
			tagName, _ := z.TagName()
			switch string(tagName) {
			case "script":
				javascript = false
				raw--
			case "style":
				style = false
				raw--
			case "pre", "code", "textarea":
				raw--
			}
			b.Write([]byte("</"))
			b.Write(tagName)
			b.WriteByte('>')
		case html.CommentToken:
			if bytes.HasPrefix(z.Raw(), []byte("<!--[if")) ||
				bytes.HasPrefix(z.Raw(), []byte("<!--//")) {
				// Preserve IE conditional and special style comments.
				b.Write(z.Raw())
			}
			// ... otherwise, skip.
		case html.TextToken:
			if javascript && options.MinifyScripts {
				min, err := jsMinify(z.Raw())
				if err != nil {
					// Just write it as is.
					b.Write(z.Raw())
				} else {
					b.Write(min)
				}
			} else if style && options.MinifyStyles {
				b.Write(cssMinify(z.Raw()))
			} else if raw > 0 {
				b.Write(z.Raw())
			} else {
				text := bytes.TrimSpace(z.Raw())
				if len(text) > 0 {
					b.Write(text)
				}
				// b.Write(trimTextToken(z.Raw()))
			}
		default:
			b.Write(z.Raw())
		}

	}
}

func isAlt(v []byte) bool {
	return len(v) == 3 && v[0] == 'a' && v[1] == 'l' && v[2] == 't'
}
