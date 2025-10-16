package console

import (
	"bytes"
	"io"
)

// nonColorable holds a writer but removes ANSI escape sequences before writing
// to the underlying destination.
type nonColorable struct {
	out io.Writer
}

// NonColorable returns a writer that strips ANSI escape sequences before
// forwarding bytes to the provided writer.
func NonColorable(w io.Writer) io.Writer {
	return &nonColorable{out: w}
}

// Write removes ANSI escape sequences from data before writing it to the
// underlying writer.
//
// Portions of this implementation are derived from github.com/mattn/go-colorable
// and remain covered by the MIT License.
func (w *nonColorable) Write(data []byte) (int, error) {
	er := bytes.NewReader(data)
	var plaintext bytes.Buffer

loop:
	for {
		c1, err := er.ReadByte()
		if err != nil {
			if plaintext.Len() > 0 {
				if _, writeErr := plaintext.WriteTo(w.out); writeErr != nil {
					return len(data) - er.Len(), writeErr
				}
			}
			break loop
		}
		if c1 != 0x1b {
			plaintext.WriteByte(c1)
			continue
		}
		if _, err = plaintext.WriteTo(w.out); err != nil {
			return len(data) - er.Len(), err
		}

		c2, err := er.ReadByte()
		if err != nil {
			break loop
		}
		if c2 != 0x5b {
			continue
		}

		for {
			c, err := er.ReadByte()
			if err != nil {
				break loop
			}
			if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '@' {
				break
			}
		}
	}

	return len(data), nil
}
