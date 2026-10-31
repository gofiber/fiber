package fiber

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestState_SetAndGet(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		state func() *State
		name  string
	}{
		{name: "state", state: newState},
		{name: "app state", state: func() *State { return New().State() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			st := tc.state()
			st.Set("foo", "bar")
			val, ok := st.Get("foo")
			require.True(t, ok)
			require.Equal(t, "bar", val)

			_, ok = st.Get("unknown")
			require.False(t, ok)
		})
	}
}

// stateTestGetter runs, as a parallel subtest called name, the three scenarios every typed State
// getter must satisfy, each as its own subtest: a stored value of the getter's type is returned
// together with true, while a value of another type (wrongType) and a missing key both return the
// zero value together with false.
//
// State hands values back untouched, so they are compared with Equal even for floats.
func stateTestGetter[T any](t *testing.T, name string, get func(*State, string) (T, bool), value T, wrongType any) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		var zero T
		for _, tc := range []struct {
			want T
			name string
			key  string
			ok   bool
		}{
			{name: "stored value", key: "value", want: value, ok: true},
			{name: "wrong type", key: "wrong", want: zero},
			{name: "missing key", key: "missing", want: zero},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				st := newState()
				st.Set("value", value)
				st.Set("wrong", wrongType)

				got, ok := get(st, tc.key)
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.want, got)
			})
		}
	})
}

// TestState_TypedGetters covers every typed getter of State, and GetState, the
// generic getter, for some of the types.
func TestState_TypedGetters(t *testing.T) {
	t.Parallel()
	stateTestGetter(t, "GetString", (*State).GetString, "hello", 123)
	stateTestGetter(t, "GetInt", (*State).GetInt, 456, "abc")
	stateTestGetter(t, "GetBool", (*State).GetBool, true, 1)
	stateTestGetter(t, "GetFloat64", (*State).GetFloat64, 3.14, 10)
	stateTestGetter(t, "GetUint", (*State).GetUint, 100, "not uint")
	stateTestGetter(t, "GetInt8", (*State).GetInt8, 10, "not int8")
	stateTestGetter(t, "GetInt16", (*State).GetInt16, 200, "not int16")
	stateTestGetter(t, "GetInt32", (*State).GetInt32, 3000, "not int32")
	stateTestGetter(t, "GetInt64", (*State).GetInt64, 4000, "not int64")
	stateTestGetter(t, "GetUint8", (*State).GetUint8, 20, "not uint8")
	stateTestGetter(t, "GetUint16", (*State).GetUint16, 300, "not uint16")
	stateTestGetter(t, "GetUint32", (*State).GetUint32, 400000, "not uint32")
	stateTestGetter(t, "GetUint64", (*State).GetUint64, 5000000, "not uint64")
	stateTestGetter(t, "GetUintptr", (*State).GetUintptr, 12345, "not uintptr")
	stateTestGetter(t, "GetFloat32", (*State).GetFloat32, 3.14, "not float32")
	stateTestGetter(t, "GetComplex64", (*State).GetComplex64, complex(2, 3), "not complex64")
	stateTestGetter(t, "GetComplex128", (*State).GetComplex128, complex(4, 5), "not complex128")
	stateTestGetter(t, "GetState[int]", GetState[int], 42, "abc")
	stateTestGetter(t, "GetState[string]", GetState[string], "hello", 100)
	stateTestGetter(t, "GetState[bool]", GetState[bool], true, 1)
	stateTestGetter(t, "GetState[float64]", GetState[float64], 3.14, 10)
}

