package fiber

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestState_SetAndGet_WithApp(t *testing.T) {
	t.Parallel()
	// Create app
	app := New()

	// test setting and getting a value
	app.State().Set("foo", "bar")
	val, ok := app.State().Get("foo")
	require.True(t, ok)
	require.Equal(t, "bar", val)

	// test key not found
	_, ok = app.State().Get("unknown")
	require.False(t, ok)
}

func TestState_SetAndGet(t *testing.T) {
	t.Parallel()
	st := newState()

	// test setting and getting a value
	st.Set("foo", "bar")
	val, ok := st.Get("foo")
	require.True(t, ok)
	require.Equal(t, "bar", val)

	// test key not found
	_, ok = st.Get("unknown")
	require.False(t, ok)
}

func TestState_GetString(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("str", "hello")
	s, ok := st.GetString("str")
	require.True(t, ok)
	require.Equal(t, "hello", s)

	// wrong type should return false
	st.Set("num", 123)
	s, ok = st.GetString("num")
	require.False(t, ok)
	require.Equal(t, "", s)

	// missing key should return false
	s, ok = st.GetString("missing")
	require.False(t, ok)
	require.Equal(t, "", s)
}

func TestState_GetInt(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("num", 456)
	i, ok := st.GetInt("num")
	require.True(t, ok)
	require.Equal(t, 456, i)

	// wrong type should return zero value
	st.Set("str", "abc")
	i, ok = st.GetInt("str")
	require.False(t, ok)
	require.Equal(t, 0, i)

	// missing key should return zero value
	i, ok = st.GetInt("missing")
	require.False(t, ok)
	require.Equal(t, 0, i)
}

func TestState_GetBool(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("flag", true)
	b, ok := st.GetBool("flag")
	require.True(t, ok)
	require.True(t, b)

	// wrong type
	st.Set("num", 1)
	b, ok = st.GetBool("num")
	require.False(t, ok)
	require.False(t, b)

	// missing key should return false
	b, ok = st.GetBool("missing")
	require.False(t, ok)
	require.False(t, b)
}

func TestState_GetFloat64(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("pi", 3.14)
	f, ok := st.GetFloat64("pi")
	require.True(t, ok)
	require.InDelta(t, 3.14, f, 0.0001)

	// wrong type should return zero value
	st.Set("int", 10)
	f, ok = st.GetFloat64("int")
	require.False(t, ok)
	require.InDelta(t, 0.0, f, 0.0001)

	// missing key should return zero value
	f, ok = st.GetFloat64("missing")
	require.False(t, ok)
	require.InDelta(t, 0.0, f, 0.0001)
}

func TestState_GetUint(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("uint", uint(100))
	u, ok := st.GetUint("uint")
	require.True(t, ok)
	require.Equal(t, uint(100), u)

	st.Set("wrong", "not uint")
	u, ok = st.GetUint("wrong")
	require.False(t, ok)
	require.Equal(t, uint(0), u)

	u, ok = st.GetUint("missing")
	require.False(t, ok)
	require.Equal(t, uint(0), u)
}

func TestState_GetInt8(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("int8", int8(10))
	i, ok := st.GetInt8("int8")
	require.True(t, ok)
	require.Equal(t, int8(10), i)

	st.Set("wrong", "not int8")
	i, ok = st.GetInt8("wrong")
	require.False(t, ok)
	require.Equal(t, int8(0), i)

	i, ok = st.GetInt8("missing")
	require.False(t, ok)
	require.Equal(t, int8(0), i)
}

func TestState_GetInt16(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("int16", int16(200))
	i, ok := st.GetInt16("int16")
	require.True(t, ok)
	require.Equal(t, int16(200), i)

	st.Set("wrong", "not int16")
	i, ok = st.GetInt16("wrong")
	require.False(t, ok)
	require.Equal(t, int16(0), i)

	i, ok = st.GetInt16("missing")
	require.False(t, ok)
	require.Equal(t, int16(0), i)
}

