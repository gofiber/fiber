package session

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// Test_Session_UnchangedRequestKeepsConcurrentChange_NaN checks that a session holding a NaN is told from an
// edited one like any other. NaN differs from itself, so the snapshot decoded again from the same bytes read as
// changed once a value Get returned could be edited in place, and a request that only read the session
// overwrote the change another request stored, a role taken away for one.
func Test_Session_UnchangedRequestKeepsConcurrentChange_NaN(t *testing.T) {
	t.Parallel()
	nan := math.NaN()
	samples := func(s *Session) []float64 {
		v, ok := s.Get("samples").([]float64)
		require.True(t, ok)
		return v
	}
	for name, tc := range map[string]struct {
		touch   func(*Session)
		want    string // the role the storage ends up with
		samples string // the samples it ends up with, as fmt prints them: NaN is not equal to itself
	}{
		"scalar NaN beside a slice read": {touch: func(s *Session) { _ = s.Get("tags") }, want: "user", samples: "[1 NaN]"},
		"NaN inside the slice read":      {touch: func(s *Session) { _ = samples(s) }, want: "user", samples: "[1 NaN]"},
		"NaN inside the map read":        {touch: func(s *Session) { _ = s.Get("scores") }, want: "user", samples: "[1 NaN]"},
		"a value changed beside the NaN": {touch: func(s *Session) { samples(s)[0] = 2 }, want: "admin", samples: "[2 NaN]"},
		"the NaN replaced in place":      {touch: func(s *Session) { samples(s)[1] = 3 }, want: "admin", samples: "[1 3]"},
		"a value replaced by a NaN":      {touch: func(s *Session) { samples(s)[0] = nan }, want: "admin", samples: "[NaN NaN]"},
		"a map entry changed in place": {touch: func(s *Session) {
			scores, ok := s.Get("scores").(map[string]float64)
			require.True(t, ok)
			scores["b"] = 3
		}, want: "admin", samples: "[1 NaN]"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			store := NewStore(Config{Storage: memory.New()})
			store.RegisterType([]string{})
			store.RegisterType([]float64{})
			store.RegisterType(map[string]float64{})
			id := seedSession(t, app, store, func(s *Session) {
				s.Set("role", "admin")
				s.Set("ratio", nan)
				s.Set("tags", []string{"x"})
				s.Set("samples", []float64{1, nan})
				s.Set("scores", map[string]float64{"a": nan, "b": 2})
			})

			inFlight := loadSession(t, app, store, id)
			tc.touch(inFlight)
			other := loadSession(t, app, store, id)
			other.Set("role", "user")
			require.NoError(t, other.Save())
			require.NoError(t, inFlight.Save())

			stored := sessionTestStored(t, store, id)
			require.Equal(t, tc.want, stored.Get("role"))
			require.Equal(t, tc.samples, fmt.Sprint(stored.Get("samples")))
		})
	}
}

// sameTestNode is session data with a field that is not exported, and a way to refer to itself.
type sameTestNode struct {
	next  *sameTestNode
	Name  string
	Tags  []string
	Score float64
	value float64
}

// sameTestDataOf wraps v the way a session holds it.
func sameTestDataOf(v any) map[any]any { return map[any]any{"key": v} }

// Test_SameData_AgreesWithDeepEqual checks that without a NaN sameData tells what reflect.DeepEqual does
// for every pair of values of a corpus of the kinds a session holds: it only adds that a NaN is the same as a NaN.
func Test_SameData_AgreesWithDeepEqual(t *testing.T) {
	t.Parallel()
	zone := time.FixedZone("", 3600)
	var nilNode *sameTestNode
	values := []any{
		nil, 0, 1, int64(1), uint8(1), "", "a", "b", true, false, 1.5, float32(1.5), complex(1, 2), complex(1, 3),
		[]byte(nil),
		[]byte{},
		[]byte("a"), []int(nil),
		[]int{},
		[]int{1},
		[]int{1, 2},
		[]int{1, 3},
		[2]int{1, 2},
		[2]int{1, 3},
		[2]string{"a", "b"},
		map[string]int(nil),
		map[string]int{},
		map[string]int{"a": 1},
		map[string]int{"a": 2},
		map[string]int{"b": 1},
		map[string]int{"a": 1, "b": 2},
		map[string]any{"a": []int{1}},
		map[string]any{"a": []int{2}},
		map[string]any{"a": 1},
		[]any{1, "a"},
		[]any{1, "b"},
		[]any{nil},
		[]any{},
		new(1), new(1), new(2), (*int)(nil),
		sameTestNode{Name: "a"},
		sameTestNode{Name: "a"},
		sameTestNode{Name: "b"},
		sameTestNode{Name: "a", value: 1},
		sameTestNode{Tags: []string{"x"}},
		sameTestNode{Tags: []string{"y"}},
		sameTestNode{Tags: []string{}},
		sameTestNode{Tags: nil},
		&sameTestNode{Name: "a"}, &sameTestNode{Name: "a"}, &sameTestNode{Name: "a", next: &sameTestNode{}}, nilNode,
		time.Unix(100, 0).UTC(), time.Unix(100, 0).In(zone), time.Unix(101, 0).UTC(),
		struct{ A int }{1},
		struct{ B int }{1},
	}
	for i, a := range values {
		for j, b := range values {
			want := reflect.DeepEqual(sameTestDataOf(a), sameTestDataOf(b))
			require.Equal(t, want, sameValue(reflect.ValueOf(sameTestDataOf(a)), reflect.ValueOf(sameTestDataOf(b))), "%d (%#v) and %d (%#v)", i, a, j, b)
			require.Equal(t, want, sameData(sameTestDataOf(a), sameTestDataOf(b)), "%d (%#v) and %d (%#v)", i, a, j, b)
		}
	}
}

