package minify

import (
	"bufio"
	"bytes"
	"io"
)

const eof = -1

// returns a minified script or an error.
func jsMinify(script []byte) (minified []byte, err error) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	r := bufio.NewReader(bytes.NewReader(script))

	m := new(jsminifier)
	m.init(r, w)
	m.run()
	if m.err != nil {
		return script, err
	}
	w.Flush()

	minified = buf.Bytes()
	if len(minified) > 0 && minified[0] == '\n' {
		minified = minified[1:]
	}

	return minified, nil
}

type jsminifier struct {
	r            *bufio.Reader
	w            *bufio.Writer
	theA         int
	theB         int
	theLookahead int
	theX         int
	theY         int
	err          error
}

func (m *jsminifier) init(r *bufio.Reader, w *bufio.Writer) {
	m.r = r
	m.w = w
	m.theLookahead = eof
	m.theX = eof
	m.theY = eof
}

func (m *jsminifier) error(s string) error {
	return m.err
}

// return true if the character is a letter, digit, underscore, dollar sign, or non-ASCII character.
func isAlphanum(c int) bool {
	return ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') || c == '_' || c == '$' || c == '\\' ||
		c > 126)
}

// return the next character from stdin. Watch out for lookahead.
// If the character is a control character, translate it to a space or linefeed.
func (m *jsminifier) get() int {
	c := m.theLookahead
	m.theLookahead = eof
	if c == eof {
		b, err := m.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				c = eof
			} else {
				m.error(err.Error())
				return eof
			}
		} else {
			c = int(b)
		}
	}
	if c >= ' ' || c == '\n' || c == eof {
		return c
	}
	if c == '\r' {
		return '\n'
	}
	return ' '
}

// get the next character without getting it.
func (m *jsminifier) peek() int {
	m.theLookahead = m.get()
	return m.theLookahead
}

// get the next character, excluding comments. peek() is used to see if a '/' is followed by a '/' or '*'.
func (m *jsminifier) next() int {
	c := m.get()
	if c == '/' {
		switch m.peek() {
		case '/':
			for {
				c = m.get()
				if c <= '\n' {
					break
				}
			}
		case '*':
			m.get()
			// Preserve license comments (/*!)
			if m.peek() == '!' {
				m.get()
				m.putc('/')
				m.putc('*')
				m.putc('!')
				for c != 0 {
					c = m.get()
					switch c {
					case '*':
						if m.peek() == '/' {
							m.get()
							c = 0
						} else {
							m.putc(c)
						}
					case eof:
						m.error("Unterminated comment.")
						return eof
					default:
						m.putc(c)
					}
				}
				m.putc('*')
				m.putc('/')
				c = '\n'
				break
			}
			// --
			for c != ' ' {
				switch m.get() {
				case '*':
					if m.peek() == '/' {
						m.get()
						c = ' '
					}
				case eof:
					m.error("Unterminated comment.")
					return eof
				}
			}
		}
	}
	m.theY = m.theX
	m.theX = c
	return c
}

func (m *jsminifier) putc(c int) {
	m.w.WriteByte(byte(c))
}

func (m *jsminifier) action(d int) {
	switch d {
	case 1:
		m.putc(m.theA)
		if (m.theY == '\n' || m.theY == ' ') &&
			(m.theA == '+' || m.theA == '-' || m.theA == '*' || m.theA == '/') &&
			(m.theB == '+' || m.theB == '-' || m.theB == '*' || m.theB == '/') {
			m.putc(m.theY)
		}
		fallthrough
	case 2:
		m.theA = m.theB
		if m.theA == '\'' || m.theA == '"' || m.theA == '`' {
			for {
				m.putc(m.theA)
				m.theA = m.get()
				if m.theA == m.theB {
					break
				}
				if m.theA == '\\' {
					m.putc(m.theA)
					m.theA = m.get()
				}
				if m.theA == eof {
					m.error("Unterminated string literal.")
					// return
					break
				}
			}
		}
		fallthrough
	case 3:
		m.theB = m.next()
		if m.theB == '/' && (m.theA == '(' || m.theA == ',' || m.theA == '=' || m.theA == ':' ||
			m.theA == '[' || m.theA == '!' || m.theA == '&' || m.theA == '|' ||
			m.theA == '?' || m.theA == '+' || m.theA == '-' || m.theA == '~' ||
			m.theA == '*' || m.theA == '/' || m.theA == '{' || m.theA == '\n') {
			m.putc(m.theA)
			if m.theA == '/' || m.theA == '*' {
				m.putc(' ')
			}
			m.putc(m.theB)
			for {
				m.theA = m.get()
				if m.theA == '[' {
					for {
						m.putc(m.theA)
						m.theA = m.get()
						if m.theA == ']' {
							break
						}
						if m.theA == '\\' {
							m.putc(m.theA)
							m.theA = m.get()
						}
						if m.theA == eof {
							m.error("Unterminated set in Regular Expression literal.")
							// return
						}
					}
				} else if m.theA == '/' {
					switch m.peek() {
					case '/', '*':
						m.error("Unterminated set in Regular Expression literal.")
						// return
					}
					break
				} else if m.theA == '\\' {
					m.putc(m.theA)
					m.theA = m.get()
				}
				if m.theA == eof {
					m.error("Unterminated Regular Expression literal.")
					return
				}
				m.putc(m.theA)
			}
			m.theB = m.next()
		}
	}
}

// Clean code: remove comments, tabs, newlines.
func (m *jsminifier) run() {
	if m.peek() == 0xEF {
		m.get()
		m.get()
		m.get()
	}
	m.theA = '\n'
	m.action(3)
	for m.theA != eof {
		switch m.theA {
		case ' ':
			if isAlphanum(m.theB) {
				m.action(1)
			} else {
				m.action(2)
			}
		case '\n':
			switch m.theB {
			case '{', '[', '(', '+', '-', '!', '~':
				m.action(1)
			case ' ':
				m.action(3)
			default:
				if isAlphanum(m.theB) {
					m.action(1)
				} else {
					m.action(2)
				}
			}
		default:
			switch m.theB {
			case ' ':
				if isAlphanum(m.theA) {
					m.action(1)
				} else {
					m.action(3)
				}
			case '\n':
				switch m.theA {
				case '}', ']', ')', '+', '-', '"', '\'', '`':
					m.action(1)
				default:
					if isAlphanum(m.theA) {
						m.action(1)
					} else {
						m.action(3)
					}
				}
			default:
				m.action(1)
			}
		}
	}
}
