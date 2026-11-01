package session

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/utils/v2"
)

// msgp -file="data.go" -o="data_msgp.go" -tests=true -unexported
// Session state should remain small to fit common storage payload limits.
//
//go:generate msgp -o=data_msgp.go -tests=true -unexported
//msgp:ignore data
type data struct {
	// codec is the Codec of the store the session belongs to, with its name, nil for
	// gob: the session of a store with a codec is gob as well, when it was stored
	// before the codec was turned on. With a codec, Data holds the encoded values
	// ([]byte) of the string keys, and the deadline under absExpirationKey. It is set
	// when the session is acquired, and read under the lock.
	codec *namedCodec

	Data map[any]any // Session key counts are expected to be bounded.

	// errs are the Sets under a codec that failed, by key. A save reports them, and a
	// later Set or Delete of the key forgets its error. Allocated by the first failure.
	errs map[any]error

	sync.RWMutex `msg:"-"`

	// Stored before each mutation, so false means Data is unchanged since the decode.
	// Only a decode clears it; clearing after an encode would drop later in-place edits.
	dirty atomic.Bool
}

// valueMayAlias reports whether the caller could mutate the session through v
// without calling Set. Scalars are copies; anything else may share memory with Data.
func valueMayAlias(v any) bool {
	if _, ok := v.(time.Time); ok {
		// A copy, and its *Location is a shared immutable. Excluded because the
		// absolute-timeout bookkeeping reads one on every session load.
		return false
	}
	t := reflect.TypeOf(v)
	if t == nil {
		return false
	}
	switch t.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		// Matched on Kind so named scalars (type UserID int) are covered too.
		return false
	default:
		return true
	}
}

var dataPool = sync.Pool{
	New: func() any {
		d := new(data)
		d.Data = make(map[any]any)
		return d
	},
}

// acquireData returns a new data object from the pool.
//
// Returns:
//   - *data: The data object.
//
// Usage:
//
//	d := acquireData()
func acquireData() *data {
	obj := dataPool.Get()
	if d, ok := obj.(*data); ok {
		d.resetAs(nil)
		return d
	}
	// Handle unexpected type in the pool
	panic("unexpected type in data pool")
}

// releaseData resets the data object and returns it to the pool.
// d must not be used after calling this function.
// If d is nil, releaseData is a no-op.
func releaseData(d *data) {
	if d == nil {
		return
	}
	d.resetAs(nil)
	dataPool.Put(d)
}

// Reset clears the data map and resets the data object. The data keeps the codec
// of its session.
//
// Usage:
//
//	d.Reset()
func (d *data) Reset() {
	d.Lock()
	defer d.Unlock()
	d.emptyLocked()
}

// resetAs is Reset that also makes the data hold what codec says, in one step: a
// session of a store with a codec starts over as a session of the store, whatever
// it was stored as, and no value is set in between that the codec has not encoded.
// A nil codec makes the data hold the values themselves, for gob, which is how a
// session that goes back to the pool lets go of the codec of the store it served.
func (d *data) resetAs(codec *namedCodec) {
	d.Lock()
	defer d.Unlock()
	d.codec = codec
	d.emptyLocked()
}

// emptyLocked drops every value and every failed Set. The caller holds the lock.
func (d *data) emptyLocked() {
	d.dirty.Store(true)
	d.errs = nil
	clear(d.Data)
}

// Get retrieves a value from the data map by key.
//
// Parameters:
//   - key: The key to retrieve.
//
// Returns:
//   - any: The value associated with the key.
//
// Usage:
//
//	value := d.Get("key")
func (d *data) Get(key any) any {
	held, _ := d.fetch(key)
	return held.decode()
}

