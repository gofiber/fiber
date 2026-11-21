// Package gobcheck reads a gob stream before encoding/gob does, and refuses one that could
// make the decoder do more work than the stream is long.
//
// encoding/gob says of itself that it is not hardened against input made to cause harm. The
// parts of it that matter here are these:
//
//   - A map is allocated for the number of entries the stream claims, before the entries are
//     read, and the entries of a map of structs are read in a loop that does not end when the
//     input does: a few bytes can ask for gigabytes of memory or for hours of CPU.
//   - Every level of nesting is a level of recursion, so a stream that is nested deeply enough
//     ends the process with a stack overflow, which is not an error that can be recovered.
//   - A field that the Go type lacks is skipped by going through the types the field is made of,
//     again for every field, so a struct of many fields of a type that is made of many others takes
//     a time and a memory that grow with the square of the stream: 22 KB took 0.4 s and 44 MB. The
//     way to skip a struct is compiled by calling the compiler for each struct that is a field of
//     it, so a chain of types as long as the stream takes a stack that the decoder does not have.
//   - The length that precedes the value of an interface is not read when the value is
//     decoded and is read when it is skipped, and for a value that holds a type definition it
//     is the length of its first part only. The two ways of reading a stream can then
//     disagree about where a value ends, and the counts that the decoder takes from a stream
//     it reads out of step are not the ones that were checked.
//
// Normalize reads the stream with the grammar of the decoder, which the standard library
// implements a second time in the debugging file of encoding/gob: every count has to be met by
// the values that follow it, nesting is bounded, the types are few and made of few, and every
// length and every message has to end where the decoder will find the end. A stream that is as long as it says is returned as it
// is. One whose interface lengths the decoder and a skip would read differently, which is what
// the encoder writes for an interface that holds a value of a type that was not sent before,
// is written out again with the type definitions in front and exact lengths. What is left of
// the work of the decoder is then proportional to the length of the stream.
//
// Memory is not: a value of a registered type takes as much as the type does, whatever the
// stream says of it, and a stream of a thousand empty elements of a type of a megabyte takes a
// gigabyte. That is the size of the application's own types, not of the stream.
package gobcheck

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"sync"
	"sync/atomic"
)

// ErrInvalid is the error of every stream that Normalize refuses. The text after it says why;
// none of it is taken from the stream.
var ErrInvalid = errors.New("gob stream refused")

// The type ids of the types gob knows without being told, and the first one it gives to the
// types that it is told of.
const (
	idBool      = 1
	idInt       = 2
	idUint      = 3
	idFloat     = 4
	idBytes     = 5
	idString    = 6
	idComplex   = 7
	idInterface = 8

	firstUserID = 64
)

const (
	// maxDepth is how many levels of values may be nested in one another: a map in a slice in a
	// struct is three. The decoder takes a few frames of stack for each, so this is megabytes,
	// where the decoder fails at a gigabyte. A session of this package holds a value of no more
	// than 64 containers, which is twice as many levels of values, so every session it wrote
	// is within it by a long way.
	maxDepth = 1000
	// maxNameLen is the longest name of a type the decoder takes for an interface value.
	maxNameLen = 1024
	// maxTypes is how many types a stream may define: a session holds a few. The decoder compiles the
	// way to skip a struct by calling itself for each struct that is a field of it, so a chain of types
	// as long as the stream would take a stack that the decoder does not have.
	maxTypes = 4096
	// maxTypeCost is how many types the decoder goes through to skip a value of one, for each field of
	// a struct that it skips: the type, and, for an array or a slice, those of its elements, and for a
	// map those of its keys and of its elements, counted again where they are shared. It starts again
	// for every field, so a struct of many fields of a type made of many types, which is all it takes
	// to define in a stream, takes a time and a memory that grow with the square of its length. A type
	// of a session is made of a few: a map of slices of maps of structs is four.
	maxTypeCost = 16

	// costUnknown is the cost of a type that was not worked out yet.
	costUnknown = -1
)

// typeID is the id gob gives a type, a signed 32-bit integer.
type typeID int32

