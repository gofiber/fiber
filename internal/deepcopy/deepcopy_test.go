package deepcopy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Reflected(t *testing.T) {
	t.Parallel()

	require.Equal(t, 42, reflected(42, 0))

	var nilMap map[string]int
	require.Nil(t, reflected(nilMap, 0))
	var nilSlice []int
	require.Nil(t, reflected(nilSlice, 0))

	nested := map[string]any{"inner": map[string]any{"k": "v"}}
	src := map[string]map[string]any{"outer": nested}
	cloned, ok := reflected(src, 0).(map[string]map[string]any)
	require.True(t, ok)
	nested["inner"].(map[string]any)["k"] = "mutated"                     //nolint:errcheck,forcetypeassert // built just above
	require.Equal(t, "v", cloned["outer"]["inner"].(map[string]any)["k"]) //nolint:errcheck,forcetypeassert // built just above

	rows := [][]int{{1, 2}}
	clonedRows, ok := reflected(rows, 0).([][]int)
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
