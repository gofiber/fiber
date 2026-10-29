package session

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testRecordMagic is the tag and the version of a record.
const testRecordMagic = recordTag + "\x01"

// collectRecord parses b and returns the header and the entries, with the values copied.
func collectRecord(b []byte) (recordHeader, []recordEntry, error) {
	var entries []recordEntry
	h, err := parseRecord(b, func(key, value []byte) {
		entries = append(entries, recordEntry{key: string(key), value: slices.Clone(value)})
	})
	return h, entries, err
}

func mustAppendRecord(tb testing.TB, codec string, deadline recordDeadline, entries []recordEntry) []byte {
	tb.Helper()
	b, err := appendRecord(nil, codec, deadline, entries)
	require.NoError(tb, err)
	return b
}

func Test_Record_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		entries  []recordEntry
		deadline recordDeadline
	}{
		{name: "empty"},
		{name: "one entry", entries: []recordEntry{{key: "a", value: []byte("1")}}},
		{name: "empty key and value", entries: []recordEntry{{key: "", value: []byte{}}, {key: "a", value: []byte{}}}},
		{name: "binary", entries: []recordEntry{{key: "\x00\xff", value: []byte{0, 1, 2, 255}}, {key: "\xff", value: []byte{0}}}},
		{name: "deadline", deadline: recordDeadline{sec: 1_700_000_000, nanos: 999_999_999, set: true}},
		{name: "deadline at the epoch", deadline: recordDeadline{set: true}},
		{name: "deadline before the epoch", deadline: recordDeadline{sec: minDeadlineSec + 1, set: true}},
		{name: "deadline at the last second of the year 9999", deadline: recordDeadline{sec: maxDeadlineSec, nanos: 999_999_999, set: true}},
		{name: "deadline a nanosecond after the zero time", deadline: recordDeadline{sec: minDeadlineSec, nanos: 1, set: true}},
		{name: "long value", entries: []recordEntry{{key: "k", value: bytes.Repeat([]byte("x"), 70_000)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded := mustAppendRecord(t, "json", tc.deadline, tc.entries)
			require.True(t, isRecord(encoded))

			h, entries, err := collectRecord(encoded)
			require.NoError(t, err)
			require.Equal(t, "json", string(h.codec))
			require.Equal(t, tc.deadline, h.deadline)
			require.Len(t, entries, len(tc.entries))
			for i := range entries {
				require.Equal(t, tc.entries[i].key, entries[i].key)
				require.True(t, bytes.Equal(tc.entries[i].value, entries[i].value))
			}

			// One record has one spelling.
			again := mustAppendRecord(t, "json", h.deadline, entries)
			require.Equal(t, encoded, again)
		})
	}
}

func Test_Record_AppendKeepsThePrefix(t *testing.T) {
	t.Parallel()

	prefix := []byte("prefix")
	out, err := appendRecord(slices.Clone(prefix), "json", recordDeadline{}, []recordEntry{{key: "a", value: []byte("b")}})
	require.NoError(t, err)
	require.Equal(t, prefix, out[:len(prefix)])

	_, _, err = collectRecord(out[len(prefix):])
	require.NoError(t, err)
}

// uvarintLen and zigzag size the buffer of appendRecord, so they have to agree with
// what encoding/binary writes at the edges of every length.
func Test_Record_VarintSizes(t *testing.T) {
	t.Parallel()

	for shift := range 64 {
		for _, x := range []uint64{1<<shift - 1, 1 << shift, 1<<shift + 1} {
			require.Len(t, binary.AppendUvarint(nil, x), uvarintLen(x), "%d", x)
		}
	}
	require.Len(t, binary.AppendUvarint(nil, math.MaxUint64), uvarintLen(math.MaxUint64))

	for shift := range 63 {
		for _, x := range []int64{1<<shift - 1, 1 << shift, 1<<shift + 1} {
			for _, v := range []int64{x, -x, -x - 1} {
				require.Equal(t, binary.AppendVarint(nil, v), binary.AppendUvarint(nil, zigzag(v)), "%d", v)
			}
		}
	}
	for _, v := range []int64{math.MaxInt64, math.MinInt64} {
		require.Equal(t, binary.AppendVarint(nil, v), binary.AppendUvarint(nil, zigzag(v)), "%d", v)
	}
}

