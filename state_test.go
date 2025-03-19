package fiber

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	flag := GetStateWithDefault[bool](st, "flag", false)
	require.True(t, flag)

	// mismatched type should return the default value
	str := GetStateWithDefault[string](st, "flag", "default")
	require.Equal(t, "default", str)

	// missing key should return the default value
	flag = GetStateWithDefault[bool](st, "missing", false)
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