// kind is what a type definition says a type is made of.
type kind uint8

const (
	kindArray   kind = iota + 1
	kindSlice        // elements of one type
	kindStruct       // fields, each of a type
	kindMap          // keys of one type, elements of another
	kindEncoder      // a type that writes itself as bytes: GobEncoder, BinaryMarshaler, TextMarshaler
)

// wire is the definition of a type of the stream.
type wire struct {
	length uint64 // kindArray: the number of elements
	first  int    // kindStruct: where its field types start in checker.fields
	count  int    // kindStruct: how many it has
	elem   typeID // kindArray, kindSlice, kindMap
	key    typeID // kindMap
	cost   int8   // how many types the decoder goes through to skip a value of it, once worked out
	kind   kind
}

// badStream is the panic that stands for a refusal inside the checker, which recovers it.
type badStream struct{ reason string }

// typeDef is a type of the stream and the id it was given.
type typeDef struct {
	w  wire
	id typeID
}

// scanTypes is how many types are looked up by going through them: a session holds one or a few.
// A stream that defines more is looked up in a map, so that it cannot make every lookup long.
const scanTypes = 8

// checker reads one stream. It is used by one goroutine at a time, and kept in a pool.
type checker struct {
	index   map[typeID]int // the place of each of types, once there are more than scanTypes
	types   []typeDef      // the types defined so far, in the order of the stream
	data    []byte
	fields  []typeID // the field types of the structs of types, one after the other
	out     []byte   // emit: the value message
	hoisted []byte   // emit: the type definitions, as messages
	pos     int
	end     int // where the message that is being read ends
	depth   int // how many values are open
	indexed bool
	canon   bool
	emit    bool // write the stream out again, in the form that has no nested type definitions
}

var checkers = sync.Pool{New: func() any { return new(checker) }}

// slot returns the place among the types of the one that has the id, if the stream defined one.
func (c *checker) slot(id typeID) (int, bool) {
	if c.indexed {
		i, ok := c.index[id]
		return i, ok
	}
	for i := range c.types {
		if c.types[i].id == id {
			return i, true
		}
	}
	return 0, false
}

// lookup returns the type that has the id, if the stream defined one.
func (c *checker) lookup(id typeID) (wire, bool) {
	i, ok := c.slot(id)
	if !ok {
		return wire{}, false
	}
	return c.types[i].w, true
}

// define adds the type that has the id.
func (c *checker) define(id typeID, w wire) {
	if len(c.types) >= maxTypes {
		c.fail("the stream defines too many types")
	}
	w.cost = costUnknown
	c.types = append(c.types, typeDef{w: w, id: id})
	switch {
	case c.indexed:
		c.index[id] = len(c.types) - 1
	case len(c.types) > scanTypes:
		if c.index == nil {
			c.index = make(map[typeID]int, 2*scanTypes)
		} else {
			clear(c.index)
		}
		for i, def := range c.types {
			c.index[def.id] = i
		}
		c.indexed = true
	default:
	}
}

// Normalize checks the stream in data, as the first value in it is read by a gob.Decoder, and
// returns the stream to decode: data itself if the lengths in it are the lengths of what they
// precede, or the same stream written again so that they are. The value is the first one that
// is not a type definition; whatever follows it is not read, as the decoder does not.
//
// The stream is refused, with an error that wraps ErrInvalid, when it is cut short, when a
// count is larger than the data that is left, when values are nested more than 1000 deep, when it
// defines more than 4096 types or one that is made of more than 16, and when it is not what the
// encoder writes in some other way.
func Normalize(data []byte) (checked []byte, err error) { //nolint:nonamedreturns // the deferred call turns a refusal into the error
	c, ok := checkers.Get().(*checker)
	if !ok {
		c = new(checker)
	}
	defer func() {
		if r := recover(); r != nil {
			checked = nil
			if bad, isBad := r.(badStream); isBad {
				err = fmt.Errorf("%w: %s", ErrInvalid, bad.reason)
			} else {
				err = fmt.Errorf("%w: internal error: %v", ErrInvalid, r)
			}
		}
		c.release()
	}()

	c.canon = false
	c.start(data, false)
	c.stream()
	c.checkCosts()
	if !c.canon {
		return data, nil
	}

	c.start(data, true)
	c.stream()
	checked = make([]byte, 0, len(c.hoisted)+9+len(c.out))
	checked = append(checked, c.hoisted...)
	checked = appendUint(checked, uint64(len(c.out)))
	checked = append(checked, c.out...)
	return checked, nil
}

