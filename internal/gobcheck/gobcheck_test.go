package gobcheck

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"net"
	"net/netip"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type (
	plain struct {
		B string
		C []byte
		F complex128
		D float64
		A int
		G uint16
		E bool
	}
	holder struct {
		Zero  [0]int
		In    any
		M     map[string]any
		P     *plain
		Items []any
		Arr   [3]int
	}
	unit struct{ A int }
	// lacks is a registered struct that has none of the fields that the streams of a chain of types have.
	lacks struct{ Z int }
	evil  struct {
		Extra map[unit]unit
		Key   string
	}
	inner struct{ P, Q int }
	// evoOld is a struct that holds a value of any type, evoHolder one that holds such a struct
	// in the field that code written later has dropped, which is holderNew.
	evoOld struct {
		Y any
		Z string
		X int
	}
	holderOld struct {
		W any
		B string
		A int
	}
	holderNew struct {
		B string
		A int
	}
)

func init() {
	for _, v := range []any{plain{}, holder{}, unit{}, evil{}, inner{}, evoOld{}, map[string]any{}, []any{}, map[string]int{}, time.Time{}, new(big.Int), net.IP{}, netip.Addr{}} {
		gob.Register(v)
	}
	gob.RegisterName("gobcheckLacks", lacks{})
	gob.RegisterName("gobcheckHolderAAAA", holderOld{})
	gob.RegisterName("gobcheckHolderBBBB", holderNew{})
}

// encode writes values as a session is stored.
func encode(tb testing.TB, values map[any]any) []byte {
	tb.Helper()
	var buf bytes.Buffer
	require.NoError(tb, gob.NewEncoder(&buf).Encode(values))
	return buf.Bytes()
}

// decode reads a stream as a session is loaded.
func decode(raw []byte) (map[any]any, error) {
	var values map[any]any
	err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&values)
	return values, err //nolint:wrapcheck // the tests read the error as gob gives it
}

// frames splits a stream into its messages, without their lengths.
func frames(tb testing.TB, stream []byte) [][]byte {
	tb.Helper()
	var out [][]byte
	for len(stream) > 0 {
		n, size := uint64(stream[0]), 1
		if stream[0] >= 0x80 {
			size = 1 - int(int8(stream[0]))
			n = 0
			for _, b := range stream[1:size] {
				n = n<<8 | uint64(b)
			}
		}
		require.LessOrEqual(tb, uint64(size)+n, uint64(len(stream)))
		out = append(out, stream[size:uint64(size)+n])
		stream = stream[uint64(size)+n:]
	}
	return out
}

// join writes the messages as a stream, each behind its length.
func join(messages ...[]byte) []byte {
	var out []byte
	for _, m := range messages {
		out = append(appendUint(out, uint64(len(m))), m...)
	}
	return out
}

// valid is what a session looks like in a store, and what the checker has to let through.
func valid(tb testing.TB) map[string]map[any]any {
	tb.Helper()
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	deep := any("leaf")
	for range 100 {
		deep = []any{deep}
	}
	nested := map[string]any{"a": []any{1, "two", 3.5, map[string]any{"b": []any{true, nil}}}}
	return map[string]map[any]any{
		"empty":             {},
		"scalars":           {"user": "alice", "id": 42, "admin": true, "ratio": 0.25, "bytes": []byte("xyz"), "u": uint(7), "i8": int8(-3), "c": 1 + 2i},
		"time and big":      {"seen": at, "big": big.NewInt(1 << 40)},
		"marshalers":        {"text": net.IPv4(10, 0, 0, 1), "binary": netip.MustParseAddr("2001:db8::1")},
		"struct":            {"p": plain{A: 1, B: "b", C: []byte{1, 2}, D: 2.5, E: true, F: 1 + 1i, G: 9}},
		"struct of structs": {"h": holder{In: plain{A: 2}, Items: []any{1, "x", plain{B: "y"}}, M: map[string]any{"k": unit{A: 3}}, Arr: [3]int{1, 2, 3}, P: &plain{A: 4}}},
		"maps":              {"m": map[string]int{"a": 1, "b": 2}, "nested": nested},
		"keys":              {1: "one", "two": 2, 3.5: "float", true: "bool"},
		"deep":              {"deep": deep},
		"types of values":   {"u": unit{A: 1}, "m": map[string]any{"k": unit{A: 2}, "j": inner{P: 3}}, "l": []any{inner{P: 4}, unit{A: 5}}},
		"a field of a type": {"w": evoOld{X: 7, Y: inner{P: 1, Q: 2}, Z: "tail"}},
	}
}

// Everything the encoder writes is let through and decodes as it did, whether it is returned as it
// is or written again.
func Test_Normalize_LetsThroughWhatGobWrites(t *testing.T) {
	t.Parallel()
	for name, values := range valid(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := encode(t, values)
			checked, err := Normalize(raw)
			require.NoError(t, err)
			if bytes.Equal(raw, checked) {
				require.Same(t, &raw[0], &checked[0], "a stream that is fine is not copied")
			}
			got, err := decode(checked)
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(values, got), "%v\n%v", values, got)

			// Written again, it is as good as it was.
			again, err := Normalize(checked)
			require.NoError(t, err)
			require.Equal(t, checked, again, "what was written again needs no more")
		})
	}
}