// fetch returns what is stored under key and whether there is a value under key. With
// a codec the value is in its encoded form, which the caller decodes outside the lock.
func (d *data) fetch(key any) (stored, bool) {
	d.RLock()
	defer d.RUnlock()
	v, ok := d.Data[key]
	if d.codec != nil {
		// What the caller gets is decoded from v, so nothing it holds aliases the session.
		return stored{value: v, codec: d.codec}, ok
	}
	if valueMayAlias(v) {
		// The caller can now mutate the session through v without calling Set.
		d.dirty.Store(true)
	}
	return stored{value: v}, ok
}

// getRaw returns what is stored under key as it is held, which under a codec is
// the encoded form.
func (d *data) getRaw(key any) any {
	d.RLock()
	defer d.RUnlock()
	return d.Data[key]
}

// Set updates or creates a new key-value pair in the data map.
//
// Parameters:
//   - key: The key to set.
//   - value: The value to set.
//
// Usage:
//
//	d.Set("key", "value")
func (d *data) Set(key, value any) {
	d.Lock()
	if codec := d.codec; codec != nil {
		d.Unlock()
		d.setEncoded(codec, key, value)
		return
	}
	defer d.Unlock()
	d.dirty.Store(true)
	d.Data[key] = value
}

// setRaw stores value under key as it is, whatever the codec: the bookkeeping of
// the session itself is not encoded.
func (d *data) setRaw(key, value any) {
	d.Lock()
	defer d.Unlock()
	d.dirty.Store(true)
	d.Data[key] = value
}

// setEncoded is Set under a codec: it encodes value now, outside the lock, so a
// later change to what value refers to is not stored, and keeps the result. A key
// that is not a string, or a value the codec cannot encode, is remembered and
// fails the save, until the key is set again or deleted.
func (d *data) setEncoded(codec *namedCodec, key, value any) {
	name, ok := key.(string)
	if !ok {
		d.fail(key, fmt.Errorf("%w, got a key of type %T", errNonStringKey, key))
		return
	}
	encoded, err := codec.Marshal(value)
	if err != nil {
		d.fail(key, fmt.Errorf("failed to encode session value %q: %w", name, err))
		return
	}
	// The session keeps its own copy: a codec may hand out a buffer that it reuses.
	owned := utils.CopyBytes(encoded)

	d.Lock()
	defer d.Unlock()
	d.dirty.Store(true)
	d.Data[name] = owned
	delete(d.errs, name)
}

// fail remembers that the Set of key failed.
func (d *data) fail(key any, err error) {
	d.Lock()
	defer d.Unlock()
	d.dirty.Store(true)
	if d.errs == nil {
		d.errs = make(map[any]error)
	}
	d.errs[key] = err
}

// pendingError returns the errors of the Sets that failed and have not been made up
// for, nil if there are none. They come in the order of their messages.
func (d *data) pendingError() error {
	if len(d.errs) == 0 {
		return nil
	}
	errs := make([]error, 0, len(d.errs))
	for _, err := range d.errs {
		errs = append(errs, err)
	}
	slices.SortFunc(errs, func(a, b error) int {
		return strings.Compare(a.Error(), b.Error())
	})
	return errors.Join(errs...)
}

// Delete removes a key-value pair from the data map.
//
// Parameters:
//   - key: The key to delete.
//
// Usage:
//
//	d.Delete("key")
func (d *data) Delete(key any) {
	d.Lock()
	defer d.Unlock()
	d.dirty.Store(true)
	delete(d.Data, key)
	if d.errs != nil {
		delete(d.errs, key)
	}
}