func TestState_GetInt32(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("int32", int32(3000))
	i, ok := st.GetInt32("int32")
	require.True(t, ok)
	require.Equal(t, int32(3000), i)

	st.Set("wrong", "not int32")
	i, ok = st.GetInt32("wrong")
	require.False(t, ok)
	require.Equal(t, int32(0), i)

	i, ok = st.GetInt32("missing")
	require.False(t, ok)
	require.Equal(t, int32(0), i)
}

func TestState_GetInt64(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("int64", int64(4000))
	i, ok := st.GetInt64("int64")
	require.True(t, ok)
	require.Equal(t, int64(4000), i)

	st.Set("wrong", "not int64")
	i, ok = st.GetInt64("wrong")
	require.False(t, ok)
	require.Equal(t, int64(0), i)

	i, ok = st.GetInt64("missing")
	require.False(t, ok)
	require.Equal(t, int64(0), i)
}

func TestState_GetUint8(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("uint8", uint8(20))
	u, ok := st.GetUint8("uint8")
	require.True(t, ok)
	require.Equal(t, uint8(20), u)

	st.Set("wrong", "not uint8")
	u, ok = st.GetUint8("wrong")
	require.False(t, ok)
	require.Equal(t, uint8(0), u)

	u, ok = st.GetUint8("missing")
	require.False(t, ok)
	require.Equal(t, uint8(0), u)
}

func TestState_GetUint16(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("uint16", uint16(300))
	u, ok := st.GetUint16("uint16")
	require.True(t, ok)
	require.Equal(t, uint16(300), u)

	st.Set("wrong", "not uint16")
	u, ok = st.GetUint16("wrong")
	require.False(t, ok)
	require.Equal(t, uint16(0), u)

	u, ok = st.GetUint16("missing")
	require.False(t, ok)
	require.Equal(t, uint16(0), u)
}

func TestState_GetUint32(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("uint32", uint32(400000))
	u, ok := st.GetUint32("uint32")
	require.True(t, ok)
	require.Equal(t, uint32(400000), u)

	st.Set("wrong", "not uint32")
	u, ok = st.GetUint32("wrong")
	require.False(t, ok)
	require.Equal(t, uint32(0), u)

	u, ok = st.GetUint32("missing")
	require.False(t, ok)
	require.Equal(t, uint32(0), u)
}

func TestState_GetUint64(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("uint64", uint64(5000000))
	u, ok := st.GetUint64("uint64")
	require.True(t, ok)
	require.Equal(t, uint64(5000000), u)

	st.Set("wrong", "not uint64")
	u, ok = st.GetUint64("wrong")
	require.False(t, ok)
	require.Equal(t, uint64(0), u)

	u, ok = st.GetUint64("missing")
	require.False(t, ok)
	require.Equal(t, uint64(0), u)
}

func TestState_GetUintptr(t *testing.T) {
	t.Parallel()
	st := newState()

	var ptr uintptr = 12345
	st.Set("uintptr", ptr)
	u, ok := st.GetUintptr("uintptr")
	require.True(t, ok)
	require.Equal(t, ptr, u)

	st.Set("wrong", "not uintptr")
	u, ok = st.GetUintptr("wrong")
	require.False(t, ok)
	require.Equal(t, uintptr(0), u)

	u, ok = st.GetUintptr("missing")
	require.False(t, ok)
	require.Equal(t, uintptr(0), u)
}

func TestState_GetFloat32(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("float32", float32(3.14))
	f, ok := st.GetFloat32("float32")
	require.True(t, ok)
	require.InDelta(t, float32(3.14), f, 0.0001)

	st.Set("wrong", "not float32")
	f, ok = st.GetFloat32("wrong")
	require.False(t, ok)
	require.InDelta(t, float32(0), f, 0.0001)

	f, ok = st.GetFloat32("missing")
	require.False(t, ok)
	require.InDelta(t, float32(0), f, 0.0001)
}

func TestState_GetComplex64(t *testing.T) {
	t.Parallel()
	st := newState()

	var c complex64 = complex(2, 3)
	st.Set("complex64", c)
	cRes, ok := st.GetComplex64("complex64")
	require.True(t, ok)
	require.Equal(t, c, cRes)

	st.Set("wrong", "not complex64")
	cRes, ok = st.GetComplex64("wrong")
	require.False(t, ok)
	require.Equal(t, complex64(0), cRes)

	cRes, ok = st.GetComplex64("missing")
	require.False(t, ok)
	require.Equal(t, complex64(0), cRes)
}