// A record is written into a buffer of exactly its length: no growth, no spare room
// that a storage which keeps the slice would hold on to.
func Test_Record_AppendReservesExactly(t *testing.T) {
	t.Parallel()

	// Each key is a prefix of the next one, which puts them in ascending order.
	sizes := []int{0, 1, 127, 128, 16_383, 16_384}
	var entries []recordEntry
	for _, size := range sizes {
		entries = append(entries, recordEntry{key: strings.Repeat("k", size), value: bytes.Repeat([]byte("v"), size)})
	}
	many := make([]recordEntry, 0, 130)
	for i := range 130 {
		many = append(many, recordEntry{key: fmt.Sprintf("%03d", i), value: []byte{byte(i)}})
	}

	for _, tc := range []struct {
		name     string
		codec    string
		entries  []recordEntry
		deadline recordDeadline
	}{
		{name: "empty", codec: "j"},
		{name: "sizes at the edges of the varints", codec: "json", entries: entries},
		{name: "count at the edge of a varint", codec: "json", entries: many[:127]},
		{name: "count past the edge of a varint", codec: "json", entries: many},
		{name: "long name", codec: strings.Repeat("n", 128)},
		{name: "longest name", codec: strings.Repeat("n", maxCodecNameLen)},
		{name: "deadline", codec: "json", deadline: recordDeadline{sec: 1_700_000_000, nanos: 999_999_999, set: true}},
		{name: "deadline at the epoch", codec: "json", deadline: recordDeadline{set: true}},
		{name: "deadline before the epoch", codec: "json", deadline: recordDeadline{sec: -1, nanos: 1, set: true}},
		{name: "first deadline", codec: "json", deadline: recordDeadline{sec: minDeadlineSec, nanos: 1, set: true}},
		{name: "last deadline", codec: "json", deadline: recordDeadline{sec: maxDeadlineSec, nanos: 999_999_999, set: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out := mustAppendRecord(t, tc.codec, tc.deadline, tc.entries)
			require.Equal(t, len(out), cap(out))
		})
	}
}

func Test_Record_AppendRefusesWhatItCannotParse(t *testing.T) {
	t.Parallel()

	dst := []byte("keep")
	for name, tc := range map[string]struct {
		codec    string
		entries  []recordEntry
		deadline recordDeadline
	}{
		"empty codec":                  {codec: ""},
		"long codec":                   {codec: strings.Repeat("c", maxCodecNameLen+1)},
		"unsorted keys":                {codec: "c", entries: []recordEntry{{key: "b"}, {key: "a"}}},
		"duplicate keys":               {codec: "c", entries: []recordEntry{{key: "a"}, {key: "a"}}},
		"nanoseconds too big":          {codec: "c", deadline: recordDeadline{set: true, nanos: nanosPerSecond}},
		"deadline after the year 9999": {codec: "c", deadline: recordDeadline{set: true, sec: maxDeadlineSec + 1}},
		"deadline before the year 1":   {codec: "c", deadline: recordDeadline{set: true, sec: minDeadlineSec - 1}},
		"deadline that wraps":          {codec: "c", deadline: recordDeadline{set: true, sec: math.MaxInt64}},
		"the zero time":                {codec: "c", deadline: recordDeadline{set: true, sec: minDeadlineSec}},
		"seconds without a deadline":   {codec: "c", deadline: recordDeadline{sec: 5}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := appendRecord(slices.Clone(dst), tc.codec, tc.deadline, tc.entries)
			require.ErrorIs(t, err, errInvalidRecord)
			require.Equal(t, dst, out)
		})
	}

	// The longest name is fine.
	_, err := appendRecord(nil, strings.Repeat("c", maxCodecNameLen), recordDeadline{}, nil)
	require.NoError(t, err)
}

// Every proper prefix of a record is refused, and so is the record with a byte
// added: nothing may be accepted that is not exactly what was written.
func Test_Record_TruncationAndTrailingBytes(t *testing.T) {
	t.Parallel()

	encoded := mustAppendRecord(t, "json", recordDeadline{sec: 123, nanos: 456, set: true},
		[]recordEntry{{key: "a", value: []byte("one")}, {key: "b", value: []byte("two")}, {key: "c", value: nil}})

	for n := range encoded {
		_, _, err := collectRecord(encoded[:n])
		require.ErrorIs(t, err, errInvalidRecord, "prefix of %d bytes", n)
	}
	_, _, err := collectRecord(append(slices.Clone(encoded), 0))
	require.ErrorIs(t, err, errInvalidRecord)
}

