package deepcopy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_ValueClonesTypedContainers(t *testing.T) {
	t.Parallel()

	require.Equal(t, 42, Value(42))

	var nilMap map[string]int
	require.Nil(t, Value(nilMap))
	var nilSlice []int
	require.Nil(t, Value(nilSlice))

	nested := map[string]any{"inner": map[string]any{"k": "v"}}
	src := map[string]map[string]any{"outer": nested}
	cloned, ok := Value(src).(map[string]map[string]any)
	require.True(t, ok)
	nested["inner"].(map[string]any)["k"] = "mutated"                     //nolint:errcheck,forcetypeassert // built just above
	require.Equal(t, "v", cloned["outer"]["inner"].(map[string]any)["k"]) //nolint:errcheck,forcetypeassert // built just above

	rows := [][]int{{1, 2}}
	clonedRows, ok := Value(rows).([][]int)
	require.True(t, ok)
	rows[0][0] = 99
	require.Equal(t, 1, clonedRows[0][0])
}

func Test_MapKeepsNilAndEmpty(t *testing.T) {
	t.Parallel()

	require.Nil(t, Map(nil))
	empty := Map(map[string]any{})
	require.NotNil(t, empty)
	require.Empty(t, empty)

	copied := Map(map[string]any{"properties": map[string]any{}})
	require.Equal(t, map[string]any{}, copied["properties"])
}

func Test_SecurityKeepsNilEmptyAndScopes(t *testing.T) {
	t.Parallel()

	require.Nil(t, Security(nil))
	empty := Security([]map[string][]string{})
	require.NotNil(t, empty)
	require.Empty(t, empty)

	src := []map[string][]string{{"oauth": {"read"}, "key": nil}}
	copied := Security(src)
	src[0]["oauth"][0] = "changed"
	require.Equal(t, []string{"read"}, copied[0]["oauth"])
	require.NotNil(t, copied[0]["key"])
	require.Empty(t, copied[0]["key"])
}

func Test_ValueStopsAtMaxDepth(t *testing.T) {
	t.Parallel()

	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	require.NotPanics(t, func() { _ = Value(cyclic) })

	cyclicList := []any{nil}
	cyclicList[0] = cyclicList
	require.NotPanics(t, func() { _ = Value(cyclicList) })
}

func Test_ValueClonesStructsPointersAndArrays(t *testing.T) {
	t.Parallel()

	type inner struct{ Tags []string }
	type model struct {
		Meta   map[string]any
		Ptr    *inner
		Arr    [2]map[string]int
		hidden map[string]int
	}
	src := model{
		Meta:   map[string]any{"k": []any{"v"}},
		Ptr:    &inner{Tags: []string{"a"}},
		Arr:    [2]map[string]int{{"x": 1}, nil},
		hidden: map[string]int{"h": 1},
	}

	got, ok := Value(src).(model)
	require.True(t, ok)

	src.Meta["k"].([]any)[0] = "changed" //nolint:errcheck,forcetypeassert // built just above
	src.Ptr.Tags[0] = "changed"
	src.Arr[0]["x"] = 99
	require.Equal(t, []any{"v"}, got.Meta["k"])
	require.Equal(t, []string{"a"}, got.Ptr.Tags)
	require.Equal(t, 1, got.Arr[0]["x"])
	require.Nil(t, got.Arr[1])
	require.Equal(t, map[string]int{"h": 1}, got.hidden, "unexported fields are copied as they are")

	var nilPtr *inner
	require.Nil(t, Value(nilPtr))

	cyclic := &struct{ Self any }{}
	cyclic.Self = cyclic
	require.NotPanics(t, func() { _ = Value(cyclic) })
}

// runsQuickly fails the test if f does not finish: a copy of a cyclic value must not blow up.
func runsQuickly(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("copy of a cyclic value did not finish")
	}
}

func Test_ValueCopiesFanOutCyclesOnce(t *testing.T) {
	t.Parallel()

	type node struct{ A, B *node }
	n := &node{}
	n.A, n.B = n, n
	runsQuickly(t, func() {
		got, ok := Value(n).(*node)
		require.True(t, ok)
		require.NotSame(t, n, got)
		require.Same(t, got, got.A, "the copy points back to the copy, not the source")
		require.Same(t, got, got.B)
	})

	list := []any{nil, nil}
	list[0], list[1] = list, list
	runsQuickly(t, func() {
		got, ok := Value(list).([]any)
		require.True(t, ok)
		require.Len(t, got, 2)
	})

	m := map[string]any{}
	m["a"], m["b"] = m, m
	runsQuickly(t, func() {
		got := Map(m)
		require.Len(t, got, 2)
		_, isMap := got["a"].(map[string]any)
		require.True(t, isMap)
	})
}
