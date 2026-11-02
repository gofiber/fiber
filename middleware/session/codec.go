package session

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
)

// errRecordNeedsCodec is returned for a record of a Codec found by a store without one.
var errRecordNeedsCodec = errors.New("failed to decode session data: the session was stored by a store with a Codec, and this store has none")

// errCodecMismatch is returned for a record of another codec than the store's.
var errCodecMismatch = errors.New("session record of another codec")

// errNonStringKey is remembered by a Set under a Codec with a key that is not a string.
var errNonStringKey = errors.New("a store with a Codec keys its values by string")

// Codec converts the values of a session to bytes and back. A store with a Codec
// keeps each value encoded on its own, so a request decodes only the values it
// reads, and a save encodes only the values that were set. Config.Codec turns it on.
//
// A Codec is called from many requests at once and must be safe for that.
type Codec interface {
	// Name identifies the format, and the options it is used with: change it when
	// you change them. It is written into every record, and a store reads only the
	// records of the codec of that name, so that a session is never decoded by a
	// codec that did not encode it. It is asked once, when the store is made, and must
	// be between 1 and 255 bytes long.
	Name() string

	// Marshal encodes v. The session keeps a copy of what it returns. It runs while
	// Middleware.Set holds the lock of the middleware, so it must not use the session
	// or the middleware it encodes for.
	Marshal(v any) ([]byte, error)

	// Unmarshal decodes data into v, a non-nil pointer. It is handed a copy of the
	// encoded value, which it may keep, and runs without a lock of the session.
	Unmarshal(data []byte, v any) error
}

// namedCodec is a Codec with its name, which is written into every record and
// compared with the name of the records that are read. A store asks for the name
// once, when it is made, and the data of a session holds the pair, so that the two
// cannot come apart. It is the format of a session stored as a record, see format.
type namedCodec struct {
	Codec
	name string
}

// JSONCodec is the Codec of encoding/json. A value is read back as the type it is
// decoded into: GetInto and GetAs decode into your own types, while Get returns
// what encoding/json makes of an interface{}: float64 for every number, string,
// bool, []any and map[string]any.
type JSONCodec struct{}

// Name returns "json".
func (JSONCodec) Name() string { return "json" }

// Marshal encodes v with encoding/json.
func (JSONCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) } //nolint:wrapcheck // the error is encoding/json's own

// Unmarshal decodes data into v with encoding/json.
func (JSONCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) } //nolint:wrapcheck // the error is encoding/json's own

// funcCodec is the Codec NewCodec makes of a name and two functions.
type funcCodec struct {
	marshal   func(v any) ([]byte, error)
	unmarshal func(data []byte, v any) error
	name      string
}

func (c *funcCodec) Name() string                       { return c.name }
func (c *funcCodec) Marshal(v any) ([]byte, error)      { return c.marshal(v) }
func (c *funcCodec) Unmarshal(data []byte, v any) error { return c.unmarshal(data, v) }

// NewCodec makes a Codec of a name and the two functions of an encoding library,
// for example msgpack or CBOR, which this package does not import itself:
//
//	codec := session.NewCodec("msgpack", msgpack.Marshal, msgpack.Unmarshal)
//
// It panics if the name is empty or longer than 255 bytes, or a function is nil.
//
// Parameters:
//   - name: The name written into every record.
//   - marshal: Encodes a value.
//   - unmarshal: Decodes into a non-nil pointer.
//
// Returns:
//   - Codec: The codec.
//
// Usage:
//
//	session.New(session.Config{Codec: session.NewCodec("cbor", cbor.Marshal, cbor.Unmarshal)})
func NewCodec(name string, marshal func(v any) ([]byte, error), unmarshal func(data []byte, v any) error) Codec {
	if marshal == nil || unmarshal == nil {
		panic("[session] NewCodec needs a marshal and an unmarshal function")
	}
	checkCodecName(name)
	return &funcCodec{name: name, marshal: marshal, unmarshal: unmarshal}
}