func Test_Record_Malformed(t *testing.T) {
	t.Parallel()

	magic := []byte(testRecordMagic)
	build := func(parts ...[]byte) []byte {
		return slices.Concat(append([][]byte{magic}, parts...)...)
	}
	uv := func(v uint64) []byte { return binary.AppendUvarint(nil, v) }
	name := slices.Concat(uv(4), []byte("json"))
	entry := func(key, value string) []byte {
		return slices.Concat(uv(uint64(len(key))), []byte(key), uv(uint64(len(value))), []byte(value))
	}

	for label, b := range map[string][]byte{
		"empty":                               nil,
		"wrong first byte":                    append([]byte{1}, magic[1:]...),
		"tag only":                            []byte(recordTag),
		"gob stream":                          {0x0c, 0xff, 0x81, 0x04, 0x01, 0x02, 0xff, 0x82, 0x00, 0x01, 0x10, 0x01},
		"empty codec name":                    build(uv(0), []byte{0}, uv(0)),
		"codec name too long":                 build(uv(maxCodecNameLen+1), bytes.Repeat([]byte("c"), maxCodecNameLen+1), []byte{0}, uv(0)),
		"codec name past the end":             build(uv(9), []byte("json")),
		"overlong name length":                build([]byte{0x84, 0x00}, []byte("json"), []byte{0}, uv(0)),
		"unknown deadline flag":               build(name, []byte{2}, uv(0)),
		"deadline without seconds":            build(name, []byte{1}),
		"deadline without nanos":              build(name, []byte{1}, binary.AppendVarint(nil, 5)),
		"nanoseconds out of range":            build(name, []byte{1}, binary.AppendVarint(nil, 5), uv(nanosPerSecond), uv(0)),
		"overlong seconds":                    build(name, []byte{1}, []byte{0x84, 0x00}, uv(0), uv(0)),
		"overlong count":                      build(name, []byte{0}, []byte{0x80, 0x00}),
		"count larger than record":            build(name, []byte{0}, uv(2), entry("a", "b")),
		"count of a billion":                  build(name, []byte{0}, uv(1_000_000_000)),
		"count overflows":                     build(name, []byte{0}, bytes.Repeat([]byte{0xff}, 10)),
		"key longer than record":              build(name, []byte{0}, uv(1), uv(1<<40), []byte("a")),
		"value longer than record":            build(name, []byte{0}, uv(1), uv(1), []byte("a"), uv(1<<40)),
		"deadline after the year 9999":        build(name, []byte{1}, binary.AppendVarint(nil, maxDeadlineSec+1), uv(0), uv(0)),
		"deadline before the year 1":          build(name, []byte{1}, binary.AppendVarint(nil, minDeadlineSec-1), uv(0), uv(0)),
		"the zero time, which is no deadline": build(name, []byte{1}, binary.AppendVarint(nil, minDeadlineSec), uv(0), uv(0)),
		"deadline that wraps":                 build(name, []byte{1}, binary.AppendVarint(nil, math.MaxInt64), uv(0), uv(0)),
		"deadline that wraps the other":       build(name, []byte{1}, binary.AppendVarint(nil, math.MinInt64), uv(0), uv(0)),
		"keys out of order":                   build(name, []byte{0}, uv(2), entry("b", "1"), entry("a", "2")),
		"duplicate keys":                      build(name, []byte{0}, uv(2), entry("a", "1"), entry("a", "2")),
		"entry without value":                 build(name, []byte{0}, uv(1), uv(1), []byte("a")),
		"trailing byte":                       build(name, []byte{0}, uv(1), entry("a", "1"), []byte{0}),
		"trailing byte after empty":           build(name, []byte{0}, uv(0), []byte{0}),
	} {
		_, _, err := collectRecord(b)
		require.ErrorIs(t, err, errInvalidRecord, label)
	}

	// A version that is not known is refused for what it is.
	_, _, err := collectRecord(append(slices.Clone(magic[:len(magic)-1]), 2, 4, 'j', 's', 'o', 'n', 0, 0))
	require.ErrorIs(t, err, errUnsupportedRecord)
	require.ErrorIs(t, err, errInvalidRecord)

	// The same pieces, well formed, are accepted.
	_, entries, err := collectRecord(build(name, []byte{0}, uv(2), entry("a", "1"), entry("b", "2")))
	require.NoError(t, err)
	require.Len(t, entries, 2)
}

// A gob stream is never taken for a record, whatever is in the map.
func Test_Record_GobStreamsDoNotLookLikeRecords(t *testing.T) {
	t.Parallel()

	gob.Register(struct{ A int }{})
	gob.Register(map[string]any{})
	gob.Register([]any{})
	values := []map[any]any{
		{},
		{"a": 1},
		{"a": "b", 1: 2.5, true: []byte("x")},
		{"nested": map[string]any{"k": []any{1, "two", 3.0}}},
		{"big": strings.Repeat("x", 200_000)},
		{"s": struct{ A int }{A: 7}},
	}
	for _, v := range values {
		var buf bytes.Buffer
		require.NoError(t, gob.NewEncoder(&buf).Encode(&v))
		require.NotEmpty(t, buf.Bytes())
		require.NotEqual(t, testRecordMagic[0], buf.Bytes()[0])
		require.False(t, isRecord(buf.Bytes()))
	}
}