// checkCosts refuses a stream that has a type that the decoder goes through too many types to skip.
func (c *checker) checkCosts() {
	for i := range c.types {
		if c.cost(c.types[i].id) > maxTypeCost {
			c.fail("a type is made of more types than the decoder goes through to skip a value of it")
		}
	}
}

// cost returns how many types the decoder goes through to skip a value of the type id, which is no
// more than maxTypeCost+1: a type that was not defined costs nothing, as the decoder refuses it, a
// struct or a type that writes itself costs one, as it keeps what it compiled for the next field, and
// a type that refers to itself costs what it does the first time. A type is worked out once, so this
// takes as long as there are types, and goes as deep as they are many, which is maxTypes.
func (c *checker) cost(id typeID) int {
	if id < firstUserID {
		if id == idInterface {
			return 1
		}
		return 0
	}
	i, ok := c.slot(id)
	if !ok {
		return 0
	}
	w := &c.types[i].w
	if w.cost != costUnknown {
		return int(w.cost)
	}
	w.cost = 1
	n := 1
	switch w.kind {
	case kindArray, kindSlice:
		n += c.cost(w.elem)
	case kindMap:
		n += c.cost(w.key)
		if n <= maxTypeCost {
			n += c.cost(w.elem)
		}
	case kindStruct, kindEncoder:
	}
	w.cost = int8(min(n, maxTypeCost+1))
	return int(w.cost)
}

// start makes the checker ready to read data from the beginning.
func (c *checker) start(data []byte, emit bool) {
	c.forget()
	c.data = data
	c.fields = c.fields[:0]
	c.out = c.out[:0]
	c.hoisted = c.hoisted[:0]
	c.pos, c.end, c.depth = 0, 0, 0
	c.emit = emit
}

// forget drops the types of the stream.
func (c *checker) forget() {
	c.indexed = false
	c.types = c.types[:0]
}

// release gives the checker back, without what it holds of the stream, or buffers that grew large.
func (c *checker) release() {
	const keep = 1 << 16
	c.forget()
	c.data = nil
	if cap(c.out) > keep {
		c.out = nil
	}
	if cap(c.hoisted) > keep {
		c.hoisted = nil
	}
	if cap(c.fields) > keep {
		c.fields = nil
	}
	if cap(c.types) > keep/16 {
		c.types, c.index = nil, nil
	}
	checkers.Put(c)
}

func (*checker) fail(reason string) {
	panic(badStream{reason})
}

// copyFrom adds what was read since start to the stream that is written, if one is.
func (c *checker) copyFrom(start int) {
	if c.emit {
		c.out = append(c.out, c.data[start:c.pos]...)
	}
}

// left is how many bytes of the stream are left, in all the messages that follow.
func (c *checker) left() uint64 {
	return uint64(len(c.data) - c.pos) //nolint:gosec // G115: the position never passes the end of the data
}

// room is how many bytes of the message that is being read are left.
func (c *checker) room() uint64 {
	return uint64(c.end - c.pos) //nolint:gosec // G115: the position never passes the end of its message
}

// skip moves past n bytes of the message, which the caller has checked against room.
func (c *checker) skip(n uint64) {
	c.pos += int(n) //nolint:gosec // G115: n <= room
}

// since is how many bytes were read after start.
func (c *checker) since(start int) uint64 {
	return uint64(c.pos - start) //nolint:gosec // G115: the position only moves forward
}

