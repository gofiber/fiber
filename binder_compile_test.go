package fiber

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type postStruct struct {
	Title string         `form:"title"`
	Body  string         `form:"body"`
	Test  postProperties `form:"test"`
}

type postStruct2 struct {
	Title string           `form:"title"`
	Body  string           `form:"body"`
	Test  postProperties   `form:"test"`
	Tests []postProperties `form:"tests"`
}

type postProperties struct {
	Desc  string `form:"desc"`
	Likes int    `form:"likes"`
}

type testStruct struct {
	Name string     `form:"name"`
	Age  int        `form:"age"`
	Post postStruct `form:"post"`
}

type testStruct2 struct {
	Name  string        `form:"name"`
	Age   int           `form:"age"`
	Post  postStruct2   `form:"post"`
	Posts []postStruct2 `form:"posts"`
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
		t.Parallel()

		field, ok := el.FieldByName("Integer")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "integer", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "integer", fieldTextDecoder.reqKey)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("float", func(t *testing.T) {
		t.Parallel()

		field, ok := el.FieldByName("Float")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "float", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "float", fieldTextDecoder.reqKey)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("bool", func(t *testing.T) {
		t.Parallel()

		field, ok := el.FieldByName("Boolean")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "boolean", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "boolean", fieldTextDecoder.reqKey)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		field, ok := el.FieldByName("String")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "string", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		fieldTextDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "string", fieldTextDecoder.reqKey)
		require.Equal(t, "form", fieldTextDecoder.tag)
		require.NotNil(t, fieldTextDecoder.dec)
		require.NotNil(t, fieldTextDecoder.get)
	})

	t.Run("embedStruct", func(t *testing.T) {
		t.Parallel()

		field, ok := el.FieldByName("EmbedStruct")
		require.True(t, ok)

		decoder, err := compileTextBasedDecoder(field, 0, "form", "embedStruct", bindCompileOption{
			bodyDecoder: true,
		})
		require.NoError(t, err)

		textDecoder, ok := decoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Equal(t, "embedStruct", textDecoder.reqKey)
		require.Equal(t, "form", textDecoder.tag)
		require.Nil(t, textDecoder.dec)
		require.NotNil(t, textDecoder.get)
		require.Len(t, textDecoder.subFieldDecoders, 3)

		checkSubFieldDecoder(t, textDecoder, textDecoder.reqKey)
	})
}

func Test_compileSliceFieldTextBasedDecoder(t *testing.T) {
	t.Parallel()

	opt := bindCompileOption{
		bodyDecoder: true,
	}

	testVar := reflect.TypeOf(&testStruct2{})
	el := testVar.Elem()

	t.Run("posts", func(t *testing.T) {
		field, ok := el.FieldByName("Posts")
		require.True(t, ok)

		decoder, err := compileSliceFieldTextBasedDecoder(field, 0, "form", "posts", opt)
		require.NoError(t, err)

		fieldSliceDecoder, ok := decoder.(*fieldSliceDecoder)
		require.True(t, ok)

		require.Equal(t, "Posts", fieldSliceDecoder.fieldName)
		require.Equal(t, "posts", string(fieldSliceDecoder.reqKey))
		require.NotNil(t, fieldSliceDecoder.visitAll)
		require.Len(t, fieldSliceDecoder.subFieldDecoders, 4)
		checkSubFieldDecoderSlice(t, fieldSliceDecoder, string(fieldSliceDecoder.reqKey))
	})
}

