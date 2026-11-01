package session

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"slices"
	"strings"
	"time"
)

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
	encodedBytes := make([]byte, byteBuffer.Len())
	copy(encodedBytes, byteBuffer.Bytes())
	return encodedBytes, nil
}

// keys returns the strings that key the values, and not the deadline of the session,
// which is not one of them. None of them can alias the session: what a read gets is
// decoded from the value.
func (*namedCodec) keys(values map[any]any) ([]any, bool) {
	keys := make([]any, 0, len(values))
	for k := range values {
		if k != absExpirationKey {
			keys = append(keys, k)
		}
	}
	return keys, false
}

// encode writes the values, which are encoded already, as a record.
func (c *namedCodec) encode(values map[any]any) ([]byte, error) {
	entries := make([]recordEntry, 0, len(values))
	var deadline recordDeadline
	for key, value := range values {
		if key == absExpirationKey {
			if at, ok := value.(time.Time); ok && !at.IsZero() {
				deadline = recordDeadline{sec: at.Unix(), nanos: uint32(at.Nanosecond()), set: true} //nolint:gosec // below a second
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

	encoded, err := appendRecord(nil, c.name, deadline, entries)
	if err != nil {
		return nil, fmt.Errorf("failed to encode session data: %w", err)
	}
	return encoded, nil
}

// loadRecord decodes raw, a record of the codec, into values, which the caller holds
// the lock of. The values stay encoded: they are decoded when they are read. Whatever
// was loaded is dropped when the record is not one the codec can load.
func (c *namedCodec) loadRecord(raw []byte, values map[any]any) error {
	header, err := parseRecord(raw, func(key, value []byte) {
		// The values stay in the snapshot, which Save writes back while the data is unchanged.
		values[string(key)] = &encodedValue{codec: c, bytes: value}
	})
	if err == nil && string(header.codec) != c.name {
		err = fmt.Errorf("%w: the record was written by the codec %q, the store has %q", errCodecMismatch, header.codec, c.name)
	}
	if err != nil {
		clear(values)
		return fmt.Errorf("failed to decode session data: %w", err)
	}
	if header.deadline.set {
		values[absExpirationKey] = time.Unix(header.deadline.sec, int64(header.deadline.nanos))
	}
	return nil
}

// reads reports whether raw is a record of this codec. A record that is damaged counts
// as one the store can read, and the decode says what is wrong with it.
func (c *namedCodec) reads(raw []byte) bool {
	version, name, ok := recordCodecOf(raw)
	return !ok || (version == recordVersion && string(name) == c.name)
}
