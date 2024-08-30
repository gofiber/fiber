package fiber

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type postStruct struct {
	Title string `form:"title"`
	Body  string `form:"body"`
}

type testStruct struct {
	Name string     `form:"name"`
	Age  int        `form:"age"`
	Post postStruct `form:"post"`
}

// type testStruct2 struct {
// 	Name  string       `form:"name"`
// 	Age   int          `form:"age"`
// 	Post  postStruct   `form:"post"`
// 	Posts []postStruct `form:"posts"`
// }

func checkSubFieldDecoder(t *testing.T, textDecoder *fieldTextDecoder, reqField string) {
	t.Helper()

	for _, subFieldDecoder := range textDecoder.subFieldDecoders {
		subFieldTextDecoder, ok := subFieldDecoder.(*fieldTextDecoder)
		require.True(t, ok)

		fmt.Println(subFieldTextDecoder.reqField)
		require.Contains(t, subFieldTextDecoder.reqField, reqField+".")

		if subFieldTextDecoder.dec == nil {
			checkSubFieldDecoder(t, subFieldTextDecoder, subFieldTextDecoder.reqField)
		} else {
			require.NotNil(t, subFieldTextDecoder.dec)
		}
		require.NotNil(t, subFieldTextDecoder.get)
	}
}

func Test_compileTextBasedDecoder(t *testing.T) {
	t.Parallel()

	_ = bindCompileOption{
		bodyDecoder: true,
	}

	type simpleStruct struct {
		Integer     int        `form:"integer"`
		Float       float64    `form:"float"`
		Boolean     bool       `form:"boolean"`
		String      string     `form:"string"`
		EmbedStruct testStruct `form:"embedStruct"`
	}

	testVar := reflect.TypeOf(&simpleStruct{})
	el := testVar.Elem()

	t.Run("int", func(t *testing.T) {
		field, ok := el.FieldByName("Integer")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "integer", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "integer", fieldTextDecoder.reqField)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("float", func(t *testing.T) {
		field, ok := el.FieldByName("Float")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "float", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "float", fieldTextDecoder.reqField)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("bool", func(t *testing.T) {
		field, ok := el.FieldByName("Boolean")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "boolean", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "boolean", fieldTextDecoder.reqField)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("string", func(t *testing.T) {
		field, ok := el.FieldByName("String")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "string", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "string", fieldTextDecoder.reqField)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("embedStruct", func(t *testing.T) {
		field, ok := el.FieldByName("EmbedStruct")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "embedStruct", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		textDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "embedStruct", textDecoder.reqField)
		require.Equal(t, "form", textDecoder.tag)
		require.Nil(t, textDecoder.dec)
		require.NotNil(t, textDecoder.get)
		require.Len(t, textDecoder.subFieldDecoders, 3)

		checkSubFieldDecoder(t, textDecoder, textDecoder.reqField)
	})
}

func Test_compileFieldDecoder(t *testing.T) {
	t.Parallel()

	opt := bindCompileOption{
		bodyDecoder: true,
	}

	testVar := reflect.TypeOf(&testStruct{})
	el := testVar.Elem()

	var decoders []decoder

	for i := 0; i < el.NumField(); i++ {
		if !el.Field(i).IsExported() {
			// ignore unexported field
			continue
		}

		dec, err := compileFieldDecoder(el.Field(i), i, opt, nil)
		require.NoError(t, err)

		if dec != nil {
			decoders = append(decoders, dec)
		}
	}

	require.Len(t, decoders, 3)
}
