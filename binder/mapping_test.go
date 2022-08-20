package binder

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_EqualFieldType(t *testing.T) {
	var out int
	require.Equal(t, false, equalFieldType(&out, reflect.Int, "key"))

	var dummy struct{ f string }
	require.Equal(t, false, equalFieldType(&dummy, reflect.String, "key"))

	var dummy2 struct{ f string }
	require.Equal(t, false, equalFieldType(&dummy2, reflect.String, "f"))

	var user struct {
		Name    string
		Address string `query:"address"`
		Age     int    `query:"AGE"`
	}
	require.Equal(t, true, equalFieldType(&user, reflect.String, "name"))
	require.Equal(t, true, equalFieldType(&user, reflect.String, "Name"))
	require.Equal(t, true, equalFieldType(&user, reflect.String, "address"))
	require.Equal(t, true, equalFieldType(&user, reflect.String, "Address"))
	require.Equal(t, true, equalFieldType(&user, reflect.Int, "AGE"))
	require.Equal(t, true, equalFieldType(&user, reflect.Int, "age"))
}
