package binder

import (
	"mime/multipart"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetAndPutToThePool(t *testing.T) {
	t.Parallel()

	// Panics in case we get from another pool
	require.Panics(t, func() { _ = GetFromThePool[*HeaderBinding](&CookieBinderPool) })

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
	_ = GetFromThePool[*MsgPackBinding](&MsgPackBinderPool)
}

// Test_Unimplemented_ReturnsError pins that the codecs a binder falls back to
// when none is configured fail with their own error instead of panicking.
func Test_Unimplemented_ReturnsError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
		want      error
		name      string
		input     []byte
	}{
		{
			name: "cbor", want: ErrUnimplementedCBOR, input: []byte{0xa0},
			marshal: UnimplementedCborMarshal, unmarshal: UnimplementedCborUnmarshal,
		},
		{
			name: "msgpack", want: ErrMsgPackNotConfigured, input: []byte{0x80},
			marshal: UnimplementedMsgpackMarshal, unmarshal: UnimplementedMsgpackUnmarshal,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.marshal(struct{ Name string }{Name: "test"})
			require.ErrorIs(t, err, tc.want)

			var out any
			require.ErrorIs(t, tc.unmarshal(tc.input, &out), tc.want)
		})
	}
}

// Test_Binders_ErrorPaths pins that a key with an unmatched bracket is an error
// from the binders that rewrite bracket notation, and so is a multipart request
// without a body.
func Test_Binders_ErrorPaths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		input string
		src   source
	}{
		{name: "query binder invalid key", src: pairSources[0], input: "invalid[%3Dval&name=john"},
		{name: "form binder invalid key", src: pairSources[1], input: "invalid[=val"},
		{name: "multipart binder invalid key", src: pairSources[2], input: "invalid[=val"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorContains(t, tc.src.bind(t, tc.input, false, &struct{}{}), "unmatched brackets")
		})
	}

	t.Run("multipart binder invalid file key", func(t *testing.T) {
		t.Parallel()
		req := newMultipartRequestWith(t, func(mw *multipart.Writer) {
			writeFormFile(t, mw, "invalid[", "file.txt", "content")
		})
		require.ErrorContains(t, (&FormBinding{}).Bind(req, &struct{}{}), "unmatched brackets")
	})

	t.Run("form binder bad multipart", func(t *testing.T) {
		t.Parallel()
		req := acquireRequest(t)
		req.Header.SetContentType(MIMEMultipartForm)
		require.Error(t, (&FormBinding{}).Bind(req, &struct{}{}))
	})
}

// Test_Binders_NameAndReset requires each binder to report its name, and Reset
// to clear its splitting flag.
func Test_Binders_NameAndReset(t *testing.T) {
	t.Parallel()
	query, form := &QueryBinding{EnableSplitting: true}, &FormBinding{EnableSplitting: true}
	header, respHeader := &HeaderBinding{EnableSplitting: true}, &RespHeaderBinding{EnableSplitting: true}
	cookie := &CookieBinding{EnableSplitting: true}
	for _, tc := range []struct {
		b interface {
			Name() string
			Reset()
		}
		splitting *bool
		subtest   string
		name      string
	}{
		{subtest: "query", b: query, splitting: &query.EnableSplitting, name: "query"},
		{subtest: "form", b: form, splitting: &form.EnableSplitting, name: "form"},
		{subtest: "header", b: header, splitting: &header.EnableSplitting, name: "header"},
		{subtest: "resp header", b: respHeader, splitting: &respHeader.EnableSplitting, name: "respHeader"},
		{subtest: "cookie", b: cookie, splitting: &cookie.EnableSplitting, name: "cookie"},
	} {
		t.Run(tc.subtest, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.name, tc.b.Name())
			tc.b.Reset()
			require.False(t, *tc.splitting)
		})
	}
}

func Test_parse_function_maps(t *testing.T) {
	t.Parallel()
	m := map[string][]string{}
	require.NoError(t, parse("query", &m, map[string][]string{"a": {"b"}}))
	require.Equal(t, []string{"b"}, m["a"])

	m2 := map[string]string{}
	require.NoError(t, parse("query", &m2, map[string][]string{"a": {"b"}}))
	require.Equal(t, "b", m2["a"])

	var zeroStringMap map[string]string
	require.NoError(t, parse("query", &zeroStringMap, map[string][]string{"foo": {"bar", "baz"}}))
	require.Equal(t, "baz", zeroStringMap["foo"])

	var zeroSliceMap map[string][]string
	require.NoError(t, parse("query", &zeroSliceMap, map[string][]string{"foo": {"bar", "baz"}}))
	require.Equal(t, []string{"bar", "baz"}, zeroSliceMap["foo"])
}

func Test_SetParserDecoder_UnknownKeys(t *testing.T) {
	SetParserDecoder(ParserConfig{IgnoreUnknownKeys: false})
	defer SetParserDecoder(ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true})
	type user struct {
		Name string `query:"name"`
	}
	data := map[string][]string{"name": {"john"}, "foo": {"bar"}}
	require.Error(t, parseToStruct("query", &user{}, data))

	// The binders decode a struct from its pairs with DecodeValues, which must
	// honor the setting as well.
	req := newQueryRequest(t, "name=john&foo=bar")
	require.ErrorContains(t, (&QueryBinding{}).Bind(req, &user{}), "foo")
	require.ErrorContains(t, parseValuesToStruct("query", &user{}, []string{"name", "foo"}, []string{"john", "bar"}), "foo")

	SetParserDecoder(ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true})
	var u user
	require.NoError(t, (&QueryBinding{}).Bind(req, &u))
	require.Equal(t, "john", u.Name)
}

func Test_SetParserDecoder_CustomConverter(t *testing.T) {
	type myInt int
	conv := func(s string) reflect.Value {
		v, _ := strconv.Atoi(s) //nolint:errcheck // not needed
		mi := myInt(v)
		return reflect.ValueOf(mi)
	}

	SetParserDecoder(ParserConfig{ParserType: []ParserType{{CustomType: myInt(0), Converter: conv}}})
	defer SetParserDecoder(ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true})

	type data struct {
		V myInt `query:"v"`
	}
	d := new(data)
	require.NoError(t, parse("query", d, map[string][]string{"v": {"5"}}))
	require.Equal(t, myInt(5), d.V)
}

// Test_SetParserDecoder_AliasTagIgnored: ParserConfig.SetAliasTag must not
// override the per-source binding tag (one global tag breaks multi-source binding).
func Test_SetParserDecoder_AliasTagIgnored(t *testing.T) {
	SetParserDecoder(ParserConfig{SetAliasTag: "ignored", IgnoreUnknownKeys: true, ZeroEmpty: true})
	defer SetParserDecoder(ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true})

	type data struct {
		Greeting string `query:"v"`
	}
	d := new(data)
	require.NoError(t, parse("query", d, map[string][]string{"v": {"hello"}}))
	require.Equal(t, "hello", d.Greeting)
}
