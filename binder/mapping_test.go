package binder

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_EqualFieldType(t *testing.T) {
	var out int
	require.False(t, equalFieldType(&out, reflect.Int, "key"))

	var dummy struct{ f string }
	require.False(t, equalFieldType(&dummy, reflect.String, "key"))

	var dummy2 struct{ f string }
	require.False(t, equalFieldType(&dummy2, reflect.String, "f"))

	var user struct {
		Name    string
		Address string `query:"address"`
		Age     int    `query:"AGE"`
	}
	require.True(t, equalFieldType(&user, reflect.String, "name"))
	require.True(t, equalFieldType(&user, reflect.String, "Name"))
	require.True(t, equalFieldType(&user, reflect.String, "address"))
	require.True(t, equalFieldType(&user, reflect.String, "Address"))
	require.True(t, equalFieldType(&user, reflect.Int, "AGE"))
	require.True(t, equalFieldType(&user, reflect.Int, "age"))
}
