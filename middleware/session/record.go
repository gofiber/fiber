package session

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"strings"
)

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
	if size == 0 || len(codec) == 0 || len(codec) > maxCodecNameLen {
		return version, nil, false
	}
	return version, codec, true
}

// parseRecord checks b, a record, and calls add for each entry in key order. The
// slices handed to add alias b, and cannot be appended to without a copy. When
// parseRecord returns an error, add may already have been called for some entries.
//
// Every length is checked against the bytes that are left, and nothing is
// allocated for what the record claims to hold, so a corrupt record cannot ask for
// more memory than it is long.
func parseRecord(b []byte, add func(key, value []byte)) (recordHeader, error) {
	var h recordHeader
	if !isRecord(b) || len(b) == len(recordTag) {
		return h, errInvalidRecord
	}
	if b[len(recordTag)] != recordVersion {
		return h, errUnsupportedRecord
	}
	b = b[len(recordTag)+1:]

	name, n := readBytes(b)
	if n == 0 || len(name) == 0 || len(name) > maxCodecNameLen {
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
		add(key, value)
	}
	if len(b) != 0 {
		return h, errInvalidRecord
	}
	return h, nil
}

// appendRecord appends a record to dst. entries must be in strictly ascending key
// order, the codec name between 1 and maxCodecNameLen bytes, and the deadline valid,
// or it returns errInvalidRecord and dst unchanged.
func appendRecord(dst []byte, codec string, deadline recordDeadline, entries []recordEntry) ([]byte, error) {
	if codec == "" || len(codec) > maxCodecNameLen || !deadline.valid() {
		return dst, errInvalidRecord
	}
	for i := 1; i < len(entries); i++ {
		if strings.Compare(entries[i-1].key, entries[i].key) >= 0 {
			return dst, errInvalidRecord
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
	if cap(dst)-len(dst) < size {
		grown := make([]byte, len(dst), len(dst)+size)
		copy(grown, dst)
		dst = grown
	}

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