func TestState_MustGet(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("exists", "value")
	val := st.MustGet("exists")
	require.Equal(t, "value", val)

	// must-get on missing key should panic
	require.Panics(t, func() { _ = st.MustGet("missing") })
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
	for _, tc := range []struct {
		name string
		keys []any // stored in this order: a string with Set, any other key straight into the map
		want []string
	}{
		{name: "string keys", keys: []any{"one", "two", "three"}, want: []string{"one", "two", "three"}},
		{name: "skips non-string keys", keys: []any{"one", "two", 42, struct{}{}}, want: []string{"one", "two"}},
		{
			name: "skips non-string keys in mixed order",
			keys: []any{"before", 42, "after"}, want: []string{"before", "after"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			st := newState()
			for _, key := range tc.keys {
				if s, ok := key.(string); ok {
					st.Set(s, s)
				} else {
					st.dependencies.Store(key, "value")
				}
			}
			require.ElementsMatch(t, tc.want, st.Keys())
		})
	}
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

func Test_MustGetStateGeneric(t *testing.T) {
	t.Parallel()
	st := newState()

	st.Set("flag", true)
	flag := MustGetState[bool](st, "flag")
	require.True(t, flag)

	// mismatched type should panic
	require.Panics(t, func() { _ = MustGetState[string](st, "flag") })

	// missing key should also panic
	require.Panics(t, func() { _ = MustGetState[string](st, "missing") })
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

func TestState_Service(t *testing.T) {
	t.Parallel()
	srv1 := &mockService{name: "test1"}
	// service 2 is using a very subtle name to check it is not picked up
	srv2 := &mockService{name: "test1 "}

	t.Run("set/get/ok", func(t *testing.T) {
		t.Parallel()
		st := newState()
		st.setService(srv1)

		got, ok := st.Get(st.serviceKey(srv1.String()))
		require.True(t, ok)
		require.Equal(t, srv1, got)
	})

	t.Run("set/get/ko", func(t *testing.T) {
		t.Parallel()
		st := newState()
		st.setService(srv1)

		koSrv := &mockService{name: "ko"}

		got, ok := st.Get(st.serviceKey(koSrv.String()))
		require.False(t, ok)
		require.Nil(t, got)
	})

	t.Run("len", func(t *testing.T) {
		t.Parallel()
		t.Run("empty", func(t *testing.T) {
			t.Parallel()
			st := newState()
			require.Equal(t, 0, st.Len())
			require.Empty(t, st.serviceKeys())
		})

		t.Run("with-services", func(t *testing.T) {
			t.Parallel()
			st := newState()
			st.setService(srv1)
			st.setService(srv2)

			require.Equal(t, 2, st.Len())
			require.Equal(t, 2, st.ServicesLen())
		})

		t.Run("with-services/with-keys", func(t *testing.T) {
			t.Parallel()
			st := newState()
			st.setService(srv1)
			st.setService(srv2)
			st.Set("key1", "value1")
			st.Set("key2", "value2")

			servicesLen := st.ServicesLen()
			require.Equal(t, 4, st.Len())
			require.Equal(t, 2, servicesLen)
		})
	})

	t.Run("keys", func(t *testing.T) {
		t.Parallel()
		t.Run("empty", func(t *testing.T) {
			t.Parallel()
			st := newState()
			// adding more keys to check they are not included
			st.Set("key1", "value1")
			st.Set("key2", "value2")

			require.Empty(t, st.serviceKeys())
		})

		// Each row stores others next to both services; only the services are listed.
		for _, tc := range []struct {
			others map[any]any
			name   string
		}{
			{name: "with-services"},
			{name: "with-services/with-keys", others: map[any]any{"key1": "value1", "key2": "value2"}},
			{name: "with-non-string-key", others: map[any]any{42: "value", struct{}{}: "value"}},
			{name: "with-non-service-keys", others: map[any]any{"other": "value", 42: "value"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				st := newState()
				st.setService(srv1)
				st.setService(srv2)
				for key, value := range tc.others {
					st.dependencies.Store(key, value)
				}

				keys := st.serviceKeys()
				require.Len(t, keys, 2)
				require.Contains(t, keys, st.serviceKey(srv1.String()))
				require.Contains(t, keys, st.serviceKey(srv2.String()))
				for key := range tc.others {
					require.NotContains(t, keys, key)
				}
			})
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			st := newState()

			st.setService(srv1)
			st.deleteService(srv1)

			_, ok := st.Get(st.serviceKey(srv1.String()))
			require.False(t, ok)
		})

		t.Run("missing", func(t *testing.T) {
			t.Parallel()
			st := newState()
			st.setService(srv1)

			st.deleteService(srv2)

			_, ok := st.Get(st.serviceKey(srv1.String()))
			require.True(t, ok)

			_, ok = st.Get(st.serviceKey(srv2.String()))
			require.False(t, ok)
		})
	})
}

func TestState_GetService(t *testing.T) {
	t.Parallel()
	srv1 := &mockService{name: "test1"}
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		st := newState()
		st.setService(srv1)

		got, ok := GetService[*mockService](st, srv1.String())
		require.True(t, ok)
		require.Equal(t, srv1, got)
	})

	t.Run("ko", func(t *testing.T) {
		t.Parallel()
		got, ok := GetService[*mockService](newState(), srv1.String())
		require.False(t, ok)
		require.Nil(t, got)
	})
}