// A stream is written again when a type is defined inside the value of an interface, and not when
// it is defined before it. Maps of one entry show it, since the order of the entries of a bigger
// one is not fixed.
func Test_Normalize_WritesAgainWhenATypeIsDefinedInsideAValue(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		values  map[any]any
		rewrite bool
	}{
		"a string":                        {values: map[any]any{"s": "x"}},
		"a struct":                        {values: map[any]any{"u": unit{A: 1}}},
		"a map of builtin values":         {values: map[any]any{"m": map[string]int{"a": 1}}},
		"a map that holds only builtins":  {values: map[any]any{"m": map[string]any{"k": 1, "j": "s"}}},
		"a map in a map":                  {values: map[any]any{"m": map[string]any{"k": map[string]any{"j": 1}}}},
		"a slice in a slice":              {values: map[any]any{"l": []any{[]any{1}}}},
		"a struct that holds nothing":     {values: map[any]any{"w": evoOld{X: 7}}},
		"a struct that holds a builtin":   {values: map[any]any{"w": evoOld{X: 7, Y: "s"}}},
		"a map that holds a new struct":   {values: map[any]any{"m": map[string]any{"k": inner{P: 1}}}, rewrite: true},
		"a slice that holds a new struct": {values: map[any]any{"l": []any{inner{P: 1}}}, rewrite: true},
		"a struct that holds a new one":   {values: map[any]any{"w": evoOld{X: 7, Y: inner{P: 1}}}, rewrite: true},
		"a slice that holds a new slice":  {values: map[any]any{"l": []any{map[string]any{"k": 1}}}, rewrite: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := encode(t, tc.values)
			checked, err := Normalize(raw)
			require.NoError(t, err)
			require.Equal(t, tc.rewrite, !bytes.Equal(raw, checked))
		})
	}
}

// Whatever is cut off a stream, it is refused.
func Test_Normalize_RefusesWhatIsCutShort(t *testing.T) {
	t.Parallel()
	for name, values := range valid(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := encode(t, values)
			for n := range raw {
				_, err := Normalize(raw[:n])
				require.ErrorIs(t, err, ErrInvalid, "the first %d of %d bytes", n, len(raw))
			}
		})
	}
}

// bigCount is the number of entries that streams in the tests claim to have.
var bigCount = appendUint(nil, 1<<40)

// patched writes the stream again with the first occurrence of old in its last message, the one
// that holds the values, replaced by with, and the lengths that go with it.
func patched(tb testing.TB, stream, old, with []byte) []byte {
	tb.Helper()
	messages := frames(tb, stream)
	last := messages[len(messages)-1]
	require.Contains(tb, string(last), string(old))
	messages[len(messages)-1] = bytes.Replace(last, old, with, 1)
	return join(messages...)
}

// Test_Normalize_RefusesWhatCostsMoreThanItIsLong has the streams that the decoder takes far more
// time or memory for than they are long: a few bytes for gigabytes.
func Test_Normalize_RefusesWhatCostsMoreThanItIsLong(t *testing.T) {
	t.Parallel()
	for name, stream := range map[string][]byte{
		// The decoder makes the map for the entries that are claimed before it reads any.
		"a map that claims entries it has not": patched(t, encode(t, map[any]any{"m": map[string]int{"a": 1}}),
			[]byte{0x00, 0x01, 0x01, 0x61, 0x02}, append(append([]byte{0x00}, bigCount...), 0x01, 0x61, 0x02)),
		// The decoder reads the entries of a map of structs for as long as it is told, empty or not.
		"a map of structs that claims entries it has not": patched(t, encode(t, map[any]any{"e": evil{Extra: map[unit]unit{{}: {}}, Key: "x"}}),
			[]byte{0x01, 0x01, 0x00, 0x00, 0x01}, append(append([]byte{0x01}, bigCount...), 0x00, 0x00, 0x01)),
		"a session that claims entries it has not": join(
			[]byte{0x7f, 0x04, 0x01, 0x02, 0xff, 0x80, 0x00, 0x01, 0x10, 0x01, 0x10, 0x00, 0x00},
			append([]byte{0xff, 0x80, 0x00}, bigCount...)),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			_, err := Normalize(stream)
			require.ErrorIs(t, err, ErrInvalid)
			require.ErrorContains(t, err, "a map claims more entries")
			require.Less(t, time.Since(start), time.Second)
		})
	}

	t.Run("an array that has not the length of its type", func(t *testing.T) {
		t.Parallel()
		stream := patched(t, encode(t, map[any]any{"h": holder{Arr: [3]int{1, 2, 3}}}), []byte{0x03, 0x02, 0x04, 0x06}, []byte{0x04, 0x02, 0x04, 0x06, 0x08})
		_, err := Normalize(stream)
		require.ErrorContains(t, err, "an array does not have the length of its type")
	})
}

// Values nest up to a depth that the decoder can follow, and not beyond it: one more level than
// the checker takes is a level of the stack of the decoder.
func Test_Normalize_RefusesValuesNestedTooDeeply(t *testing.T) {
	t.Parallel()
	nest := func(levels int) []byte {
		var v any = "leaf"
		for range levels {
			v = []any{v}
		}
		return encode(t, map[any]any{"deep": v})
	}
	// A slice in an interface is two levels, and the map, its key and the leaf a few more.
	_, err := Normalize(nest(maxDepth/2 - 10))
	require.NoError(t, err)
	_, err = Normalize(nest(maxDepth/2 + 10))
	require.ErrorContains(t, err, "nested too deeply")
}

// A stream that nests two hundred thousand values is refused as the first thousand are read: the
// checker does not take a stack that the decoder would not, which is gigabytes, and the stream is
// as short as it can be made, four megabytes, with no length in it that is right.
func Test_Normalize_RefusesNestingThatWouldEndTheProcess(t *testing.T) {
	t.Parallel()
	messages := frames(t, encode(t, map[any]any{"d": []any{[]any{"x"}}}))
	require.Len(t, messages, 3, "the map type, the first value with the slice type, the value")
	// The value starts with the id of the slice type, which gob gives out as types are used, so
	// that it is read from the stream, the length of its value, a zero and a count of one: what each
	// level of []any in []any repeats after the name of the type.
	idLen := 1
	if messages[2][0] >= 0x80 {
		idLen += -int(int8(messages[2][0]))
	}
	id := messages[2][:idLen]
	level := append(append([]byte{0x0e}, "[]interface {}"...), id...)
	level = append(level, 0x00, 0x00, 0x01)
	value := append(append([]byte{}, id...), 0x00, 0x00, 0x01)
	value = append(value, bytes.Repeat(level, 200_000)...)
	value = append(value, 0x06, 's', 't', 'r', 'i', 'n', 'g', 0x0c, 0x03, 0x00, 0x01, 'x')
	stream := join(messages[0], messages[1], value)
	require.Greater(t, len(stream), 3_000_000)

	start := time.Now()
	_, err := Normalize(stream)
	require.ErrorContains(t, err, "nested too deeply")
	require.Less(t, time.Since(start), time.Second)
}