// uvarintBefore reads an unsigned integer that has to end before limit: a byte below 128 is the
// number, any other is the negated count of the bytes that hold it, up to eight.
func (c *checker) uvarintBefore(limit int) uint64 {
	if c.pos >= limit {
		c.fail("the data ends in the middle of a value")
	}
	b := c.data[c.pos]
	c.pos++
	if b <= 0x7f {
		return uint64(b)
	}
	n := 256 - int(b)
	if n > 8 {
		c.fail("an unsigned integer is out of range")
	}
	if limit-c.pos < n {
		c.fail("the data ends in the middle of a value")
	}
	var x uint64
	for _, v := range c.data[c.pos : c.pos+n] {
		x = x<<8 | uint64(v)
	}
	c.pos += n
	return x
}

// uvarint reads an unsigned integer of the message that is being read.
func (c *checker) uvarint() uint64 {
	// What nearly every integer is, so that this is inlined.
	if c.pos < c.end {
		if b := c.data[c.pos]; b <= 0x7f {
			c.pos++
			return uint64(b)
		}
	}
	return c.uvarintBefore(c.end)
}

// varint reads a signed integer: the unsigned one is the number shifted left by one, with the
// low bit set to say that it is the complement.
func (c *checker) varint() int64 {
	x := c.uvarint()
	i := int64(x >> 1)
	if x&1 != 0 {
		i = ^i
	}
	return i
}

// id reads a type id, which has to fit the 32 bits it has.
func (c *checker) id() typeID {
	i := c.varint()
	if i < math.MinInt32+1 || i > math.MaxInt32 {
		c.fail("a type id is out of range")
	}
	return typeID(i)
}

// reference reads the id of a type that a definition refers to: one that gob knows, or one that
// it is given.
func (c *checker) reference() typeID {
	id := c.id()
	if id < idBool || (id > idInterface && id < firstUserID) {
		c.fail("a type refers to a type id that is not one")
	}
	return id
}

// bytes skips the length of a string or of bytes, and the bytes.
func (c *checker) bytes() {
	n := c.uvarint()
	if n > c.room() {
		c.fail("a string is longer than the data that is left")
	}
	c.skip(n)
}

// message starts the next message of the stream, whose length is next, and which has to be in
// the data. It is called when the one before is read to the end.
func (c *checker) message() {
	n := c.uvarintBefore(len(c.data))
	if n > c.left() {
		c.fail("the length of a message does not fit the data")
	}
	c.end = c.pos + int(n) //nolint:gosec // G115: n <= left
}

// stream reads the type definitions, each in a message of its own, and the message of the value
// that follows them, as Decoder.Decode does.
func (c *checker) stream() {
	for {
		start := c.pos
		c.message()
		first := c.pos
		id := c.id()
		if id < 0 {
			c.messageTypedef(-id)
			if c.emit {
				c.hoisted = append(c.hoisted, c.data[start:c.pos]...)
			}
			continue
		}
		c.copyFrom(first)
		c.value(id)
		if c.pos != c.end {
			c.fail("a message holds more than its value")
		}
		return
	}
}

// messageTypedef reads the definition of the type id that is all of the message that is being
// read. The definitions that streams start with are the same from one stream to the next, so
// the ones that were read before are remembered, by their bytes.
func (c *checker) messageTypedef(id typeID) {
	body := c.data[c.pos:c.end]
	if cached := cachedTypeFor(id, body); cached != nil {
		if _, ok := c.lookup(id); ok {
			c.fail("a type is defined twice")
		}
		w := cached.w
		w.first = len(c.fields)
		c.fields = append(c.fields, cached.fields...)
		c.define(id, w)
		c.pos = c.end
		return
	}
	first := len(c.fields)
	c.typedef(id)
	if c.pos != c.end {
		c.fail("a message holds more than a type definition")
	}
	// Only what a stream defines first is remembered: a stream that defines thousands of types would
	// otherwise replace the list again and again.
	if w, _ := c.lookup(id); len(body) <= maxCachedBody && len(c.types) <= maxCached {
		rememberType(id, body, w, c.fields[first:])
	}
}