// checkCodecName panics if name cannot be written into a record.
func checkCodecName(name string) {
	if !codecNameLenOK(len(name)) {
		panic(fmt.Sprintf("[session] the name of a Codec must be 1 to %d bytes long, got %d", maxCodecNameLen, len(name)))
	}
}

// encodedValue is what a session of a store with a codec holds under a key: the value
// as the codec encoded it, with the codec. What a session holds says what it is by its
// type, so a read asks nothing of the session: an encodedValue is decoded after the lock
// is let go, since a codec is not called with a lock held, and anything else, which is
// every value of a session stored as gob, is the value itself.
type encodedValue struct {
	codec *namedCodec
	bytes []byte
}

// readValue returns what a session holds as an interface{}. What is not an encoded
// value, a missing key or a nil, is returned as it is. A value that does not decode is
// returned encoded, a copy of its bytes, so that it is never taken for a key that is not
// there.
func readValue(held any) any {
	if e, ok := held.(*encodedValue); ok {
		return e.decode()
	}
	return held
}

// unmarshal decodes the value into out, a non-nil pointer. The codec is given a copy of
// the bytes, since it may keep them and the session owns these.
func (e *encodedValue) unmarshal(out any) error {
	return e.codec.Unmarshal(utils.CopyBytes(e.bytes), out)
}

// decode is readValue for an encoded value, a function of its own so that readValue is
// small enough to be inlined, and a read of a value that is not encoded costs no call.
func (e *encodedValue) decode() any {
	var out any
	if err := e.unmarshal(&out); err != nil {
		return utils.CopyBytes(e.bytes)
	}
	return out
}

// decodeAs reads an encoded value as a V. It is a function of its own so that the V it
// hands to the codec, which then lives on the heap, is made only when there is a codec.
func decodeAs[V any](e *encodedValue) (V, bool) {
	var out V
	if e.unmarshal(&out) != nil {
		var zero V
		return zero, false
	}
	return out, true
}

// readInto stores held, what a session holds under key, into target, a non-nil
// pointer, all or nothing. A value that is not encoded is the one stored and has to be
// assignable to what the target points to. An encoded one is decoded by decodeInto.
func readInto(held, key any, target reflect.Value) error {
	if e, ok := held.(*encodedValue); ok {
		return decodeInto(key, e, target)
	}

	elem := target.Elem()
	if held == nil {
		elem.SetZero()
		return nil
	}
	have := reflect.ValueOf(held)
	if !have.Type().AssignableTo(elem.Type()) {
		return fmt.Errorf("session value %#v is a %s, not assignable to a %s", key, have.Type(), elem.Type())
	}
	elem.Set(have)
	return nil
}

// decodeInto decodes e, the value of key, into target, a non-nil pointer. A target that
// holds its zero value, which is what a fresh variable holds, is decoded into directly:
// nothing is allocated for a second value and nothing is copied over the target. A target
// that holds something is replaced by a new value instead, since a decoder merges into
// the map or the struct it is given, and nothing of the old value may be left in the
// result. Either way a failure leaves the target as it was.
func decodeInto(key any, e *encodedValue, target reflect.Value) error {
	elem := target.Elem()
	if elem.IsZero() {
		decoded := false
		defer func() {
			if !decoded {
				// Undo what a decode that failed halfway, or panicked, put into the target.
				elem.SetZero()
			}
		}()
		if err := e.unmarshal(target.Interface()); err != nil {
			return decodeFailed(key, err)
		}
		decoded = true
		return nil
	}

	fresh := reflect.New(elem.Type())
	if err := e.unmarshal(fresh.Interface()); err != nil {
		return decodeFailed(key, err)
	}
	elem.Set(fresh.Elem())
	return nil
}

// decodeFailed is the error of GetInto for a value the codec could not decode.
func decodeFailed(key any, err error) error {
	return fmt.Errorf("failed to decode session value %#v: %w", key, err)
}

// getInto is GetInto of a session or of its middleware, whose lookup is passed in: the
// lookup takes the lock, and a codec is not called with it held.
func getInto(lookup func(key any) (any, bool), key, target any) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("session: the target of GetInto must be a non-nil pointer, got %T", target)
	}
	held, ok := lookup(key)
	if !ok {
		return keyNotFound(key)
	}
	return readInto(held, key, rv)
}