// A field that the decoder does not know is skipped by the length of its value, and the length
// that the encoder writes for a value that holds a definition is the length of its first part. The
// decoder then finds the end of the value in the wrong place, and reads what is after the end as
// what follows the field. The checker writes such a stream again, with exact lengths, so that the
// skip ends where the value does.
func Test_Normalize_FixesTheLengthOfAValueThatIsSkipped(t *testing.T) {
	t.Parallel()
	old := encode(t, map[any]any{
		"k":     holderOld{A: 7, W: evoOld{X: 1, Y: inner{P: 1, Q: 2}, Z: "tail"}, B: "bee"},
		"after": "zzz",
	})
	// The code that reads it has dropped the field W: the stream names the type that has it, and
	// the type of the code has not.
	raw := bytes.Replace(old, []byte("gobcheckHolderAAAA"), []byte("gobcheckHolderBBBB"), 1)
	_, err := decode(raw)
	require.Error(t, err, "gob skips the value too short, and takes what is after it for the next field")

	checked, err := Normalize(raw)
	require.NoError(t, err)
	require.NotEqual(t, raw, checked)
	got, err := decode(checked)
	require.NoError(t, err)
	require.Equal(t, map[any]any{"k": holderNew{A: 7, B: "bee"}, "after": "zzz"}, got)
}

// seeds are the streams that the tests change: what a session looks like, and a stream that gob
// skips a field of.
func seeds(tb testing.TB) map[string][]byte {
	tb.Helper()
	out := map[string][]byte{}
	for name, values := range valid(tb) {
		if name == "deep" {
			continue // a hundred levels is a few thousand bytes to change one by one
		}
		out[name] = encode(tb, values)
	}
	out["a type made of many types"] = chain(idInt, maxTypeCost-2, 40)
	out["a skipped field"] = bytes.Replace(encode(tb, map[any]any{
		"k":     holderOld{A: 7, W: evoOld{X: 1, Y: inner{P: 1, Q: 2}, Z: "tail"}, B: "bee"},
		"after": "zzz",
	}), []byte("gobcheckHolderAAAA"), []byte("gobcheckHolderBBBB"), 1)
	return out
}

// changes calls visit with the stream changed in one byte, to each of a few other values, for each
// byte.
func changes(raw []byte, visit func(changed []byte)) {
	for at := range raw {
		for _, b := range []byte{0x00, 0x7f, 0xf8, 0xff, raw[at] ^ 1, raw[at] + 1} {
			changed := bytes.Clone(raw)
			changed[at] = b
			visit(changed)
		}
	}
}

// A stream is as long as it is, so what the checker lets through takes the decoder a time and a
// memory that are those of its length. This reads streams that are changed one byte at a time.
//
// conventions:ignore parallel: it measures the memory of the whole process, which a test that runs at the same time would add to
func Test_Normalize_WhatItLetsThroughCostsLittle(t *testing.T) {
	for name, raw := range seeds(t) {
		t.Run(name, func(t *testing.T) {
			changes(raw, func(changed []byte) { requireCheapOrRefused(t, changed) })
		})
	}
}

// verdict reads a stream as Normalize does, the first time through or the second, and says whether
// it was refused and whether it needs to be written again.
func verdict(raw []byte, emit bool) (refused, again bool) { //nolint:nonamedreturns // the deferred call sets refused
	c := new(checker)
	defer func() {
		if recover() != nil {
			refused = true
		}
	}()
	c.start(raw, emit)
	c.stream()
	return false, c.canon
}

// The first reading takes the shortcut for values of a few kinds, and the second does not, which it
// cannot, since it writes the stream out. They have to agree on every stream.
func Test_Normalize_TheShortcutAgreesWithTheReadingOfEverythingElse(t *testing.T) {
	t.Parallel()
	for name, raw := range seeds(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			check := func(stream []byte) {
				refused, rewrite := verdict(stream, false)
				slowRefused, slowRewrite := verdict(stream, true)
				require.Equal(t, slowRefused, refused, "% x", stream)
				require.Equal(t, slowRewrite, rewrite, "% x", stream)
			}
			check(raw)
			changes(raw, check)
		})
	}
}

// randomValue is a value of a session, of kinds that the encoder writes in the ways that it has:
// every kind of builtin, structs, slices, maps and arrays, which hold values of any kind, so that
// types are defined where the values that have them are, and before.
func randomValue(r *rand.Rand, depth int) any {
	kinds := 17
	if depth <= 0 {
		kinds = 9
	}
	switch r.IntN(kinds) {
	case 0:
		return r.Int64N(1<<40) - 1<<39
	case 1:
		return strings.Repeat("s", r.IntN(300))
	case 2:
		return r.IntN(2) == 0
	case 3:
		return float64(r.IntN(1000)) / 8
	case 4:
		return append([]byte{1}, make([]byte, r.IntN(200))...)
	case 5:
		return uint(r.Uint64N(1 << 50))
	case 6:
		return time.Date(2000+r.IntN(100), time.Month(1+r.IntN(12)), 1+r.IntN(28), r.IntN(24), 0, 0, 0, time.UTC)
	case 7:
		return unit{A: r.IntN(100)}
	case 8:
		return inner{P: r.IntN(1000), Q: -r.IntN(1000)}
	case 9:
		items := make([]any, 1+r.IntN(4))
		for i := range items {
			items[i] = randomValue(r, depth-1)
		}
		return items
	case 10:
		m := map[string]any{}
		for i := range 1 + r.IntN(4) {
			m[strings.Repeat("k", i+1)] = randomValue(r, depth-1)
		}
		return m
	case 11:
		return evoOld{X: r.IntN(50), Y: randomValue(r, depth-1), Z: "z"}
	case 12:
		return holder{In: randomValue(r, depth-1), Items: []any{randomValue(r, depth-1)}, Arr: [3]int{1, 2, r.IntN(9)}}
	case 13:
		return plain{A: r.IntN(9), B: "b", C: []byte{2}}
	case 14:
		return map[string]int{"a": r.IntN(9), "b": 2}
	case 15:
		return evil{Extra: map[unit]unit{{A: r.IntN(9)}: {A: 1}}, Key: "k"}
	default:
		return plain{A: r.IntN(9), D: 0.5}
	}
}

