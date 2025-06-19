package binder

import (
	"mime/multipart"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_GetAndPutToThePool(t *testing.T) {
	t.Parallel()

	// Panics in case we get from another pool
	require.Panics(t, func() {
		_ = GetFromThePool[*HeaderBinding](&CookieBinderPool)
	})

	// We get from the pool
	binder := GetFromThePool[*HeaderBinding](&HeaderBinderPool)
	PutToThePool(&HeaderBinderPool, binder)

	_ = GetFromThePool[*RespHeaderBinding](&RespHeaderBinderPool)
	_ = GetFromThePool[*QueryBinding](&QueryBinderPool)
	_ = GetFromThePool[*FormBinding](&FormBinderPool)
	_ = GetFromThePool[*URIBinding](&URIBinderPool)
	_ = GetFromThePool[*XMLBinding](&XMLBinderPool)
	_ = GetFromThePool[*JSONBinding](&JSONBinderPool)
	_ = GetFromThePool[*CBORBinding](&CBORBinderPool)
}

func Test_Binders_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("query binder invalid key", func(t *testing.T) {
		b := &QueryBinding{}
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("invalid[%3Dval&name=john")
		defer fasthttp.ReleaseRequest(req)
		err := b.Bind(req, &struct{}{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unmatched brackets")
	})

	t.Run("form binder invalid key", func(t *testing.T) {
		b := &FormBinding{}
		req := fasthttp.AcquireRequest()
		req.SetBodyString("invalid[=val")
		req.Header.SetContentType("application/x-www-form-urlencoded")
		defer fasthttp.ReleaseRequest(req)
		err := b.Bind(req, &struct{}{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unmatched brackets")
	})

	t.Run("form binder bad multipart", func(t *testing.T) {
		b := &FormBinding{}
		req := fasthttp.AcquireRequest()
		req.Header.SetContentType(MIMEMultipartForm)
		defer fasthttp.ReleaseRequest(req)
		err := b.Bind(req, &struct{}{})
		require.Error(t, err)
	})
}

func Test_GetFieldCache_Panic(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() { getFieldCache("unknown") })
}

func Test_parseToMap_defaultCase(t *testing.T) {
	t.Parallel()
	m := map[string]int{}
	err := parseToMap(m, map[string][]string{"a": {"1"}})
	require.NoError(t, err)
	require.Empty(t, m)

	m2 := map[string]string{}
	err = parseToMap(m2, map[string][]string{"empty": {}})
	require.NoError(t, err)
	require.Equal(t, "", m2["empty"])
}

func Test_parse_function_maps(t *testing.T) {
	t.Parallel()

	m := map[string][]string{}
	err := parse("query", &m, map[string][]string{"a": {"b"}})
	require.NoError(t, err)
	require.Equal(t, []string{"b"}, m["a"])

	m2 := map[string]string{}
	err = parse("query", &m2, map[string][]string{"a": {"b"}})
	require.NoError(t, err)
	require.Equal(t, "b", m2["a"])
}

func Test_SetParserDecoder_UnknownKeys(t *testing.T) {
	t.Parallel()
	SetParserDecoder(ParserConfig{IgnoreUnknownKeys: false})
	type user struct {
		Name string `query:"name"`
	}
	data := map[string][]string{"name": {"john"}, "foo": {"bar"}}
	err := parseToStruct("query", &user{}, data)
	require.Error(t, err)
	SetParserDecoder(ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true})
}

func Test_SetParserDecoder_CustomConverter(t *testing.T) {
	t.Parallel()

	type myInt int
	conv := func(s string) reflect.Value {
		v, _ := strconv.Atoi(s)
		mi := myInt(v)
		return reflect.ValueOf(mi)
	}

	SetParserDecoder(ParserConfig{ParserType: []ParserType{{CustomType: myInt(0), Converter: conv}}})
	defer SetParserDecoder(ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true})

	type data struct {
		V myInt `query:"v"`
	}
	d := new(data)
	err := parse("query", d, map[string][]string{"v": {"5"}})
	require.NoError(t, err)
	require.Equal(t, myInt(5), d.V)
}

func Test_formatBindData_typeMismatch(t *testing.T) {
	t.Parallel()
	out := struct{}{}
	files := map[string][]*multipart.FileHeader{}
	err := formatBindData("query", out, files, "file", 123, false, false)
	require.Error(t, err)
	require.Equal(t, "unsupported value type: int", err.Error())
}