// format is how the data of a session is stored: a gob stream of the values themselves
// (gobFormat), or, when the store has a Codec, a record of the values encoded one by
// one (*namedCodec). It holds what the two do differently with the values of a session,
// which they are handed and not the session, and runs once per request. What runs for
// each value does not go through it: a read tells the two apart by the type of the
// value it finds (encodedValue), and Set by a type test of the format. What only a
// record needs, reading one and saying which codec wrote it, is not part of it: only
// *namedCodec has it.
type format interface {
	// keys returns the keys of the session, and whether one of them could alias the
	// session, so that the caller can change it through the key without a Set. The
	// caller holds the read lock.
	keys(values map[any]any) (keys []any, aliased bool)
	// encode returns the values of the session as the bytes to store. The caller holds
	// the read lock.
	encode(values map[any]any) ([]byte, error)
}

// gobFormat is the format of a store without a Codec, which every store can read.
type gobFormat struct{}

// gobStream is the one value of gobFormat. It holds nothing, so every session that is
// stored as gob shares it.
var gobStream = gobFormat{}

func (gobFormat) keys(values map[any]any) ([]any, bool) {
	keys := make([]any, 0, len(values))
	aliased := false
	for k := range values {
		// gob flattens pointer keys, so mutating a pointee changes the payload.
		aliased = aliased || valueMayAlias(k)
		keys = append(keys, k)
	}
	return keys, aliased
}

func (gobFormat) encode(values map[any]any) ([]byte, error) {
	byteBuffer := byteBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
	defer byteBufferPool.Put(byteBuffer)
	defer byteBuffer.Reset()
	if err := gob.NewEncoder(byteBuffer).Encode(values); err != nil {
		return nil, fmt.Errorf("failed to encode session data: %w", err)
	}
	return utils.CopyBytes(byteBuffer.Bytes()), nil
}

// keys returns the strings that key the values, and not the deadline of the session,
// which is not one of them. None of them can alias the session: what a read gets is
// decoded from the value, and a key is copied, since the one the session holds points
// into the snapshot and would keep it alive for as long as the caller keeps the key.
func (*namedCodec) keys(values map[any]any) ([]any, bool) {
	keys := make([]any, 0, len(values))
	for k := range values {
		if name, ok := k.(string); ok {
			keys = append(keys, strings.Clone(name))
		}
	}
	return keys, false
}

// encode writes the values, which are encoded already, as a record.
func (c *namedCodec) encode(values map[any]any) ([]byte, error) {
	var onStack [8]recordEntry // a session of a few values sorts its entries without allocating
	entries := onStack[:0]
	if len(values) > len(onStack) {
		entries = make([]recordEntry, 0, len(values))
	}
	var deadline recordDeadline
	for key, value := range values {
		if key == absExpirationKey {
			if at, ok := value.(time.Time); ok {
				deadline = deadlineOf(at)
			}
			continue
		}
		name, okKey := key.(string)
		held, okValue := value.(*encodedValue)
		if !okKey || !okValue {
			// Set keeps nothing else under a codec.
			return nil, fmt.Errorf("%w: unexpected entry %v of type %T", errInvalidRecord, key, value)
		}
		entries = append(entries, recordEntry{key: name, value: held.bytes})
	}
	slices.SortFunc(entries, func(a, b recordEntry) int { return strings.Compare(a.key, b.key) })

	encoded, err := encodeRecord(c.name, deadline, entries)
	if err != nil {
		return nil, fmt.Errorf("failed to encode session data: %w", err)
	}
	return encoded, nil
}