// Keys retrieves all keys in the data map.
//
// Returns:
//   - []any: A slice of all keys in the data map.
//
// Usage:
//
//	keys := d.Keys()
func (d *data) Keys() []any {
	d.RLock()
	defer d.RUnlock()
	keys := make([]any, 0, len(d.Data))
	if d.codec != nil {
		// Strings, and the deadline of the session, which is not one of its values.
		for k := range d.Data {
			if k != absExpirationKey {
				keys = append(keys, k)
			}
		}
		return keys
	}
	for k := range d.Data {
		if valueMayAlias(k) {
			// gob flattens pointer keys, so mutating a pointee changes the payload.
			d.dirty.Store(true)
		}
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of key-value pairs in the data map.
//
// Returns:
//   - int: The number of key-value pairs.
//
// Usage:
//
//	length := d.Len()
func (d *data) Len() int {
	d.RLock()
	defer d.RUnlock()
	return len(d.Data)
}

// isEmpty reports whether nothing a handler stored is in the data: no value, and no
// Set that failed, which a save would report. The absolute-timeout deadline that
// every new session of a store with an AbsoluteTimeout carries is not a value.
func (d *data) isEmpty() bool {
	d.RLock()
	defer d.RUnlock()
	if len(d.errs) > 0 {
		return false
	}
	for key := range d.Data {
		if key != absExpirationKey {
			return false
		}
	}
	return true
}

// byteBufferPool reuses the buffers that gob reads from and writes to.
var byteBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// bind makes the data hold what codec says, nil for gob. It is for data that was just
// acquired, which nothing else refers to yet, so it needs no lock.
func (d *data) bind(codec *namedCodec) {
	d.codec = codec
}

// decode loads rawData, a stored session, into the data and returns the bytes it was
// decoded from, a copy the data owns, which a save writes back while the data is
// unchanged. A record is read with the codec of the data, and its values stay encoded
// until they are read. Anything else is gob, and so stays the session of a store with
// a codec until it ends or is reset: its values are not converted, which could not be
// done without loss for every type. The caller holds the lock.
func (d *data) decode(rawData []byte) ([]byte, error) {
	// Copied before decoding: the storage owns rawData and may reuse it after Get
	// returns, so the decode and the snapshot Save writes back read the same bytes.
	raw := utils.CopyBytes(rawData)
	if isRecord(raw) {
		// The caller has seen to it that the store can read the record.
		if err := d.loadRecord(raw); err != nil {
			return nil, err
		}
		return raw, nil
	}
	byteBuffer := byteBufferPool.Get().(*bytes.Buffer) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
	defer byteBufferPool.Put(byteBuffer)
	defer byteBuffer.Reset()
	_, _ = byteBuffer.Write(raw)
	if err := gob.NewDecoder(byteBuffer).Decode(&d.Data); err != nil {
		return nil, fmt.Errorf("failed to decode session data: %w", err)
	}
	d.codec = nil
	d.dirty.Store(false)
	return raw, nil
}

// encode returns the data as the bytes to store: a record under a codec, a gob stream
// otherwise. The caller holds the read lock.
func (d *data) encode() ([]byte, error) {
	if d.codec != nil {
		return d.record()
	}
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

// loadRecord decodes raw, a record of the codec of the data, into it. The values
// stay encoded: they are decoded when they are read. The caller holds the lock.
func (d *data) loadRecord(raw []byte) error {
	if d.codec == nil {
		return errRecordNeedsCodec
	}
	header, err := parseRecord(raw, func(key, value []byte) {
		// The values stay in the snapshot, which Save writes back while the data is unchanged.
		d.Data[string(key)] = value
	})
	if err == nil && string(header.codec) != d.codec.name {
		err = fmt.Errorf("%w: the record was written by the codec %q, the store has %q", errCodecMismatch, header.codec, d.codec.name)
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

// record writes the data, whose values are encoded already, as a record. It is for
// data that holds a codec, and the caller holds the lock.
func (d *data) record() ([]byte, error) {
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
		encoded, okValue := value.([]byte)
		if !okKey || !okValue {
			// Set keeps nothing else under a codec.
			return nil, fmt.Errorf("%w: unexpected entry %v of type %T", errInvalidRecord, key, value)
		}
		entries = append(entries, recordEntry{key: name, value: encoded})
	}
	slices.SortFunc(entries, func(a, b recordEntry) int { return strings.Compare(a.key, b.key) })

	encoded, err := appendRecord(nil, d.codec.name, deadline, entries)
	if err != nil {
		return nil, fmt.Errorf("failed to encode session data: %w", err)
	}
	return encoded, nil
}