// A stream that defines more types than are looked up by going through them is looked up in a map,
// and reads the same.
func Test_Normalize_LetsThroughAStreamOfManyTypes(t *testing.T) {
	t.Parallel()
	values := map[any]any{}
	for i := range 3 * scanTypes {
		typ := reflect.StructOf([]reflect.StructField{
			{Name: "X", Type: reflect.TypeFor[int]()},
			{Name: fmt.Sprintf("F%d", i), Type: reflect.TypeFor[string]()},
		})
		v := reflect.New(typ).Elem()
		v.Field(0).SetInt(int64(i))
		gob.RegisterName(fmt.Sprintf("gobcheckMany%d", i), v.Interface())
		values[i] = v.Interface()
	}
	raw := encode(t, values)
	checked, err := Normalize(raw)
	require.NoError(t, err)
	got, err := decode(checked)
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(values, got))
	// Cut short, the types that are looked up in the map are still cut short.
	for n := len(raw) - 1; n > len(raw)-40; n-- {
		_, err = Normalize(raw[:n])
		require.ErrorIs(t, err, ErrInvalid)
	}
}

// A type that is defined twice is refused, as the decoder refuses it.
func Test_Normalize_RefusesATypeDefinedTwice(t *testing.T) {
	t.Parallel()
	messages := frames(t, encode(t, map[any]any{"k": "v"}))
	require.Len(t, messages, 2, "the type of the map, the value")
	_, err := Normalize(join(messages[0], messages[0], messages[1]))
	require.ErrorContains(t, err, "a type is defined twice")
	// And again, from what was remembered of the first.
	_, err = Normalize(join(messages[0], messages[0], messages[1]))
	require.ErrorContains(t, err, "a type is defined twice")
}

// signed writes a signed integer as gob does: twice the number, and the complement of that, for a
// negative one, with the low bit set.
func signed(i int64) []byte {
	if i < 0 {
		return appendUint(nil, uint64(^i)<<1|1)
	}
	return appendUint(nil, uint64(i)<<1)
}

// What follows writes streams by hand, as the encoder writes them a field at a time, so that each of
// the ways in which a stream can be wrong is a stream that is wrong in that way alone.

// cat joins the parts of a stream, lit writes bytes as they are, str a string as gob does, and
// uns an unsigned integer.
func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
func lit(b ...byte) []byte       { return b }
func str(s string) []byte        { return append(appendUint(nil, uint64(len(s))), s...) }
func uns(x uint64) []byte        { return appendUint(nil, x) }

// own is the id that the types that are written by hand have.
const own = firstUserID

// identity is what every definition of a type starts with: its name and its id.
func identity(id int64) []byte {
	return cat(lit(0x01, 0x01), str("T"), lit(0x01), signed(id), lit(0x00))
}

// The definitions of a map, a slice, an array of integers of the type own, a struct that has a field of each of the
// types it is given, and a type that writes itself, which is a wire type of the kind that field number
// says.
func mapDef(id, key, elem int64) []byte {
	return cat(lit(0x04), identity(id), lit(0x01), signed(key), lit(0x01), signed(elem), lit(0x00, 0x00))
}

func sliceDef(id, elem int64) []byte {
	return cat(lit(0x02), identity(id), lit(0x01), signed(elem), lit(0x00, 0x00))
}

func arrayDef(length int64) []byte {
	return cat(lit(0x01), identity(own), lit(0x01), signed(idInt), lit(0x01), signed(length), lit(0x00, 0x00))
}

func structDef(id int64, fields ...int64) []byte {
	def := cat(lit(0x03), identity(id), lit(0x01), uns(uint64(len(fields))))
	for _, f := range fields {
		def = cat(def, lit(0x01), str("F"), lit(0x01), signed(f), lit(0x00))
	}
	return cat(def, lit(0x00, 0x00))
}

func encoderDef(id int64, wireField byte) []byte {
	return cat(lit(wireField), identity(id), lit(0x00, 0x00))
}

// typedefMessage and valueMessage are the messages of a stream: the definition of the type id, and a
// value of it, which holds the parts that are given.
func typedefMessage(id int64, def []byte) []byte { return join(cat(signed(-id), def)) }

func valueMessage(id int64, parts ...[]byte) []byte { return join(cat(signed(id), cat(parts...))) }

// ifaceOf is a value that an interface holds, of a type that gob has: the name it is registered by,
// the id of its type, and the length of the value, which is a zero and the parts that are given.
func ifaceOf(name string, id int64, parts ...[]byte) []byte {
	value := cat(lit(0x00), cat(parts...))
	return cat(str(name), signed(id), uns(uint64(len(value))), value)
}

// A slice of interfaces of the type own, and a value of it that holds the entries that are given.
func entries(items ...[]byte) []byte {
	return cat(typedefMessage(own, sliceDef(own, idInterface)), valueMessage(own, lit(0x00), uns(uint64(len(items))), cat(items...)))
}