// loadRecord decodes raw, a record of the codec, into values, which the caller holds
// the lock of. The values stay encoded: they are decoded when they are read. Whatever
// was loaded is dropped when the record is not one the codec can load.
func (c *namedCodec) loadRecord(raw []byte, values map[any]any) error {
	// The values are held in slabs of up to 16, not one allocation each. A slab is never
	// grown, so the pointers into it hold, and none is larger than 16 entries however many
	// the record claims: a corrupt record cannot ask for more memory than it is long.
	var slab []encodedValue
	header, err := parseRecord(raw, func(key, value []byte, left int) {
		if len(slab) == cap(slab) {
			slab = make([]encodedValue, 0, min(left, 16))
		}
		// The values stay in the snapshot, which Save writes back while the data is
		// unchanged, and so do the keys: the snapshot belongs to the session and is
		// never written to, and keys leaves it only as copies.
		slab = append(slab, encodedValue{codec: c, bytes: value})
		values[utils.UnsafeString(key)] = &slab[len(slab)-1]
	})
	if err == nil && string(header.codec) != c.name {
		err = fmt.Errorf("%w: the record was written by the codec %q, the store has %q", errCodecMismatch, header.codec, c.name)
	}
	if err != nil {
		clear(values)
		return fmt.Errorf("failed to decode session data: %w", err)
	}
	if header.deadline.set {
		values[absExpirationKey] = header.deadline.time()
	}
	return nil
}

// reads reports whether raw is a record of this codec. A record that is damaged counts
// as one the store can read, and the decode says what is wrong with it.
func (c *namedCodec) reads(raw []byte) bool {
	version, name, ok := recordCodecOf(raw)
	return !ok || (version == recordVersion && string(name) == c.name)
}

// A store with a Codec keeps a session as a table of separately encoded values,
// so that a request decodes only the values it reads and a save re-encodes only
// the values that changed. The table is this record:
//
//	00 'F' 'S' 'R'           tag
//	01                       version
//	uvarint n, n bytes       name of the codec that encoded the values
//	byte 0                   no absolute deadline
//	byte 1, varint, uvarint  absolute deadline: Unix seconds, nanoseconds
//	uvarint count            number of entries
//	count times:             uvarint len, key bytes, uvarint len, value bytes
//
// Keys come in strictly ascending byte order, every integer is written in its
// shortest form, and the deadline lies between the years 1 and 9999, so that a
// record has exactly one encoding: parsing a record and writing it again gives
// the same bytes.
//
// A gob stream never starts with a zero byte, since its first byte is the length
// of a message and no message is empty. That tells a record from a session that
// was stored by a store without a Codec.
const (
	recordTag     = "\x00FSR"
	recordVersion = 1

	// maxCodecNameLen is the longest codec name a record can carry.
	maxCodecNameLen = 255

	// nanosPerSecond bounds the nanoseconds of a deadline.
	nanosPerSecond = 1_000_000_000

	// The deadline is a time of the years 1 to 9999. Beyond them the seconds of a
	// time.Time wrap, and a deadline that lies far in the future would read as one
	// that has passed, or the other way round.
	minDeadlineSec = -62_135_596_800 // 0001-01-01T00:00:00Z
	maxDeadlineSec = 253_402_300_799 // 9999-12-31T23:59:59Z
)

// codecNameLenOK reports whether n bytes can be the name of a codec in a record: one to
// maxCodecNameLen.
func codecNameLenOK(n int) bool {
	return n > 0 && n <= maxCodecNameLen
}

var (
	errInvalidRecord = errors.New("invalid session record")

	// errUnsupportedRecord is a record that is well formed as far as it can be read, but
	// of a version this package does not know.
	errUnsupportedRecord = fmt.Errorf("%w: unsupported version", errInvalidRecord)
)

// recordDeadline is the absolute deadline of a session as a record spells it.
type recordDeadline struct {
	sec   int64  // Unix seconds, between minDeadlineSec and maxDeadlineSec
	nanos uint32 // nanoseconds, below one second
	set   bool   // false if the session has no deadline
}

// deadlineOf returns the deadline of a session that ends at t, as a record spells it; the
// zero time stands for no deadline.
func deadlineOf(t time.Time) recordDeadline {
	if t.IsZero() {
		return recordDeadline{}
	}
	return recordDeadline{sec: t.Unix(), nanos: uint32(t.Nanosecond()), set: true} //nolint:gosec // below a second
}