// cachedType is a definition that was read, as it came in a message of its own.
type cachedType struct {
	body   []byte   // the definition, which follows the id in the message
	fields []typeID // the types of the fields, for a struct
	w      wire     // the type, with the fields counted from the start of fields
	id     typeID
}

const (
	// maxCached is how many definitions are remembered, and maxCachedBody how long one can be.
	maxCached     = 16
	maxCachedBody = 512
)

// typeCache holds the definitions that were read, as a list that is never changed: a new one
// replaces the list, so that reading it needs no lock.
var typeCache atomic.Pointer[[]cachedType]

// cachedTypeFor returns what was remembered of the definition body of the type id, if anything.
func cachedTypeFor(id typeID, body []byte) *cachedType {
	list := typeCache.Load()
	if list == nil {
		return nil
	}
	for i := range *list {
		if cached := &(*list)[i]; cached.id == id && bytes.Equal(cached.body, body) {
			return cached
		}
	}
	return nil
}

// rememberType remembers a definition: the list is copied with it at the end, without the oldest
// when it is full.
func rememberType(id typeID, body []byte, w wire, fields []typeID) {
	w.first = 0
	entry := cachedType{id: id, body: bytes.Clone(body), fields: slices.Clone(fields), w: w}
	for {
		old := typeCache.Load()
		var list []cachedType
		if old != nil {
			list = *old
			if len(list) >= maxCached {
				list = list[len(list)-maxCached+1:]
			}
		}
		next := append(slices.Clone(list), entry)
		if typeCache.CompareAndSwap(old, &next) {
			return
		}
	}
}

// value reads a value of the type id as the stream holds it at the top: a struct as its fields,
// anything else after a zero, which is the number of the field that a struct would start with.
func (c *checker) value(id typeID) {
	if id > idInterface {
		if w, ok := c.lookup(id); ok && w.kind == kindStruct {
			c.structValue(w)
			return
		}
	}
	start := c.pos
	if c.uvarint() != 0 {
		c.fail("a value that is not a struct does not start with a zero")
	}
	c.copyFrom(start)
	c.field(id)
}

// field reads a value of the type id, as a field of a struct or an element of a slice does.
func (c *checker) field(id typeID) {
	c.depth++
	if c.depth > maxDepth {
		c.fail("the values are nested too deeply")
	}
	start := c.pos
	switch id {
	case idBool, idInt, idUint, idFloat:
		c.uvarint()
		c.copyFrom(start)
	case idComplex:
		c.uvarint()
		c.uvarint()
		c.copyFrom(start)
	case idBytes, idString:
		c.bytes()
		c.copyFrom(start)
	case idInterface:
		c.iface()
	default:
		w, ok := c.lookup(id)
		if !ok {
			c.fail("a value is of a type that was not defined")
		}
		c.composite(w, start)
	}
	c.depth--
}

// composite reads a value of a type that was defined, other than a struct at the top.
func (c *checker) composite(w wire, start int) {
	switch w.kind {
	case kindArray:
		n := c.uvarint()
		c.copyFrom(start)
		if n != w.length || n > c.left() {
			c.fail("an array does not have the length of its type")
		}
		for range n {
			c.field(w.elem)
		}
	case kindSlice:
		n := c.uvarint()
		c.copyFrom(start)
		if n > c.left() {
			c.fail("a slice claims more elements than there are bytes left")
		}
		for range n {
			c.field(w.elem)
		}
	case kindMap:
		n := c.uvarint()
		c.copyFrom(start)
		if n > c.left() {
			c.fail("a map claims more entries than there are bytes left")
		}
		for range n {
			c.field(w.key)
			c.field(w.elem)
		}
	case kindStruct:
		c.structValue(w)
	case kindEncoder:
		c.bytes()
		c.copyFrom(start)
	}
}

// advance returns the number of the field that a delta after the field last leads to, of the
// fields numbered from 0 to count-1, and whether there is one.
func advance(last int, delta uint64, count int) (int, bool) {
	if delta > uint64(count-1-last) { //nolint:gosec // G115: last < count
		return 0, false
	}
	return last + int(delta), true //nolint:gosec // G115: delta <= count-1-last
}