func Test_compileFieldDecoder(t *testing.T) {
	t.Parallel()

	opt := bindCompileOption{
		bodyDecoder: true,
	}

	testVar := reflect.TypeOf(&testStruct2{})
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

	require.Len(t, decoders, 4)

	decoder0, ok := decoders[0].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "name", decoder0.reqKey)
	require.Equal(t, "form", decoder0.tag)
	require.NotNil(t, decoder0.dec)
	require.NotNil(t, decoder0.get)
	require.Len(t, decoder0.subFieldDecoders, 0)

	decoder1, ok := decoders[1].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "age", decoder1.reqKey)
	require.Equal(t, "form", decoder1.tag)
	require.NotNil(t, decoder1.dec)
	require.NotNil(t, decoder1.get)
	require.Len(t, decoder1.subFieldDecoders, 0)

	decoder2, ok := decoders[2].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "post", decoder2.reqKey)
	require.Equal(t, "form", decoder2.tag)
	require.Nil(t, decoder2.dec)
	require.NotNil(t, decoder2.get)
	require.Len(t, decoder2.subFieldDecoders, 4)

	decoder20 := decoder2.subFieldDecoders[0].(*fieldTextDecoder)
	require.Equal(t, "post.title", decoder20.reqKey)
	require.Equal(t, "form", decoder20.tag)
	require.NotNil(t, decoder20.dec)
	require.NotNil(t, decoder20.get)
	require.Len(t, decoder20.subFieldDecoders, 0)

	decoder21 := decoder2.subFieldDecoders[1].(*fieldTextDecoder)
	require.Equal(t, "post.body", decoder21.reqKey)
	require.Equal(t, "form", decoder21.tag)
	require.NotNil(t, decoder21.dec)
	require.NotNil(t, decoder21.get)
	require.Len(t, decoder21.subFieldDecoders, 0)

	decoder22 := decoder2.subFieldDecoders[2].(*fieldTextDecoder)
	require.Equal(t, "post.test", decoder22.reqKey)
	require.Equal(t, "form", decoder22.tag)
	require.Nil(t, decoder22.dec)
	require.NotNil(t, decoder22.get)
	require.Len(t, decoder22.subFieldDecoders, 2)

	decoder220 := decoder22.subFieldDecoders[0].(*fieldTextDecoder)
	require.Equal(t, "post.test.desc", decoder220.reqKey)
	require.Equal(t, "form", decoder220.tag)
	require.NotNil(t, decoder220.dec)
	require.NotNil(t, decoder220.get)
	require.Len(t, decoder220.subFieldDecoders, 0)

	decoder221 := decoder22.subFieldDecoders[1].(*fieldTextDecoder)
	require.Equal(t, "post.test.likes", decoder221.reqKey)
	require.Equal(t, "form", decoder221.tag)
	require.NotNil(t, decoder221.dec)
	require.NotNil(t, decoder221.get)
	require.Len(t, decoder221.subFieldDecoders, 0)

	decoder3, ok := decoders[3].(*fieldSliceDecoder)
	require.True(t, ok)

	require.Equal(t, "Posts", decoder3.fieldName)
	require.Equal(t, "posts", string(decoder3.reqKey))
	require.NotNil(t, decoder3.visitAll)
	require.Len(t, decoder3.subFieldDecoders, 4)

	checkSubFieldDecoderSlice(t, decoder3, string(decoder3.reqKey))

	decoder30, ok := decoder3.subFieldDecoders[0].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "posts.NUM.title", decoder30.reqKey)
	require.Equal(t, "form", decoder30.tag)
	require.NotNil(t, decoder30.dec)
	require.NotNil(t, decoder30.get)
	require.Len(t, decoder30.subFieldDecoders, 0)

	decoder31, ok := decoder3.subFieldDecoders[1].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "posts.NUM.body", decoder31.reqKey)
	require.Equal(t, "form", decoder31.tag)
	require.NotNil(t, decoder31.dec)
	require.NotNil(t, decoder31.get)
	require.Len(t, decoder31.subFieldDecoders, 0)

	decoder32, ok := decoder3.subFieldDecoders[2].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "posts.NUM.test", decoder32.reqKey)
	require.Equal(t, "form", decoder32.tag)
	require.Nil(t, decoder32.dec)
	require.NotNil(t, decoder32.get)
	require.Len(t, decoder32.subFieldDecoders, 2)

	decoder320 := decoder32.subFieldDecoders[0].(*fieldTextDecoder)
	require.Equal(t, "posts.NUM.test.desc", decoder320.reqKey)
	require.Equal(t, "form", decoder320.tag)
	require.NotNil(t, decoder320.dec)
	require.NotNil(t, decoder320.get)
	require.Len(t, decoder320.subFieldDecoders, 0)

	decoder321 := decoder32.subFieldDecoders[1].(*fieldTextDecoder)
	require.Equal(t, "posts.NUM.test.likes", decoder321.reqKey)
	require.Equal(t, "form", decoder321.tag)
	require.NotNil(t, decoder321.dec)
	require.NotNil(t, decoder321.get)
	require.Len(t, decoder321.subFieldDecoders, 0)

	decoder33, ok := decoder3.subFieldDecoders[3].(*fieldSliceDecoder)
	require.True(t, ok)

	require.Equal(t, "Tests", decoder33.fieldName)
	require.Equal(t, "posts.NUM.tests", string(decoder33.reqKey))
	require.NotNil(t, decoder33.visitAll)
	require.Len(t, decoder33.subFieldDecoders, 2)

	decoder330, ok := decoder33.subFieldDecoders[0].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "posts.NUM.tests.NUM.desc", decoder330.reqKey)
	require.Equal(t, "form", decoder330.tag)
	require.NotNil(t, decoder330.dec)
	require.NotNil(t, decoder330.get)
	require.Len(t, decoder330.subFieldDecoders, 0)

	decoder331, ok := decoder33.subFieldDecoders[1].(*fieldTextDecoder)
	require.True(t, ok)

	require.Equal(t, "posts.NUM.tests.NUM.likes", decoder331.reqKey)
	require.Equal(t, "form", decoder331.tag)
	require.NotNil(t, decoder331.dec)
	require.NotNil(t, decoder331.get)
}