// time returns the deadline as the time a session holds it as. It is meant for a deadline
// that is set.
func (d recordDeadline) time() time.Time {
	return time.Unix(d.sec, int64(d.nanos))
}

// valid reports whether the deadline can be written into a record and read back.
func (d recordDeadline) valid() bool {
	if !d.set {
		return d.sec == 0 && d.nanos == 0
	}
	if d.sec == minDeadlineSec && d.nanos == 0 {
		// The zero time.Time, which stands for no deadline: it would never expire.
		return false
	}
	return d.sec >= minDeadlineSec && d.sec <= maxDeadlineSec && d.nanos < nanosPerSecond
}

// recordHeader is what a record carries besides its entries.
type recordHeader struct {
	codec    []byte // aliases the record that was parsed
	deadline recordDeadline
}

// recordEntry is one key and its encoded value.
type recordEntry struct {
	key   string
	value []byte
}

// isRecord reports whether b is a record of a store with a Codec, as opposed to the
// gob stream of a store without one.
func isRecord(b []byte) bool {
	return len(b) >= len(recordTag) && string(b[:len(recordTag)]) == recordTag
}

// readUvarint reads an unsigned varint from the start of b. It refuses the
// longer-than-needed spellings of a number that binary.Uvarint accepts. It
// reports the number of bytes read, 0 if b does not start with a valid varint.
func readUvarint(b []byte) (value uint64, size int) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming the value and its size
	value, size = binary.Uvarint(b)
	if size <= 0 || (size > 1 && b[size-1] == 0) {
		return 0, 0
	}
	return value, size
}

// uvarintLen is the number of bytes binary.AppendUvarint writes for x.
func uvarintLen(x uint64) int {
	return (bits.Len64(x|1) + 6) / 7
}

// zigzag is the unsigned number that binary.AppendVarint writes for x.
func zigzag(x int64) uint64 {
	u := uint64(x) << 1 //nolint:gosec // the bits are the point
	if x < 0 {
		u = ^u
	}
	return u
}

// readVarint is readUvarint for the signed, zigzag-coded varint.
func readVarint(b []byte) (value int64, size int) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming the value and its size
	u, size := readUvarint(b)
	if size == 0 {
		return 0, 0
	}
	value = int64(u >> 1)
	if u&1 != 0 {
		value = ^value
	}
	return value, size
}

// readBytes reads a length-prefixed byte string from the start of b. It reports
// the bytes of the string and the number of bytes read in all, 0 if b does not
// start with one. The string has no spare capacity: appending to it cannot
// overwrite what follows it in b.
func readBytes(b []byte) (str []byte, size int) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming the string and its size
	length, n := readUvarint(b)
	if n == 0 || length > uint64(len(b)-n) { //nolint:gosec // n <= len(b)
		return nil, 0
	}
	end := n + int(length) //nolint:gosec // length <= len(b)-n, checked above
	return b[n:end:end], end
}

// recordCodecOf reads the version and the codec name from the start of b, a
// record, without looking at the rest. ok is false if they are not there.
func recordCodecOf(b []byte) (version byte, codec []byte, ok bool) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming three results of different kinds
	if !isRecord(b) || len(b) <= len(recordTag) {
		return 0, nil, false
	}
	version = b[len(recordTag)]
	if version != recordVersion {
		return version, nil, true
	}
	codec, size := readBytes(b[len(recordTag)+1:])
	if size == 0 || !codecNameLenOK(len(codec)) {
		return version, nil, false
	}
	return version, codec, true
}