func TestState_MustGetService(t *testing.T) {
	t.Parallel()
	srv1 := &mockService{name: "test1"}
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		st := newState()
		st.setService(srv1)

		require.Equal(t, srv1, MustGetService[*mockService](st, srv1.String()))
	})

	t.Run("panics", func(t *testing.T) {
		t.Parallel()
		st := newState()
		require.Panics(t, func() { _ = MustGetService[*mockService](st, srv1.String()) })
	})
}

// stateBenchKeys is how many keys stateBenchFill puts in a State.
const stateBenchKeys = 1000

// stateBenchFill reports allocations and returns a State holding stateBenchKeys keys, "key0" onwards, each key i set
// to value(i).
func stateBenchFill(b *testing.B, value func(i int) any) *State {
	b.Helper()

	b.ReportAllocs()

	st := newState()
	for i := range stateBenchKeys {
		st.Set("key"+strconv.Itoa(i), value(i))
	}
	return st
}

func BenchmarkState_Set(b *testing.B) {
	b.ReportAllocs()

	st := newState()

	i := 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i)
		st.Set(key, i)
	}
}

func BenchmarkState_Get(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.Get(key)
	}
}

func BenchmarkState_GetString(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return strconv.Itoa(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetString(key)
	}
}

func BenchmarkState_GetInt(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetInt(key)
	}
}

func BenchmarkState_GetBool(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i%2 == 0 })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetBool(key)
	}
}

func BenchmarkState_GetFloat64(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return float64(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetFloat64(key)
	}
}

func BenchmarkState_MustGet(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.MustGet(key)
	}
}

func BenchmarkState_GetStateGeneric(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		GetState[int](st, key)
	}
}

func BenchmarkState_MustGetStateGeneric(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		MustGetState[int](st, key)
	}
}

func BenchmarkState_GetStateWithDefault(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		GetStateWithDefault(st, key, 0)
	}
}

func BenchmarkState_Has(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		st.Has("key" + strconv.Itoa(i%n))
	}
}

func BenchmarkState_Delete(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		st := newState()
		st.Set("a", 1)
		st.Delete("a")
	}
}

func BenchmarkState_Reset(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		st := newState()
		for j := range 100 {
			st.Set("key"+strconv.Itoa(j), j)
		}
		st.Reset()
	}
}

func BenchmarkState_Keys(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	for b.Loop() {
		_ = st.Keys()
	}
}

func BenchmarkState_Len(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return i })
	for b.Loop() {
		_ = st.Len()
	}
}

func BenchmarkState_GetUint(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return uint(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetUint(key)
	}
}

func BenchmarkState_GetInt8(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return int8(i % 128) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetInt8(key)
	}
}

func BenchmarkState_GetInt16(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return int16(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetInt16(key)
	}
}

func BenchmarkState_GetInt32(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return int32(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetInt32(key)
	}
}

func BenchmarkState_GetInt64(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return int64(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetInt64(key)
	}
}

func BenchmarkState_GetUint8(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return uint8(i % 256) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetUint8(key)
	}
}

func BenchmarkState_GetUint16(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return uint16(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetUint16(key)
	}
}

func BenchmarkState_GetUint32(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return uint32(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetUint32(key)
	}
}

func BenchmarkState_GetUint64(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return uint64(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetUint64(key)
	}
}

func BenchmarkState_GetUintptr(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return uintptr(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetUintptr(key)
	}
}

func BenchmarkState_GetFloat32(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return float32(i) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetFloat32(key)
	}
}

func BenchmarkState_GetComplex64(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return complex(float32(i), float32(i)) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetComplex64(key)
	}
}

func BenchmarkState_GetComplex128(b *testing.B) {
	st := stateBenchFill(b, func(i int) any { return complex(float64(i), float64(i)) })
	n, i := stateBenchKeys, 0
	for b.Loop() {
		i++
		key := "key" + strconv.Itoa(i%n)
		st.GetComplex128(key)
	}
}

func BenchmarkState_GetService(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	srv := &mockService{name: "benchService"}
	st.setService(srv)

	for b.Loop() {
		_, _ = GetService[*mockService](st, srv.String())
	}
}

func BenchmarkState_MustGetService(b *testing.B) {
	b.ReportAllocs()

	st := newState()
	srv := &mockService{name: "benchService"}
	st.setService(srv)

	for b.Loop() {
		_ = MustGetService[*mockService](st, srv.String())
	}
}