func Fuzz_Record_Parse(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte(testRecordMagic))
	f.Add(mustAppendRecord(f, "json", recordDeadline{}, nil))
	f.Add(mustAppendRecord(f, "json", recordDeadline{sec: 1, nanos: 2, set: true},
		[]recordEntry{{key: "", value: nil}, {key: "a", value: []byte("b")}, {key: "b", value: bytes.Repeat([]byte("v"), 300)}}))
	f.Add(slices.Concat([]byte(testRecordMagic), []byte{4}, []byte("json"), []byte{0}, binary.AppendUvarint(nil, 1<<62)))

	f.Fuzz(func(t *testing.T, b []byte) {
		before := slices.Clone(b)
		h, entries, err := collectRecord(b)
		require.Equal(t, before, b, "parsing must not change its input")
		if err != nil {
			require.ErrorIs(t, err, errInvalidRecord)
			return
		}

		// What parses is exactly what appendRecord would have written.
		again, err := appendRecord(nil, string(h.codec), h.deadline, entries)
		require.NoError(t, err)
		require.Equal(t, b, again)

		// No entry is larger than the record it came from.
		total := 0
		for _, e := range entries {
			total += len(e.key) + len(e.value)
		}
		require.LessOrEqual(t, total, len(b))
	})
}

// An entry has no capacity beyond its length: a codec that keeps a slice of the
// record and appends to it cannot overwrite the entry that follows.
func Test_Record_EntriesHaveNoSpareCapacity(t *testing.T) {
	t.Parallel()

	encoded := mustAppendRecord(t, "json", recordDeadline{}, []recordEntry{
		{key: "a", value: []byte("one")}, {key: "b", value: []byte("two")}, {key: "c", value: []byte("three")},
	})
	original := slices.Clone(encoded)

	seen := 0
	_, err := parseRecord(encoded, func(key, value []byte) {
		seen++
		require.Equal(t, len(key), cap(key))
		require.Equal(t, len(value), cap(value))
		// Appending is what is tested: it must copy, and leave the record alone.
		_ = append(value, "overwrite"...)
		_ = append(key, "overwrite"...)
	})
	require.NoError(t, err)
	require.Equal(t, 3, seen)
	require.Equal(t, original, encoded)
}

// Parsing a record takes no memory, whatever the record claims to hold.
func Test_Record_ParseAllocatesNothing(t *testing.T) {
	// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
	valid := mustAppendRecord(t, "json", recordDeadline{sec: 1, set: true}, []recordEntry{
		{key: "a", value: []byte("one")}, {key: "b", value: []byte("two")},
	})
	noop := func(_, _ []byte) {}
	require.Zero(t, testing.AllocsPerRun(100, func() {
		if _, err := parseRecord(valid, noop); err != nil {
			t.Error(err)
		}
	}))

	// A record that claims a billion entries, or entries as long as the address space.
	claims := slices.Concat([]byte(testRecordMagic), []byte{4}, []byte("json"), []byte{0}, binary.AppendUvarint(nil, 1<<30), make([]byte, 1<<20))
	require.Zero(t, testing.AllocsPerRun(10, func() {
		_, err := parseRecord(claims, noop)
		if err == nil {
			t.Error("a record that claims what it does not hold was accepted")
		}
	}))
}

func Test_Record_CodecOf(t *testing.T) {
	t.Parallel()

	encoded := mustAppendRecord(t, "msgpack", recordDeadline{}, nil)
	version, codec, ok := recordCodecOf(encoded)
	require.True(t, ok)
	require.EqualValues(t, recordVersion, version)
	require.Equal(t, "msgpack", string(codec))

	// A version that is not known: no name is read.
	unknown := slices.Clone(encoded)
	unknown[len(recordTag)] = 9
	version, codec, ok = recordCodecOf(unknown)
	require.True(t, ok)
	require.EqualValues(t, 9, version)
	require.Nil(t, codec)

	for name, b := range map[string][]byte{
		"nothing":        nil,
		"a gob stream":   {0x0d, 0x7f, 0x04, 0x01},
		"the tag only":   []byte(recordTag),
		"no name":        []byte(testRecordMagic),
		"a long name":    slices.Concat([]byte(testRecordMagic), binary.AppendUvarint(nil, 99), []byte("short")),
		"an empty name":  slices.Concat([]byte(testRecordMagic), []byte{0}),
		"a name too big": slices.Concat([]byte(testRecordMagic), binary.AppendUvarint(nil, maxCodecNameLen+1), make([]byte, maxCodecNameLen+1)),
	} {
		_, _, ok := recordCodecOf(b)
		require.False(t, ok, name)
	}
}