// parseRecord checks b, a record, and calls add for each entry in key order, with the
// number of entries the record claims to hold from this one on, this one included. The
// slices handed to add alias b, and cannot be appended to without a copy. When
// parseRecord returns an error, add may already have been called for some entries.
//
// Every length is checked against the bytes that are left, and nothing is
// allocated for what the record claims to hold, so a corrupt record cannot ask for
// more memory than it is long: it is for add to bound what it allocates for the count.
func parseRecord(b []byte, add func(key, value []byte, left int)) (recordHeader, error) {
	var h recordHeader
	if !isRecord(b) || len(b) == len(recordTag) {
		return h, errInvalidRecord
	}
	if b[len(recordTag)] != recordVersion {
		return h, errUnsupportedRecord
	}
	b = b[len(recordTag)+1:]

	name, n := readBytes(b)
	if n == 0 || !codecNameLenOK(len(name)) {
		return h, errInvalidRecord
	}
	h.codec = name
	b = b[n:]

	if len(b) == 0 {
		return h, errInvalidRecord
	}
	flag := b[0]
	b = b[1:]
	switch flag {
	case 0:
	case 1:
		sec, read := readVarint(b)
		if read == 0 {
			return h, errInvalidRecord
		}
		b = b[read:]
		nanos, read := readUvarint(b)
		if read == 0 || nanos >= nanosPerSecond {
			return h, errInvalidRecord
		}
		b = b[read:]
		h.deadline = recordDeadline{sec: sec, nanos: uint32(nanos), set: true}
		if !h.deadline.valid() {
			return h, errInvalidRecord
		}
	default:
		return h, errInvalidRecord
	}

	count, n := readUvarint(b)
	// An entry takes at least two bytes: the two empty lengths.
	if n == 0 || count > uint64(len(b)-n)/2 { //nolint:gosec // n <= len(b)
		return h, errInvalidRecord
	}
	b = b[n:]

	var prev []byte
	for i := range count {
		key, n := readBytes(b)
		if n == 0 {
			return h, errInvalidRecord
		}
		b = b[n:]
		if i > 0 && bytes.Compare(prev, key) >= 0 {
			return h, errInvalidRecord
		}
		prev = key

		value, n := readBytes(b)
		if n == 0 {
			return h, errInvalidRecord
		}
		b = b[n:]
		add(key, value, int(count-i)) //nolint:gosec // count <= len(b)/2
	}
	if len(b) != 0 {
		return h, errInvalidRecord
	}
	return h, nil
}

// encodeRecord returns a record, in a buffer of exactly its size. entries must be in
// strictly ascending key order, the codec name between 1 and maxCodecNameLen bytes,
// and the deadline valid, or it returns errInvalidRecord.
func encodeRecord(codec string, deadline recordDeadline, entries []recordEntry) ([]byte, error) {
	if !codecNameLenOK(len(codec)) || !deadline.valid() {
		return nil, errInvalidRecord
	}
	for i := 1; i < len(entries); i++ {
		if strings.Compare(entries[i-1].key, entries[i].key) >= 0 {
			return nil, errInvalidRecord
		}
	}

	size := len(recordTag) + 1 + uvarintLen(uint64(len(codec))) + len(codec) + 1 + uvarintLen(uint64(len(entries)))
	if deadline.set {
		size += uvarintLen(zigzag(deadline.sec)) + uvarintLen(uint64(deadline.nanos))
	}
	for i := range entries {
		size += uvarintLen(uint64(len(entries[i].key))) + len(entries[i].key) +
			uvarintLen(uint64(len(entries[i].value))) + len(entries[i].value)
	}
	dst := make([]byte, 0, size)
	dst = append(dst, recordTag...)
	dst = append(dst, recordVersion)
	dst = binary.AppendUvarint(dst, uint64(len(codec)))
	dst = append(dst, codec...)
	if deadline.set {
		dst = append(dst, 1)
		dst = binary.AppendVarint(dst, deadline.sec)
		dst = binary.AppendUvarint(dst, uint64(deadline.nanos))
	} else {
		dst = append(dst, 0)
	}
	dst = binary.AppendUvarint(dst, uint64(len(entries)))
	for i := range entries {
		dst = binary.AppendUvarint(dst, uint64(len(entries[i].key)))
		dst = append(dst, entries[i].key...)
		dst = binary.AppendUvarint(dst, uint64(len(entries[i].value)))
		dst = append(dst, entries[i].value...)
	}
	return dst, nil
}