// structValue reads the fields of a struct: each is the distance from the one before, which
// is never zero, and the value; a zero ends the struct.
func (c *checker) structValue(w wire) {
	last := -1
	for {
		start := c.pos
		delta := c.uvarint()
		c.copyFrom(start)
		if delta == 0 {
			return
		}
		var ok bool
		if last, ok = advance(last, delta, w.count); !ok {
			c.fail("a field number is out of range")
		}
		c.field(c.fields[w.first+last])
	}
}

// iface reads an interface value: the name it was registered by, which is empty for nil, the
// definitions of types that were not sent before, the id of the type of the value, the length of
// the value and the value.
//
// What the length says is not read by the decoder when it decodes the value, and is how far it
// skips when it does not, and the encoder writes the length of the first part of a value that
// holds a definition. A length that is not what was read here makes the stream one that is
// written out again.
func (c *checker) iface() {
	start := c.pos
	n := c.uvarint()
	if n == 0 {
		c.copyFrom(start)
		return
	}
	if n > maxNameLen || n > c.room() {
		c.fail("the name of a type is not one")
	}
	c.skip(n)
	c.copyFrom(start)
	if !c.emit && c.scalar() {
		return
	}

	var (
		idStart int
		id      typeID
	)
	for {
		// The decoder asks for the next message when it has read the one that it has.
		if c.pos == c.end {
			c.message()
		}
		idStart = c.pos
		id = c.id()
		if id >= 0 {
			break
		}
		c.typedef(-id)
		if c.emit {
			c.hoisted = appendUint(c.hoisted, c.since(idStart))
			c.hoisted = append(c.hoisted, c.data[idStart:c.pos]...)
		}
		// What follows a definition that did not end the message is the length of the next
		// piece, which the decoder skips.
		if c.pos < c.end {
			c.uvarint()
		}
	}
	idEnd := c.pos
	size := c.uvarint()
	region := c.pos

	at := 0
	if c.emit {
		c.out = append(c.out, c.data[idStart:idEnd]...)
		at = len(c.out)
		c.out = append(c.out, make([]byte, 9)...)
	}
	c.value(id)
	if c.since(region) != size {
		c.canon = true
	}
	if c.emit {
		putFixed(c.out[at:at+9], uint64(len(c.out)-at-9)) //nolint:gosec // G115: the nine bytes are in out
	}
}

// scalar reads the rest of an interface value that holds a boolean, a number or a string, if that
// is what follows the name: the id of the type, the length of the value, a zero and the value.
// Nearly every value of a session is one, and is read here without the calls that any other takes.
// It reads nothing, and says so, when it is anything else, or when it is not as it should be; the
// reading that takes on every case finds out why. It is the same reading for what it takes.
func (c *checker) scalar() bool {
	if c.end-c.pos < 4 {
		return false
	}
	p := c.data[c.pos:c.end]
	id, size := p[0], p[1]
	// The id of a type from the boolean to the string, as a signed integer is written: twice the
	// id, in one byte. The length of the value is one byte as well, the zero follows it.
	if id&1 != 0 || id < idBool<<1 || id > idString<<1 || size > 0x7f || p[2] != 0 {
		return false
	}
	n := 4
	switch id >> 1 {
	case idBytes, idString:
		if p[3] > 0x7f || int(p[3]) > len(p)-4 {
			return false
		}
		n += int(p[3])
	default:
		// One byte below 128, or the count of the bytes and the bytes.
		if p[3] > 0x7f {
			k := 256 - int(p[3])
			if k > 8 || k > len(p)-4 {
				return false
			}
			n += k
		}
	}
	if int(size) != n-2 {
		return false
	}
	c.pos += n
	return true
}