func TestState_GetComplex128(t *testing.T) {
	t.Parallel()
	st := newState()

	c := complex(4, 5)
	st.Set("complex128", c)
	cRes, ok := st.GetComplex128("complex128")
	require.True(t, ok)
	require.Equal(t, c, cRes)

	st.Set("wrong", "not complex128")
	cRes, ok = st.GetComplex128("wrong")
	require.False(t, ok)
	require.Equal(t, complex128(0), cRes)

	cRes, ok = st.GetComplex128("missing")
	require.False(t, ok)
	require.Equal(t, complex128(0), cRes)
}

func TestState_MustGet(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("exists", "value")
	val := st.MustGet("exists")
	require.Equal(t, "value", val)

	// must-get on missing key should panic
	require.Panics(t, func() {
		_ = st.MustGet("missing")
	})
}

func TestState_Has(t *testing.T) {
	t.Parallel()

	st := newState()

	st.Set("key", "value")
	require.True(t, st.Has("key"))
}

func TestState_Delete(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("key", "value")
	st.Delete("key")
	_, ok := st.Get("key")
	require.False(t, ok)
}

func TestState_Reset(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("a", 1)
	st.Set("b", 2)
	st.Reset()
	require.Equal(t, 0, st.Len())
	require.Empty(t, st.Keys())
}

func TestState_Keys(t *testing.T) {
	t.Parallel()
	st := newState()

	keys := []string{"one", "two", "three"}
	for _, k := range keys {
		st.Set(k, k)
	}

	returnedKeys := st.Keys()
	require.ElementsMatch(t, keys, returnedKeys)
}

func TestState_Len(t *testing.T) {
	t.Parallel()
	st := newState()

	require.Equal(t, 0, st.Len())

	st.Set("a", "a")
	require.Equal(t, 1, st.Len())

	st.Set("b", "b")
	require.Equal(t, 2, st.Len())

	st.Delete("a")
	require.Equal(t, 1, st.Len())
}

type testCase[T any] struct { //nolint:govet // It does not really matter for test
	name     string
	key      string
	value    any
	expected T
	ok       bool
}

func runGenericTest[T any](t *testing.T, getter func(*State, string) (T, bool), tests []testCase[T]) {
	t.Helper()

	st := newState()
	for _, tc := range tests {
		st.Set(tc.key, tc.value)
		got, ok := getter(st, tc.key)
		require.Equal(t, tc.ok, ok, tc.name)
		require.Equal(t, tc.expected, got, tc.name)
	}
}

func TestState_GetGeneric(t *testing.T) {
	t.Parallel()

	runGenericTest[int](t, GetState[int], []testCase[int]{
		{"int correct conversion", "num", 42, 42, true},
		{"int wrong conversion from string", "str", "abc", 0, false},
	})

	runGenericTest[string](t, GetState[string], []testCase[string]{
		{"string correct conversion", "strVal", "hello", "hello", true},
		{"string wrong conversion from int", "intVal", 100, "", false},
	})

	runGenericTest[bool](t, GetState[bool], []testCase[bool]{
		{"bool correct conversion", "flag", true, true, true},
		{"bool wrong conversion from int", "intFlag", 1, false, false},
	})

	runGenericTest[float64](t, GetState[float64], []testCase[float64]{
		{"float64 correct conversion", "pi", 3.14, 3.14, true},
		{"float64 wrong conversion from int", "intVal", 10, 0.0, false},
	})
}

func Test_MustGetStateGeneric(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("flag", true)
	flag := MustGetState[bool](st, "flag")
	require.True(t, flag)

	// mismatched type should panic
	require.Panics(t, func() {
		_ = MustGetState[string](st, "flag")
	})

	// missing key should also panic
	require.Panics(t, func() {
		_ = MustGetState[string](st, "missing")
	})
}