func checkSubFieldDecoder(t *testing.T, textDecoder *fieldTextDecoder, reqKey string) {
	t.Helper()

	for _, subFieldDecoder := range textDecoder.subFieldDecoders {
		fmt.Print(subFieldDecoder.Kind())
		subFieldTextDecoder, ok := subFieldDecoder.(*fieldTextDecoder)
		require.True(t, ok)

		require.Contains(t, subFieldTextDecoder.reqKey, reqKey+".")

		if subFieldTextDecoder.dec == nil {
			checkSubFieldDecoder(t, subFieldTextDecoder, subFieldTextDecoder.reqKey)
		} else {
			require.NotNil(t, subFieldTextDecoder.dec)
		}
		require.NotNil(t, subFieldTextDecoder.get)
	}
}

func checkSubFieldDecoderSlice(t *testing.T, sliceDecoder *fieldSliceDecoder, reqKey string) {
	t.Helper()

	for _, subFieldDecoder := range sliceDecoder.subFieldDecoders {
		if subFieldDecoder.Kind() == "text" {
			subFieldTextDecoder, ok := subFieldDecoder.(*fieldTextDecoder)
			require.True(t, ok)

			require.Contains(t, subFieldTextDecoder.reqKey, reqKey+".")

			if subFieldTextDecoder.dec == nil {
				checkSubFieldDecoder(t, subFieldTextDecoder, subFieldTextDecoder.reqKey)
			} else {
				require.NotNil(t, subFieldTextDecoder.dec)
			}
			require.NotNil(t, subFieldTextDecoder.get)
		} else {
			subFieldSliceDecoder, ok := subFieldDecoder.(*fieldSliceDecoder)
			require.True(t, ok)

			require.Contains(t, string(subFieldSliceDecoder.reqKey), reqKey+".")

			if subFieldSliceDecoder.elementDecoder == nil {
				checkSubFieldDecoderSlice(t, subFieldSliceDecoder, string(subFieldSliceDecoder.reqKey))
			}
			require.NotNil(t, subFieldSliceDecoder.visitAll)
		}
	}
}