// Test_SameData_NaN pins where a NaN is the same as a NaN, and that nothing else is made the same by it.
func Test_SameData_NaN(t *testing.T) {
	t.Parallel()
	nan := math.NaN()
	type nested = map[string][]map[string]float64
	for name, tc := range map[string]struct {
		a, b any
		want bool
	}{
		"NaN and NaN":                    {a: nan, b: nan, want: true},
		"NaN and a number":               {a: nan, b: 1.0},
		"a number and NaN":               {a: 1.0, b: nan},
		"infinity and NaN":               {a: math.Inf(1), b: nan},
		"float32 NaN and NaN":            {a: float32(nan), b: float32(nan), want: true},
		"float32 NaN and float64 NaN":    {a: float32(nan), b: nan},
		"complex with a NaN part":        {a: complex(nan, 1), b: complex(nan, 1), want: true},
		"complex with another real part": {a: complex(nan, 1), b: complex(2, 1)},
		"complex with another imaginary": {a: complex(1, nan), b: complex(1, 2)},
		"slice":                          {a: []float64{1, nan}, b: []float64{1, nan}, want: true},
		"slice with another number":      {a: []float64{2, nan}, b: []float64{1, nan}},
		"slice with a shorter one":       {a: []float64{1, nan}, b: []float64{1}},
		"slice with nil":                 {a: []float64{nan}, b: []float64(nil)},
		"array":                          {a: [2]float64{nan, 1}, b: [2]float64{nan, 1}, want: true},
		"array with another number":      {a: [2]float64{nan, 1}, b: [2]float64{nan, 2}},
		"map value":                      {a: map[string]float64{"a": nan}, b: map[string]float64{"a": nan}, want: true},
		"map value of another key":       {a: map[string]float64{"a": nan}, b: map[string]float64{"b": nan}},
		"map with another entry":         {a: map[string]float64{"a": nan, "b": 1}, b: map[string]float64{"a": nan, "b": 2}},
		"map with an entry more":         {a: map[string]float64{"a": nan}, b: map[string]float64{"a": nan, "b": 1}},
		"map keyed by a NaN":             {a: map[float64]int{nan: 1}, b: map[float64]int{nan: 1}}, // as with DeepEqual: never found
		"struct":                         {a: sameTestNode{Score: nan}, b: sameTestNode{Score: nan}, want: true},
		"struct field that is not exported": {
			a: sameTestNode{value: nan}, b: sameTestNode{value: nan}, want: true,
		},
		"struct with another name":  {a: sameTestNode{Name: "a", Score: nan}, b: sameTestNode{Name: "b", Score: nan}},
		"pointer":                   {a: new(nan), b: new(nan), want: true},
		"pointer and nil":           {a: new(nan), b: (*float64)(nil)},
		"interfaces":                {a: []any{nan, "x"}, b: []any{nan, "x"}, want: true},
		"interfaces and a nil":      {a: []any{nan}, b: []any{nil}},
		"nested":                    {a: nested{"a": {{"b": nan}}}, b: nested{"a": {{"b": nan}}}, want: true},
		"nested with another value": {a: nested{"a": {{"b": nan, "c": 1}}}, b: nested{"a": {{"b": nan, "c": 2}}}},
		"nested with a slice less":  {a: nested{"a": {{"b": nan}}}, b: nested{"a": {}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, sameData(sameTestDataOf(tc.a), sameTestDataOf(tc.b)))
			require.Equal(t, tc.want, sameData(sameTestDataOf(tc.b), sameTestDataOf(tc.a)), "the other way round")
		})
	}
}

// Test_SameData_SelfReference checks that the walk ends where the data was made to refer to itself, as the
// data of a request can be after an edit in place, by following the snapshot, which cannot.
func Test_SameData_SelfReference(t *testing.T) {
	t.Parallel()
	nan := math.NaN()
	loop := &sameTestNode{Score: nan}
	loop.next = loop
	snapshot := sameTestDataOf(&sameTestNode{Score: nan, next: &sameTestNode{Score: nan}})
	require.False(t, sameData(snapshot, sameTestDataOf(loop)))
	require.False(t, sameData(sameTestDataOf(loop), snapshot))
}
