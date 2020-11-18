package json

// Tokenizer is an iterator-style type which can be used to progressively parse
// through a json input.
//
// Tokenizing json is useful to build highly efficient parsing operations, for
// example when doing tranformations on-the-fly where as the program reads the
// input and produces the transformed json to an output buffer.
//
// Here is a common pattern to use a tokenizer:
//
//	for t := json.NewTokenizer(b); t.Next(); {
//		switch t.Delim {
//		case '{':
//			...
//		case '}':
//			...
//		case '[':
//			...
//		case ']':
//			...
//		case ':':
//			...
//		case ',':
//			...
//		}
//
//		switch {
//		case t.Value.String():
//			...
//		case t.Value.Null():
//			...
//		case t.Value.True():
//			...
//		case t.Value.False():
//			...
//		case t.Value.Number():
//			...
//		}
//	}
//
type Tokenizer struct {
	// When the tokenizer is positioned on a json delimiter this field is not
	// zero. In this case the possible values are '{', '}', '[', ']', ':', and
	// ','.
	Delim Delim

	// This field contains the raw json token that the tokenizer is pointing at.
	// When Delim is not zero, this field is a single-element byte slice
	// continaing the delimiter value. Otherwise, this field holds values like
	// null, true, false, numbers, or quoted strings.
	Value RawValue

	// When the tokenizer has encountered invalid content this field is not nil.
	Err error

	// When the value is in an array or an object, this field contains the depth
	// at which it was found.
	Depth int

	// When the value is in an array or an object, this field contains the
	// position at which it was found.
	Index int

	// This field is true when the value is the key of an object.
	IsKey bool

	// Tells whether the next value read from the tokenizer is a key.
	isKey bool

	// json input for the tokenizer, pointing at data right after the last token
	// that was parsed.
	json []byte

	// Stack used to track entering and leaving arrays, objects, and keys. The
	// buffer is used as a pre-allocated space to
	stack  []state
	buffer [8]state
}

type state struct {
	typ scope
	len int
}

type scope int

const (
	inArray scope = iota
	inObject
)

// NewTokenizer constructs a new Tokenizer which reads its json input from b.
func NewTokenizer(b []byte) *Tokenizer { return &Tokenizer{json: b} }

// Reset erases the state of t and re-initializes it with the json input from b.
func (t *Tokenizer) Reset(b []byte) {
	// This code is similar to:
	//
	//	*t = Tokenizer{json: b}
	//
	// However, it does not compile down to an invocation of duff-copy, which
	// ends up being slower and prevents the code from being inlined.
	t.Delim = 0
	t.Value = nil
	t.Err = nil
	t.Depth = 0
	t.Index = 0
	t.IsKey = false
	t.isKey = false
	t.json = b
	t.stack = nil
}

// Next returns a new tokenizer pointing at the next token, or the zero-value of
// Tokenizer if the end of the json input has been reached.
//
// If the tokenizer encounters malformed json while reading the input the method
// sets t.Err to an error describing the issue, and returns false. Once an error
// has been encountered, the tokenizer will always fail until its input is
// cleared by a call to its Reset method.
func (t *Tokenizer) Next() bool {
	if t.Err != nil {
		return false
	}

	// Inlined code of the skipSpaces function, this give a ~15% speed boost.
	i := 0
skipLoop:
	for _, c := range t.json {
		switch c {
		case sp, ht, nl, cr:
			i++
		default:
			break skipLoop
		}
	}

	if t.json = t.json[i:]; len(t.json) == 0 {
		t.Reset(nil)
		return false
	}

	var d Delim
	var v []byte
	var b []byte
	var err error

	switch t.json[0] {
	case '"':
		v, b, err = parseString(t.json)
	case 'n':
		v, b, err = parseNull(t.json)
	case 't':
		v, b, err = parseTrue(t.json)
	case 'f':
		v, b, err = parseFalse(t.json)
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		v, b, err = parseNumber(t.json)
	case '{', '}', '[', ']', ':', ',':
		d, v, b = Delim(t.json[0]), t.json[:1], t.json[1:]
	default:
		v, b, err = t.json[:1], t.json[1:], syntaxError(t.json, "expected token but found '%c'", t.json[0])
	}

	t.Delim = d
	t.Value = RawValue(v)
	t.Err = err
	t.Depth = t.depth()
	t.Index = t.index()
	t.IsKey = d == 0 && t.isKey
	t.json = b

	if d != 0 {
		switch d {
		case '{':
			t.isKey = true
			t.push(inObject)
		case '[':
			t.push(inArray)
		case '}':
			err = t.pop(inObject)
			t.Depth--
			t.Index = t.index()
		case ']':
			err = t.pop(inArray)
			t.Depth--
			t.Index = t.index()
		case ':':
			t.isKey = false
		case ',':
			if len(t.stack) == 0 {
				t.Err = syntaxError(t.json, "found unexpected comma")
				return false
			}
			if t.is(inObject) {
				t.isKey = true
			}
			t.stack[len(t.stack)-1].len++
		}
	}

	return (d != 0 || len(v) != 0) && err == nil
}

func (t *Tokenizer) push(typ scope) {
	if t.stack == nil {
		t.stack = t.buffer[:0]
	}
	t.stack = append(t.stack, state{typ: typ, len: 1})
}

func (t *Tokenizer) pop(expect scope) error {
	i := len(t.stack) - 1

	if i < 0 {
		return syntaxError(t.json, "found unexpected character while tokenizing json input")
	}

	if found := t.stack[i]; expect != found.typ {
		return syntaxError(t.json, "found unexpected character while tokenizing json input")
	}

	t.stack = t.stack[:i]
	return nil
}

func (t *Tokenizer) is(typ scope) bool {
	return len(t.stack) != 0 && t.stack[len(t.stack)-1].typ == typ
}

func (t *Tokenizer) depth() int {
	return len(t.stack)
}

func (t *Tokenizer) index() int {
	if len(t.stack) == 0 {
		return 0
	}
	return t.stack[len(t.stack)-1].len - 1
}

// RawValue represents a raw json value, it is intended to carry null, true,
// false, number, and string values only.
type RawValue []byte

// String returns true if v contains a string value.
func (v RawValue) String() bool { return len(v) != 0 && v[0] == '"' }

// Null returns true if v contains a null value.
func (v RawValue) Null() bool { return len(v) != 0 && v[0] == 'n' }

// True returns true if v contains a true value.
func (v RawValue) True() bool { return len(v) != 0 && v[0] == 't' }

// False returns true if v contains a false value.
func (v RawValue) False() bool { return len(v) != 0 && v[0] == 'f' }

// Number returns true if v contains a number value.
func (v RawValue) Number() bool {
	if len(v) != 0 {
		switch v[0] {
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return true
		}
	}
	return false
}

// AppendUnquote writes the unquoted version of the string value in v into b.
func (v RawValue) AppendUnquote(b []byte) []byte {
	s, r, new, err := parseStringUnquote([]byte(v), b)
	if err != nil {
		panic(err)
	}
	if len(r) != 0 {
		panic(syntaxError(r, "unexpected trailing tokens after json value"))
	}
	if new {
		b = s
	} else {
		b = append(b, s...)
	}
	return b
}

// Unquote returns the unquoted version of the string value in v.
func (v RawValue) Unquote() []byte {
	return v.AppendUnquote(nil)
}