// Test_Normalize_RefusesEachWayAStreamCanBeWrong has a stream for every reason that a stream is
// refused for, wrong in that way alone, and each is refused for it. The streams that the encoder
// writes are in the tests above, and what the checker refuses of them when they are cut or changed.
func Test_Normalize_RefusesEachWayAStreamCanBeWrong(t *testing.T) {
	t.Parallel()
	typed := func(id int64, def, value []byte) []byte { return cat(typedefMessage(id, def), value) }
	empty := valueMessage(own, lit(0x00, 0x00))
	for name, tc := range map[string]struct {
		reason string
		stream []byte
	}{
		// The messages and the integers.
		"no data":                    {"the data ends in the middle of a value", nil},
		"an empty message":           {"the data ends in the middle of a value", lit(0x00)},
		"a message longer than data": {"the length of a message does not fit the data", lit(0x03, 0x04, 0x00)},
		"a length that is cut":       {"the data ends in the middle of a value", lit(0xfe, 0x01)},
		"an integer that the end of its message cuts": {"the data ends in the middle of a value", cat(join(cat(signed(idInt), lit(0x00, 0xfe, 0x01))), lit(0x05))},
		"an integer of nine bytes":                    {"an unsigned integer is out of range", join(cat(signed(idInt), lit(0x00, 0xf7), make([]byte, 9)))},
		"a type id that is too large":                 {"a type id is out of range", join(signed(math.MaxInt32 + 1))},
		"a type id that is too small":                 {"a type id is out of range", join(signed(math.MinInt32))},
		"a value that has no type":                    {"a value is of a type that was not defined", join(cat(signed(own), lit(0x00, 0x00)))},
		"a value of the largest id":                   {"a value is of a type that was not defined", join(cat(signed(math.MaxInt32), lit(0x00, 0x00)))},
		"a message that has more than its value":      {"a message holds more than its value", join(cat(signed(idInt), lit(0x00, 0x02, 0x07)))},
		"a number that is not behind a zero":          {"a value that is not a struct does not start with a zero", join(cat(signed(idInt), lit(0x01, 0x02)))},
		"a string that its message cuts":              {"a string is longer than the data that is left", cat(join(cat(signed(idString), lit(0x00, 0x02, 'a'))), lit('b'))},

		// The definitions of types.
		"a type defined twice, differently": {"a type is defined twice", cat(typedefMessage(own, mapDef(own, idInterface, idInterface)), typedefMessage(own, arrayDef(77)), empty)},
		"a type defined twice, as before":   {"a type is defined twice", cat(typedefMessage(own, mapDef(own, idInterface, idInterface)), typedefMessage(own, mapDef(own, idInterface, idInterface)), empty)},
		"a type of an id that gob keeps":    {"a type is defined with an id that gob keeps", typed(firstUserID-1, mapDef(firstUserID-1, idInterface, idInterface), valueMessage(firstUserID-1, lit(0x00, 0x00)))},
		"a type of no kind":                 {"a type definition is of no kind", join(cat(signed(-own), lit(0x08)))},
		"a definition of two types":         {"a type definition holds more than one type", join(cat(signed(-own), mapDef(own, idInterface, idInterface)[:len(mapDef(own, idInterface, idInterface))-1], lit(0x01)))},
		"a type that writes itself, and has an element": {
			"a type definition has a field it should not",
			join(cat(signed(-own), lit(0x05), identity(own), lit(0x01), signed(idInterface), lit(0x00, 0x00))),
		},
		"a slice that has the elements of a map": {
			"a type definition has a field it should not",
			join(cat(signed(-own), lit(0x02), identity(own), lit(0x01), signed(idInterface), lit(0x01), signed(idInterface), lit(0x00, 0x00))),
		},
		"a map that has a third type": {
			"a type definition has a field it should not",
			join(cat(signed(-own), lit(0x04), identity(own), lit(0x01), signed(idInterface), lit(0x01), signed(idInterface), lit(0x01), signed(idInterface), lit(0x00, 0x00))),
		},
		"a common part that has a third field": {
			"the common part of a type definition has a field it should not",
			join(cat(signed(-own), lit(0x04, 0x01, 0x01), str("T"), lit(0x01), signed(own), lit(0x01, 0x00))),
		},
		"a common part that has an id out of range": {
			"a type id is out of range",
			join(cat(signed(-own), lit(0x04, 0x01, 0x01), str("T"), lit(0x01), signed(1<<31), lit(0x00))),
		},
		"an array of a negative length": {"an array has a negative length", join(cat(signed(-own), arrayDef(-1)))},
		"a struct that has a field of no type": {
			"a field of a struct has no type",
			typed(own, cat(lit(0x03), identity(own), lit(0x01), uns(1), lit(0x01), str("F"), lit(0x00, 0x00, 0x00)), valueMessage(own, lit(0x00))),
		},
		"a struct that has a part of a field it should not": {
			"a field of a struct has a part it should not",
			join(cat(signed(-own), lit(0x03), identity(own), lit(0x01), uns(1), lit(0x01), str("F"), lit(0x01), signed(idInt), lit(0x01, 0x00))),
		},
		"a struct that claims more fields than there are bytes": {
			"a struct has more fields than there are bytes left",
			join(cat(signed(-own), lit(0x03), identity(own), lit(0x01), uns(1))),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// The second time is from what the first remembered, if it remembered anything.
			for range 2 {
				_, err := Normalize(tc.stream)
				require.ErrorIs(t, err, ErrInvalid)
				require.ErrorContains(t, err, tc.reason)
			}
		})
	}

	// A type that refers to a type id that no type can have: none, a gap, and a negative one, by the
	// elements of a slice and by the field of a struct.
	for _, id := range []int64{0, 9, firstUserID - 1, -1} {
		for name, def := range map[string][]byte{"a slice": sliceDef(own, id), "a struct": structDef(own, id)} {
			t.Run(fmt.Sprintf("%s of %d", name, id), func(t *testing.T) {
				t.Parallel()
				_, err := Normalize(join(cat(signed(-own), def)))
				require.ErrorContains(t, err, "a type refers to a type id that is not one")
			})
		}
	}

	// The claims of the values.
	values := map[string]struct {
		reason string
		stream []byte
	}{
		"an array that is shorter than its type": {
			"an array does not have the length of its type",
			typed(own, arrayDef(3), valueMessage(own, lit(0x00), uns(2), lit(0x02, 0x04))),
		},
		"an array that has all the length it claims": {
			"an array does not have the length of its type",
			typed(own, arrayDef(1<<40), valueMessage(own, lit(0x00), uns(1<<40))),
		},
		"a slice that has not an element": {"a slice claims more elements than there are bytes left", typed(own, sliceDef(own, idInt), valueMessage(own, lit(0x00), uns(1)))},
		"a slice that claims 2^40":        {"a slice claims more elements than there are bytes left", typed(own, sliceDef(own, idInt), valueMessage(own, lit(0x00), uns(1<<40)))},
		"a map that has not an entry":     {"a map claims more entries than there are bytes left", typed(own, mapDef(own, idInt, idInt), valueMessage(own, lit(0x00), uns(1)))},
		"a field that is not there":       {"a field number is out of range", typed(own, structDef(own, idInt), valueMessage(own, lit(0x02)))},
		"a field of a type that is not": {
			"a value is of a type that was not defined",
			typed(own, structDef(own, own+1), valueMessage(own, lit(0x01, 0x00, 0x00))),
		},
		"a name that is too long":      {"the name of a type is not one", entries(cat(uns(maxNameLen+1), bytes.Repeat([]byte{'a'}, maxNameLen+1)))},
		"a name that its message cuts": {"the name of a type is not one", entries(cat(uns(10), lit('a')))},
		"an interface string that its message cuts": {
			"a string is longer than the data that is left",
			entries(cat(str("string"), signed(idString), uns(4), lit(0x00, 0x02, 'a'))),
		},
		"an interface string that is not there": {
			"a string is longer than the data that is left",
			entries(cat(str("string"), signed(idString), uns(102), lit(0x00, 0x64, 'a', 'b'))),
		},
		"an interface that has not its value": {
			"the data ends in the middle of a value",
			entries(cat(str("int"), signed(idInt), uns(1), lit(0x00))),
		},
		"an interface number that its message cuts": {
			"the data ends in the middle of a value",
			entries(cat(str("int"), signed(idInt), uns(9), lit(0x00, 0xf9, 1, 2, 3, 4, 5, 6))),
		},
		"an interface number of nine bytes": {
			"an unsigned integer is out of range",
			entries(cat(str("int"), signed(idInt), uns(11), lit(0x00, 0xf7), make([]byte, 9))),
		},
	}
	for name, tc := range values {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for range 2 {
				_, err := Normalize(tc.stream)
				require.ErrorIs(t, err, ErrInvalid)
				require.ErrorContains(t, err, tc.reason)
			}
		})
	}
}