func Test_GetStateWithDefault(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("flag", true)
	flag := GetStateWithDefault(st, "flag", false)
	require.True(t, flag)

	// mismatched type should return the default value
	str := GetStateWithDefault(st, "flag", "default")
	require.Equal(t, "default", str)

	// missing key should return the default value
	flag = GetStateWithDefault(st, "missing", false)
	require.False(t, flag)
}

func BenchmarkState_Set(b *testing.B) {
	b.ReportAllocs()

	st := newState()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}
}

func BenchmarkState_Get(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.Get(key)
	}
}

func BenchmarkState_GetString(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, strconv.Itoa(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetString(key)
	}
}

func BenchmarkState_GetInt(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetInt(key)
	}
}

func BenchmarkState_GetBool(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i%2 == 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetBool(key)
	}
}

func BenchmarkState_GetFloat64(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, float64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetFloat64(key)
	}
}

func BenchmarkState_MustGet(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.MustGet(key)
	}
}

func BenchmarkState_GetStateGeneric(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		GetState[int](st, key)
	}
}

func BenchmarkState_MustGetStateGeneric(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		MustGetState[int](st, key)
	}
}

func BenchmarkState_GetStateWithDefault(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		GetStateWithDefault[int](st, key, 0)
	}
}

func BenchmarkState_Has(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	// pre-populate the state
	for i := 0; i < n; i++ {
		st.Set("key"+strconv.Itoa(i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.Has("key" + strconv.Itoa(i%n))
	}
}

func BenchmarkState_Delete(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := newState()
		st.Set("a", 1)
		st.Delete("a")
	}
}

func BenchmarkState_Reset(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := newState()
		// add a fixed number of keys before clearing
		for j := 0; j < 100; j++ {
			st.Set("key"+strconv.Itoa(j), j)
		}
		st.Reset()
	}
}

func BenchmarkState_Keys(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	for i := 0; i < n; i++ {
		st.Set("key"+strconv.Itoa(i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = st.Keys()
	}
}

func BenchmarkState_Len(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	n := 1000
	for i := 0; i < n; i++ {
		st.Set("key"+strconv.Itoa(i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = st.Len()
	}
}

func BenchmarkState_GetUint(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with uint values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, uint(i)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetUint(key)
	}
}

func BenchmarkState_GetInt8(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with int8 values (using modulo to stay in range).
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, int8(i%128)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetInt8(key)
	}
}

func BenchmarkState_GetInt16(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with int16 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, int16(i)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetInt16(key)
	}
}

func BenchmarkState_GetInt32(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with int32 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, int32(i)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetInt32(key)
	}
}

func BenchmarkState_GetInt64(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with int64 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, int64(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetInt64(key)
	}
}

func BenchmarkState_GetUint8(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with uint8 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, uint8(i%256)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetUint8(key)
	}
}

func BenchmarkState_GetUint16(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with uint16 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, uint16(i)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetUint16(key)
	}
}

func BenchmarkState_GetUint32(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with uint32 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, uint32(i)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetUint32(key)
	}
}

func BenchmarkState_GetUint64(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with uint64 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, uint64(i)) //nolint:gosec // This is a test
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetUint64(key)
	}
}

func BenchmarkState_GetUintptr(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with uintptr values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, uintptr(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetUintptr(key)
	}
}

func BenchmarkState_GetFloat32(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with float32 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		st.Set(key, float32(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetFloat32(key)
	}
}

func BenchmarkState_GetComplex64(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with complex64 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		// Create a complex64 value with both real and imaginary parts.
		st.Set(key, complex(float32(i), float32(i)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetComplex64(key)
	}
}

func BenchmarkState_GetComplex128(b *testing.B) {
	b.ReportAllocs()
	st := newState()
	n := 1000
	// Pre-populate the state with complex128 values.
	for i := 0; i < n; i++ {
		key := "key" + strconv.Itoa(i)
		// Create a complex128 value with both real and imaginary parts.
		st.Set(key, complex(float64(i), float64(i)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key" + strconv.Itoa(i%n)
		st.GetComplex128(key)
	}
}
