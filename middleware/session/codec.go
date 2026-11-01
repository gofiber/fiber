package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/gofiber/fiber/v3/log"
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
// cannot come apart. A nil *namedCodec stands for gob.
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
	if name == "" || len(name) > maxCodecNameLen {
		panic(fmt.Sprintf("[session] the name of a Codec must be 1 to %d bytes long, got %d", maxCodecNameLen, len(name)))
	}
}

// decodeAny decodes the encoded value v of a session under codec into an
// interface{}. What is not an encoded value, a missing key or a nil, is returned
// as it is, and so is every value of a session without a codec. A value that does
// not decode is returned encoded, a copy of its bytes, so that it is never taken
// for a key that is not there.
func decodeAny(v any, codec *namedCodec) any {
	if codec == nil {
		return v
	}
	return decodeEncoded(v, codec)
}

// decodeEncoded is decodeAny for a session that has a codec. A function of its own,
// so that decodeAny is small enough to be inlined, and a read of a session without
// a codec costs no call.
func decodeEncoded(v any, codec *namedCodec) any {
	encoded, ok := v.([]byte)
	if !ok {
		return v
	}
	var out any
	if err := codec.Unmarshal(utils.CopyBytes(encoded), &out); err != nil {
		return utils.CopyBytes(encoded)
	}
	return out
}

// decodeAs reads v, the encoded value of a key under a codec, as a V. It is a
// function of its own so that the V it hands to the codec, which then lives on
// the heap, is made only when there is a codec.
func decodeAs[V any](v any, codec *namedCodec) (V, bool) {
	var out V
	encoded, ok := v.([]byte)
	if !ok || codec.Unmarshal(utils.CopyBytes(encoded), &out) != nil {
		var zero V
		return zero, false
	}
	return out, true
}

// intoTarget checks that target, the argument of GetInto, is a non-nil pointer.
func intoTarget(target any) (reflect.Value, error) {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return reflect.Value{}, fmt.Errorf("session: the target of GetInto must be a non-nil pointer, got %T", target)
	}
	return rv, nil
}

// valueInto stores v, the stored value of a key, into target, a non-nil pointer,
// all or nothing. Without a codec v is the value itself and has to be assignable
// to what the target points to. With one, v is the encoded value, and decodeInto
// decodes it.
func valueInto(key, v any, codec *namedCodec, target reflect.Value) error {
	if codec != nil {
		encoded, ok := v.([]byte)
		if !ok {
			return fmt.Errorf("%w: %#v", ErrKeyNotFound, key)
		}
		return decodeInto(key, utils.CopyBytes(encoded), codec, target)
	}

	elem := target.Elem()
	if v == nil {
		elem.SetZero()
		return nil
	}
	have := reflect.ValueOf(v)
	if !have.Type().AssignableTo(elem.Type()) {
		return fmt.Errorf("session value %#v is a %s, not assignable to a %s", key, have.Type(), elem.Type())
	}
	elem.Set(have)
	return nil
}

// decodeInto decodes encoded, the value of key, which the codec may keep, into
// target, a non-nil pointer. A target that holds its zero value, which is what a
// fresh variable holds, is decoded into directly: nothing is allocated for a second
// value and nothing is copied over the target. A target that holds something is
// replaced by a new value instead, since a decoder merges into the map or the
// struct it is given, and nothing of the old value may be left in the result. Either
// way a failure leaves the target as it was.
func decodeInto(key any, encoded []byte, codec *namedCodec, target reflect.Value) error {
	elem := target.Elem()
	if elem.IsZero() {
		decoded := false
		defer func() {
			if !decoded {
				// Undo what a decode that failed halfway, or panicked, put into the target.
				elem.SetZero()
			}
		}()
		if err := codec.Unmarshal(encoded, target.Interface()); err != nil {
			return fmt.Errorf("failed to decode session value %#v: %w", key, err)
		}
		decoded = true
		return nil
	}

	fresh := reflect.New(elem.Type())
	if err := codec.Unmarshal(encoded, fresh.Interface()); err != nil {
		return fmt.Errorf("failed to decode session value %#v: %w", key, err)
	}
	elem.Set(fresh.Elem())
	return nil
}

// sessionCodec returns the codec the sessions of the store hold, with its name, nil
// for gob. NewStore asks for the name once; a Store built without NewStore asks
// every time.
func (s *Store) sessionCodec() *namedCodec {
	if s.named != nil || s.Codec == nil {
		return s.named
	}
	return s.unnamedCodec()
}

// unnamedCodec makes the codec of a Store that was built without NewStore, which
// asks for the name every time.
func (s *Store) unnamedCodec() *namedCodec {
	return &namedCodec{Codec: s.Codec, name: s.Codec.Name()}
}

// refusesRecord reports whether raw, a stored session, is a record this store
// cannot read, and says so, once per store. A store reads the records of the codec
// that wrote them, and no other. (A stored session that is not a record is gob,
// which every store reads, a store with a codec as a session that stays gob.) A
// record that is damaged counts as one the store can read, and the decode says
// what is wrong with it.
func (s *Store) refusesRecord(raw []byte) bool {
	return isRecord(raw) && s.cannotRead(raw)
}

// cannotRead is refusesRecord for a stored session that is a record.
func (s *Store) cannotRead(raw []byte) bool {
	codec := s.sessionCodec()
	if codec != nil {
		version, name, ok := recordCodecOf(raw)
		if !ok || (version == recordVersion && string(name) == codec.name) {
			return false
		}
	}
	s.warnUnreadable(raw, codec)
	return true
}

// warnUnreadable says, once per store, that a stored session is a record this store,
// whose codec is codec, cannot read, and is treated as a session that does not exist.
func (s *Store) warnUnreadable(raw []byte, codec *namedCodec) {
	if !s.unreadableWarned.CompareAndSwap(false, true) {
		return
	}
	have := "has no codec"
	if codec != nil {
		have = fmt.Sprintf("uses the codec %q", codec.name)
	}
	version, name, known := recordCodecOf(raw)
	switch {
	case len(name) > 0:
		log.Warnf("[session] a stored session is a record of the codec %q (format version %d), which this store, which %s, cannot read; "+
			"it is treated as a new session, and the record is left alone. This is expected once after Config.Codec was changed or turned off; "+
			"if it goes on, applications with different codecs share a storage and a session cookie",
			name, version, have)
	case known:
		log.Warnf("[session] a stored session is a record of format version %d, which this store, which %s, does not know; "+
			"it was probably written by a newer release of the application. It is treated as a new session, and the record is left alone",
			version, have)
	default:
		log.Warn("[session] a stored session starts like a record, but its header is damaged; " +
			"it is treated as a new session, and the record is left alone")
	}
}