// A definition that was read before is not read again, and the id it was read under still has to be
// one that a type can have.
func Test_Normalize_ARememberedDefinitionDoesNotMakeAnIdValid(t *testing.T) {
	t.Parallel()
	def := mapDef(own, idInterface, idInterface)
	for range 2 {
		_, err := Normalize(cat(typedefMessage(own, def), valueMessage(own, lit(0x00, 0x00))))
		require.NoError(t, err)
	}
	_, err := Normalize(cat(typedefMessage(9, def), valueMessage(9, lit(0x00, 0x00))))
	require.ErrorContains(t, err, "a type is defined with an id that gob keeps")
}

// What is at the edge of what the checker takes is taken: the longest name, an array of the length
// of its type, values nested as deeply as they may be, and every kind of type that writes itself.
func Test_Normalize_TakesWhatIsAtTheEdges(t *testing.T) {
	t.Parallel()
	nest := func(levels int) []byte {
		return cat(typedefMessage(own, structDef(own, own)), valueMessage(own, bytes.Repeat(lit(0x01), levels), bytes.Repeat(lit(0x00), levels+1)))
	}
	for name, stream := range map[string][]byte{
		"a name of the longest length":       entries(ifaceOf(strings.Repeat("a", maxNameLen), idString, str("x"))),
		"an array of the length of its type": cat(typedefMessage(own, arrayDef(2)), valueMessage(own, lit(0x00), uns(2), lit(0x02, 0x04))),
		"values nested as deeply as may be":  nest(maxDepth),
		"a type that writes itself":          cat(typedefMessage(own, encoderDef(own, 0x05)), valueMessage(own, lit(0x00), str("x"))),
		"one that is a binary marshaler":     cat(typedefMessage(own, encoderDef(own, 0x06)), valueMessage(own, lit(0x00), str("x"))),
		"one that is a text marshaler":       cat(typedefMessage(own, encoderDef(own, 0x07)), valueMessage(own, lit(0x00), str("x"))),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checked, err := Normalize(stream)
			require.NoError(t, err)
			require.Equal(t, stream, checked)
		})
	}
	_, err := Normalize(nest(maxDepth + 1))
	require.ErrorContains(t, err, "the values are nested too deeply")
}

// What a stream defines is remembered for the streams that follow, which define the same types, but
// only the first types of a stream and only short ones, so that a stream that defines thousands of
// types, or long ones, neither replaces the list again and again nor fills it with what no other
// stream has.
func Test_Normalize_RemembersOnlyTheFirstShortTypesOfAStream(t *testing.T) {
	t.Parallel()
	var many []byte
	for i := range 3 * maxCached {
		many = append(many, typedefMessage(1000+int64(i), sliceDef(1000+int64(i), idInt))...)
	}
	_, err := Normalize(cat(many, valueMessage(1000, lit(0x00, 0x00))))
	require.NoError(t, err)

	fields := make([]int64, 100)
	for i := range fields {
		fields[i] = idInt
	}
	long := structDef(2000, fields...)
	require.Greater(t, len(long), maxCachedBody)
	_, err = Normalize(cat(typedefMessage(2000, long), valueMessage(2000, lit(0x00))))
	require.NoError(t, err)

	list := typeCache.Load()
	require.NotNil(t, list, "the first types of the first stream")
	for _, cached := range *list {
		require.NotEqual(t, typeID(2000), cached.id, "a definition that is too long")
		if cached.id >= 1000 && cached.id < 2000 {
			require.Less(t, cached.id, typeID(1000+maxCached), "a type after the first ones of its stream")
		}
	}
}

