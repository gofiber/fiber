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
// one (*namedCodec). It holds what differs between the two and runs once per request.
// What runs for each value does not go through it: a read tells the two apart by the
// type of the value it finds (encodedValue), and Set by a type test of the format.
type format interface {
	// keys returns the keys of the session. The caller holds the read lock.
	keys(d *data) []any
	// encode returns the data as the bytes to store. The caller holds the read lock.
	encode(d *data) ([]byte, error)
	// loadRecord loads raw, a record, into the data. The caller holds the lock.
	loadRecord(d *data, raw []byte) error
	// reads reports whether a store of this format can read raw, a stored record.
	reads(raw []byte) bool
	// describe says what a store of this format has, for the warning about a record it
	// cannot read.
	describe() string
}

// gobFormat is the format of a store without a Codec, which every store can read.
type gobFormat struct{}

func (gobFormat) keys(d *data) []any {
	keys := make([]any, 0, len(d.Data))
	for k := range d.Data {
		if valueMayAlias(k) {
			// gob flattens pointer keys, so mutating a pointee changes the payload.
			d.dirty.Store(true)
		}
		keys = append(keys, k)
	}
	return keys
}

func (gobFormat) encode(d *data) ([]byte, error) {
	byteBuffer := byteBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
	defer byteBufferPool.Put(byteBuffer)
	defer byteBuffer.Reset()
	if err := gob.NewEncoder(byteBuffer).Encode(&d.Data); err != nil {
		return nil, fmt.Errorf("failed to encode session data: %w", err)
	}
	encodedBytes := make([]byte, byteBuffer.Len())
	copy(encodedBytes, byteBuffer.Bytes())
	return encodedBytes, nil
}

func (gobFormat) loadRecord(*data, []byte) error { return errRecordNeedsCodec }

func (gobFormat) reads([]byte) bool { return false }

func (gobFormat) describe() string { return "has no codec" }

// keys returns the strings that key the values, and not the deadline of the session,
// which is not one of them.
func (*namedCodec) keys(d *data) []any {
	keys := make([]any, 0, len(d.Data))
	for k := range d.Data {
		if k != absExpirationKey {
			keys = append(keys, k)
		}
	}
	return keys
}

// encode writes the data, whose values are encoded already, as a record.
func (c *namedCodec) encode(d *data) ([]byte, error) {
	if err := d.pendingError(); err != nil {
		return nil, err
	}

	entries := make([]recordEntry, 0, len(d.Data))
	var deadline recordDeadline
	for key, value := range d.Data {
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

// loadRecord decodes raw, a record of the codec, into the data. The values stay
// encoded: they are decoded when they are read.
func (c *namedCodec) loadRecord(d *data, raw []byte) error {
	header, err := parseRecord(raw, func(key, value []byte) {
		// The values stay in the snapshot, which Save writes back while the data is unchanged.
		d.Data[string(key)] = &encodedValue{codec: c, bytes: value}
	})
	if err == nil && string(header.codec) != c.name {
		err = fmt.Errorf("%w: the record was written by the codec %q, the store has %q", errCodecMismatch, header.codec, c.name)
	}
	if err != nil {
		clear(d.Data)
		return fmt.Errorf("failed to decode session data: %w", err)
	}
	if header.deadline.set {
		d.Data[absExpirationKey] = time.Unix(header.deadline.sec, int64(header.deadline.nanos))
	}
	d.dirty.Store(false)
	return nil
}

// reads reports whether raw is a record of this codec. A record that is damaged counts
// as one the store can read, and the decode says what is wrong with it.
func (c *namedCodec) reads(raw []byte) bool {
	version, name, ok := recordCodecOf(raw)
	return !ok || (version == recordVersion && string(name) == c.name)
}

func (c *namedCodec) describe() string { return fmt.Sprintf("uses the codec %q", c.name) }