// typedef reads the definition of the type id, which follows its negated id: a wireType, which is
// a struct of which exactly one of seven fields is sent.
func (c *checker) typedef(id typeID) {
	if id < firstUserID {
		c.fail("a type is defined with an id that gob keeps")
	}
	if _, ok := c.lookup(id); ok {
		c.fail("a type is defined twice")
	}
	var w wire
	switch c.uvarint() {
	case 1:
		w.kind = kindArray
	case 2:
		w.kind = kindSlice
	case 3:
		w.kind = kindStruct
	case 4:
		w.kind = kindMap
	case 5, 6, 7:
		w.kind = kindEncoder
	default:
		c.fail("a type definition is of no kind")
	}
	c.typeStruct(&w)
	if c.uvarint() != 0 {
		c.fail("a type definition holds more than one type")
	}
	c.define(id, w)
}

// typeStruct reads the struct that defines a type of the kind of w. Its first field is the
// common part, and the others are, for an array, the type of the elements and their number, for
// a slice the type of the elements, for a struct its list of fields, for a map the type of the
// keys and of the elements, and for a type that writes itself none.
func (c *checker) typeStruct(w *wire) {
	last := -1
	for {
		delta := c.uvarint()
		if delta == 0 {
			return
		}
		var ok bool
		if last, ok = advance(last, delta, 3); !ok {
			c.fail("a type definition has a field it should not")
		}
		c.typeField(w, last)
	}
}

// typeField reads the field of a type definition that has the number field.
func (c *checker) typeField(w *wire, field int) {
	switch {
	case field == 0:
		c.common()
	case field == 1 && (w.kind == kindArray || w.kind == kindSlice):
		w.elem = c.reference()
	case field == 1 && w.kind == kindMap:
		w.key = c.reference()
	case field == 1 && w.kind == kindStruct:
		c.structFields(w)
	case field == 2 && w.kind == kindArray:
		length := c.varint()
		if length < 0 {
			c.fail("an array has a negative length")
		}
		w.length = uint64(length)
	case field == 2 && w.kind == kindMap:
		w.elem = c.reference()
	default:
		c.fail("a type definition has a field it should not")
	}
}

// common reads the part that every definition starts with: the name of the type and its id.
func (c *checker) common() {
	last := -1
	for {
		delta := c.uvarint()
		if delta == 0 {
			return
		}
		var ok bool
		if last, ok = advance(last, delta, 2); !ok {
			c.fail("the common part of a type definition has a field it should not")
		}
		if last == 0 {
			c.bytes()
		} else {
			c.id()
		}
	}
}

// structFields reads the list of fields of a struct type: how many there are and, for each, a
// name and the id of a type.
func (c *checker) structFields(w *wire) {
	n := c.uvarint()
	if n > c.left() {
		c.fail("a struct has more fields than there are bytes left")
	}
	w.first = len(c.fields)
	for range n {
		var id typeID
		last := -1
		for {
			delta := c.uvarint()
			if delta == 0 {
				break
			}
			var ok bool
			if last, ok = advance(last, delta, 2); !ok {
				c.fail("a field of a struct has a part it should not")
			}
			if last == 0 {
				c.bytes()
			} else {
				id = c.reference()
			}
		}
		if id == 0 {
			c.fail("a field of a struct has no type")
		}
		c.fields = append(c.fields, id)
	}
	w.count = len(c.fields) - w.first
}

// appendUint writes x as gob does: below 128 as a byte, otherwise as the negated count of its
// bytes and the bytes.
func appendUint(dst []byte, x uint64) []byte {
	if x < 0x80 {
		return append(dst, byte(x))
	}
	var buf [9]byte
	binary.BigEndian.PutUint64(buf[1:], x)
	skip := bits.LeadingZeros64(x) >> 3
	buf[skip] = byte(skip - 8) //nolint:gosec // G115: the count of the bytes that follow, negated
	return append(dst, buf[skip:]...)
}

// putFixed writes x into the nine bytes of dst, as the decoder takes an integer that has leading
// zeros, so that what is written after the place for it does not depend on how large it turns out
// to be.
func putFixed(dst []byte, x uint64) {
	dst[0] = 0xf8 // minus eight: eight bytes follow
	binary.BigEndian.PutUint64(dst[1:], x)
}