// chain is the stream of a session that holds, under the key "k", a value of a struct that has f
// fields of a type that is made of n types: a chain of n slices, each of the one before and the first
// of the type base. The struct is registered by the name gobcheckLacks, which has none of its fields,
// so the decoder skips all of them, and goes through the n types of the chain for each. The value
// itself is empty.
func chain(base int64, n, f int) []byte {
	types := typedefMessage(own, mapDef(own, idInterface, idInterface))
	for i := range n {
		id := int64(own + 1 + i)
		elem := base
		if i > 0 {
			elem = id - 1
		}
		types = append(types, typedefMessage(id, sliceDef(id, elem))...)
	}
	fields := make([]int64, f)
	for i := range fields {
		fields[i] = int64(own + n)
	}
	sid := int64(own + n + 1)
	return cat(types, typedefMessage(sid, structDef(sid, fields...)),
		valueMessage(own, lit(0x00), uns(1), ifaceOf("string", idString, str("k")), str("gobcheckLacks"), signed(sid), uns(1), lit(0x00)))
}

// The decoder skips a field that its type lacks by going through the types the field is made of, and
// starts again for the next field: a struct of f fields of a type made of n types takes n times f.
// Both are as large as the stream makes them, so a stream of 22 KB took 0.4 s and 44 MB, one of
// 93 KB 2.4 s and 628 MB, and the cost grows with the square of the length. A type may be made of
// a few types, and a stream may define a few thousand.
func Test_Normalize_RefusesTypesThatTheDecoderGoesThroughAgainForEachField(t *testing.T) {
	t.Parallel()
	const reason = "a type is made of more types than the decoder goes through to skip a value of it"
	_, err := Normalize(chain(idInt, maxTypeCost, 2000))
	require.NoError(t, err, "a chain of the most that is let through")

	for name, stream := range map[string][]byte{
		"a chain one longer":                chain(idInt, maxTypeCost+1, 3),
		"a chain of interfaces, one longer": chain(idInterface, maxTypeCost, 3),
		"the chain of 22 KB":                chain(idInt, 800, 800),
		"types that are shared":             dag(4),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Normalize(stream)
			require.ErrorIs(t, err, ErrInvalid)
			require.ErrorContains(t, err, reason)
		})
	}

	// Types that are made of the same type twice cost it twice, and a type that refers to itself, or to
	// the one that refers to it, costs what it does the first time: the decoder goes through it once.
	for name, stream := range map[string][]byte{
		"types that are shared, within the limit":  dag(3),
		"a chain of interfaces, as long as may be": chain(idInterface, maxTypeCost-1, 3),
		"a chain over a type that is not defined":  chain(own+1000, maxTypeCost, 3),
		"a slice of itself":                        cat(typedefMessage(own, sliceDef(own, own)), valueMessage(own, lit(0x00, 0x00))),
		"slices of each other": cat(typedefMessage(own, sliceDef(own, own+1)), typedefMessage(own+1, sliceDef(own+1, own)),
			valueMessage(own, lit(0x00, 0x00))),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Normalize(stream)
			require.NoError(t, err)
		})
	}

	t.Run("as many types as are let through", func(t *testing.T) {
		t.Parallel()
		many := func(n int) []byte {
			var types []byte
			for i := range n {
				id := int64(own + i)
				types = append(types, typedefMessage(id, sliceDef(id, idInt))...)
			}
			return cat(types, valueMessage(own, lit(0x00, 0x00)))
		}
		_, err := Normalize(many(maxTypes))
		require.NoError(t, err)
		_, err = Normalize(many(maxTypes + 1))
		require.ErrorContains(t, err, "the stream defines too many types")
	})
}

// A chain of the most that is let through, with as many fields as a stream of its length can hold,
// costs the decoder time and memory in proportion to the length, by the factor that the limit allows.
//
// conventions:ignore parallel: it measures the memory of the whole process, which a test that runs at the same time would add to
func Test_Normalize_WhatTheLongestChainCostsTheDecoder(t *testing.T) {
	requireCheapOrRefused(t, chain(idInt, maxTypeCost, 2000))
}

// dag is a stream of the types T0, a slice of integers, and Ti, a map of T(i-1) to T(i-1), which cost
// twice what the one before does, and of an empty value of the last.
func dag(levels int) []byte {
	types := typedefMessage(own, sliceDef(own, idInt))
	for i := 1; i <= levels; i++ {
		id := int64(own + i)
		types = append(types, typedefMessage(id, mapDef(id, id-1, id-1))...)
	}
	return cat(types, valueMessage(int64(own+levels), lit(0x00, 0x00)))
}

// The length of a value that an interface holds is how far a decoder that skips the field goes, so
// a stream whose lengths are not what they precede is written out again with the right ones: too
// short, as the encoder writes it when the value holds a definition, or too long.
func Test_Normalize_WritesAgainALengthThatIsTooLongOrTooShort(t *testing.T) {
	t.Parallel()
	for name, size := range map[string]uint64{"too long": 4, "too short": 2} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stream := entries(cat(str("string"), signed(idString), uns(size), lit(0x00, 0x01, 'a')), lit(0x00))
			checked, err := Normalize(stream)
			require.NoError(t, err)
			require.NotEqual(t, stream, checked)
			var got []any
			require.NoError(t, gob.NewDecoder(bytes.NewReader(checked)).Decode(&got))
			require.Equal(t, []any{"a", nil}, got)
			again, err := Normalize(checked)
			require.NoError(t, err)
			require.Equal(t, checked, again)
		})
	}
}

