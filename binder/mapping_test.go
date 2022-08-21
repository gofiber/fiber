package binder

import (
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3/utils"
)

func Test_EqualFieldType(t *testing.T) {
	var out int
	utils.AssertEqual(t, false, equalFieldType(&out, reflect.Int, "key"))

	var dummy struct{ f string }
	utils.AssertEqual(t, false, equalFieldType(&dummy, reflect.String, "key"))

	var dummy2 struct{ f string }
	utils.AssertEqual(t, false, equalFieldType(&dummy2, reflect.String, "f"))

	var user struct {
		Name    string
		Address string `query:"address"`
		Age     int    `query:"AGE"`
	}
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "name"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "Name"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "address"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "Address"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.Int, "AGE"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.Int, "age"))
}