// The shortcut for the numbers, booleans and strings that an interface holds is for streams that are
// what the encoder writes, and says so when one is not. At every edge of what it looks at, it takes
// what the reading of everything else takes, and no more.
func Test_Normalize_TheShortcutTakesWhatTheReadingOfEverythingElseTakes(t *testing.T) {
	t.Parallel()
	var sizes []byte
	for size := range byte(15) {
		sizes = append(sizes, size)
	}
	sizes = append(sizes, 0x7f, 0x80)
	for _, id := range []byte{0x00, 0x02, 0x04, 0x06, 0x08, 0x0c, 0x0e, 0x10, 0x14, 0x80} {
		for _, size := range sizes {
			for _, third := range []byte{0x00, 0x01} {
				for _, first := range []byte{0x00, 0x01, 0x02, 0x03, 0x05, 0x7f, 0x80, 0xf7, 0xf8, 0xf9} {
					for _, fill := range []int{0, 1, 2, 3, 4, 8, 9, 10, 12} {
						stream := entries(cat(str("x"), lit(id, size, third, first), bytes.Repeat(lit(0xab), fill)))
						refused, again := verdict(stream, false)
						slowRefused, slowAgain := verdict(stream, true)
						if refused != slowRefused || again != slowAgain {
							t.Fatalf("the shortcut and the reading of everything else differ on % x: %v %v against %v %v", stream, refused, again, slowRefused, slowAgain)
						}
					}
				}
			}
		}
	}
}

// A session of a few thousand values, longer than the buffers that are kept for the next stream, is
// let through, with the definitions that lie inside its values written out in front, and reads back as
// it was.
func Test_Normalize_LetsThroughALargeSession(t *testing.T) {
	t.Parallel()
	values := map[any]any{}
	for i := range 3000 {
		switch i % 3 {
		case 0:
			values[i] = fmt.Sprintf("value number %d", i)
		case 1:
			values[i] = plain{A: i, B: "b", C: []byte{1, 2, 3}}
		default:
			values[i] = map[string]any{"a": []any{i, "x"}}
		}
	}
	raw := encode(t, values)
	require.Greater(t, len(raw), 1<<16)
	checked, err := Normalize(raw)
	require.NoError(t, err)
	require.NotEqual(t, raw, checked)
	got, err := decode(checked)
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(values, got))
}

// Whatever values a session holds, in whatever order the types in them come, the stream is let
// through and reads back as the values were.
func Test_Normalize_LetsThroughValuesOfEveryShape(t *testing.T) {
	t.Parallel()
	rewritten := 0
	for seed := range uint64(500) {
		r := rand.New(rand.NewPCG(seed, 7))
		values := map[any]any{}
		for i := range 1 + r.IntN(4) {
			values[strings.Repeat("v", i+1)] = randomValue(r, 4)
		}
		raw := encode(t, values)
		checked, err := Normalize(raw)
		require.NoError(t, err, "seed %d", seed)
		got, err := decode(checked)
		require.NoError(t, err, "seed %d", seed)
		require.True(t, reflect.DeepEqual(values, got), "seed %d\n%v\n%v", seed, values, got)
		if !bytes.Equal(raw, checked) {
			rewritten++
			again, err := Normalize(checked)
			require.NoError(t, err)
			require.Equal(t, checked, again, "seed %d: what was written again needs no more", seed)
		}
	}
	require.Positive(t, rewritten, "streams that define types inside values")
	require.Less(t, rewritten, 500, "streams that do not")
}

// requireCheapOrRefused checks a stream, and decodes it if it is let through. It then has to take
// no more than a time and a memory that go with its length.
func requireCheapOrRefused(tb testing.TB, raw []byte) {
	tb.Helper()
	checked, err := Normalize(raw)
	if err != nil {
		require.ErrorIs(tb, err, ErrInvalid)
		require.NotContains(tb, err.Error(), "internal error", "a stream that is not one made the checker fail")
		return
	}
	if !bytes.Equal(raw, checked) {
		again, err := Normalize(checked)
		require.NoError(tb, err, "a stream that was written again is refused: % x", raw)
		require.Equal(tb, checked, again, "a stream that was written again needs more: % x", raw)
	}
	done := make(chan struct{})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	go func() {
		defer close(done)
		defer func() { _ = recover() }() //nolint:errcheck // a key that cannot be hashed is a panic of gob, which the session turns into an error
		_, _ = decode(checked)           //nolint:errcheck // it fails for most of what is changed
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		tb.Fatalf("the decoder was still reading a stream of %d bytes after 5 seconds: % x", len(raw), raw)
	}
	runtime.ReadMemStats(&after)
	limit := uint64(len(checked))*1024 + 4<<20
	require.LessOrEqual(tb, after.TotalAlloc-before.TotalAlloc, limit, "the decoder took too much memory for a stream of %d bytes: % x", len(raw), raw)
}

// FuzzNormalize lets the fuzzer change streams, and holds the checker to what it promises: it does
// not fail on any, and the decoder does little with one that it lets through.
func FuzzNormalize(f *testing.F) {
	for _, raw := range seeds(f) {
		f.Add(raw)
	}
	f.Add(join([]byte{0x7f, 0x04, 0x01, 0x02, 0xff, 0x80, 0x00, 0x01, 0x10, 0x01, 0x10, 0x00, 0x00}, append([]byte{0xff, 0x80, 0x00}, bigCount...)))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<12 {
			t.Skip()
		}
		requireCheapOrRefused(t, raw)
	})
}

// A stream that is refused is refused in words that do not come from it.
func Test_Normalize_ErrorsDoNotQuoteTheStream(t *testing.T) {
	t.Parallel()
	_, err := Normalize([]byte("\x05evil\n"))
	require.ErrorIs(t, err, ErrInvalid)
	require.False(t, strings.ContainsAny(err.Error(), "\n\x00"))
	require.NotContains(t, err.Error(), "evil")
}

func Benchmark_Normalize(b *testing.B) {
	for name, values := range map[string]map[any]any{
		"a few scalars": {"user": "alice", "id": 42, "admin": true},
		"a document":    {"doc": map[string]any{"a": []any{1, "x", map[string]any{"b": []any{2.5, true}}}, "c": map[string]any{"d": "e"}}},
		"a struct":      {"p": plain{A: 1, B: "b", C: []byte{1, 2}, D: 2.5}},
	} {
		raw := encode(b, values)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Normalize(raw); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
