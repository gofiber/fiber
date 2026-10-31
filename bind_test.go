//nolint:wrapcheck,tagliatelle // We must not wrap errors in tests
package fiber

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/schema"
	"github.com/shamaton/msgpack/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

const helloWorld = "hello world"

// bindTestSource is a place a Bind method reads its input from. Scenarios that only differ in where the input is put
// and in which method binds it are written once and run for each source.
type bindTestSource struct {
	method func(b *Bind, out any) error // (*Bind).Header, (*Bind).Cookie, ...
	name   string                       // subtest name
	source string                       // BindSource constant, which is also the struct tag the source reads
}

var (
	bindTestQuery      = bindTestSource{name: "Query", source: BindSourceQuery, method: (*Bind).Query}
	bindTestHeader     = bindTestSource{name: "Header", source: BindSourceHeader, method: (*Bind).Header}
	bindTestRespHeader = bindTestSource{name: "RespHeader", source: BindSourceRespHeader, method: (*Bind).RespHeader}
	bindTestCookie     = bindTestSource{name: "Cookie", source: BindSourceCookie, method: (*Bind).Cookie}
)

// bind binds out from the source of c.
func (s bindTestSource) bind(c Ctx, out any) error {
	return s.method(c.Bind(), out)
}

// add puts a value under key into the request (or the response) the source reads. The query is not filled this way:
// tests set the whole query string.
func (s bindTestSource) add(c Ctx, key, value string) {
	switch s.source {
	case BindSourceHeader:
		c.Request().Header.Add(key, value)
	case BindSourceRespHeader:
		c.Response().Header.Add(key, value)
	case BindSourceCookie:
		c.Request().Header.SetCookie(key, value)
	default:
		panic("bindTestSource.add does not fill " + s.name)
	}
}

// del removes what add put under key.
func (s bindTestSource) del(c Ctx, key string) {
	switch s.source {
	case BindSourceHeader:
		c.Request().Header.Del(key)
	case BindSourceRespHeader:
		c.Response().Header.Del(key)
	case BindSourceCookie:
		c.Request().Header.DelCookie(key)
	default:
		panic("bindTestSource.del does not empty " + s.name)
	}
}

// set adds each key/value pair of kv, given as alternating keys and values.
func (s bindTestSource) set(c Ctx, kv ...string) {
	for key, value := range apptest.Pairs(kv) {
		s.add(c, key, value)
	}
}

// bindTestRequireBindError requires err to be a *BindError of the given source and returns it for further checks. It is
// returned by value so that callers who do not check it are not reported for ignoring an error.
func bindTestRequireBindError(t *testing.T, err error, source string) BindError {
	t.Helper()

	require.Error(t, err)
	var be *BindError
	require.ErrorAs(t, err, &be)
	require.Equal(t, source, be.Source)
	return *be
}

// bindTestRequireMultiError requires err to be the MultiError of a failed field check, with exactly the message msg.
func bindTestRequireMultiError(t *testing.T, err error, msg string) {
	t.Helper()

	require.Error(t, err)
	require.Equal(t, msg, err.Error())
	require.ErrorAs(t, err, &MultiError{})
}

// bindTestMultipart encodes fields as a multipart/form-data body. It returns the body with the Content-Type that names
// its boundary, which is the given one or, when it is empty, a random one.
func bindTestMultipart(tb testing.TB, boundary string, fields ...[2]string) *RequestConfig {
	tb.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if boundary != "" {
		require.NoError(tb, w.SetBoundary(boundary))
	}
	for _, field := range fields {
		require.NoError(tb, w.WriteField(field[0], field[1]))
	}
	require.NoError(tb, w.Close())
	return &RequestConfig{ContentType: w.FormDataContentType(), Body: body.Bytes()}
}

// bindTestPostName posts body with the given Content-Type to a handler that binds the form field "name" with bind and
// requires it to be "john". check, if it is not nil, then inspects the request inside the handler.
func bindTestPostName(t *testing.T, bind func(c Ctx, out any) error, contentType string, body []byte, check func(c Ctx)) {
	t.Helper()

	app := New()
	app.Post("/", func(c Ctx) error {
		var out struct {
			Name string `form:"name"`
		}
		require.NoError(t, bind(c, &out))
		require.Equal(t, "john", out.Name)
		if check != nil {
			check(c)
		}
		return nil
	})

	req := httptest.NewRequest(MethodPost, "/", bytes.NewReader(body))
	req.Header.Set(HeaderContentType, contentType)
	apptest.Expect(t, app, req, StatusOK)
}

// bindBenchQuery returns a context of app whose request has an empty body and Content-Type and the given query.
func bindBenchQuery(b *testing.B, app *App, query string) CustomCtx {
	b.Helper()

	c := apptest.Ctx(b, app)
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString(query)
	return c
}

// bindBenchSource returns a context of a new app whose request has an empty body and Content-Type and whose src
// carries the id, Name and Hobby the source benchmarks bind.
func bindBenchSource(b *testing.B, src bindTestSource) CustomCtx {
	b.Helper()

	c := apptest.Ctx(b, New())
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	src.set(c, "id", "1", "Name", "John Doe", "Hobby", "golang,fiber")
	return c
}

// bindBenchBody returns a context of app whose request carries body as contentType.
func bindBenchBody(b *testing.B, app *App, contentType string, body []byte) CustomCtx {
	b.Helper()

	c := apptest.Ctx(b, app)
	(&RequestConfig{ContentType: contentType, Body: body}).ApplyTo(c)
	return c
}

// bindBenchDemo is what the body benchmarks bind: a name that every body format spells the same way.
type bindBenchDemo struct {
	Name string `json:"name" xml:"name" form:"name" msgpack:"name"`
}

// bindBenchFields is the struct the query, header and cookie benchmarks bind.
type bindBenchFields struct {
	Name  string
	Hobby []string
	ID    int
}

// bindTestAllRequest returns a request that fills every field the Bind().All tests and benchmarks bind.
func bindTestAllRequest() *RequestConfig {
	return &RequestConfig{
		ContentType: MIMEApplicationJSON,
		Body:        []byte(`{"name":"john", "email": "john@doe.com", "session_id": "abc1234", "id": 1}`),
		Headers:     map[string]string{"X-User-Role": "admin"},
		Cookies:     map[string]string{"session_id": "abc123"},
		Query:       "id=1&name=john",
	}
}

type typedNilBindError struct {
	message string
}

func (e *typedNilBindError) Error() string {
	return e.message
}

func Test_returnErr(t *testing.T) {
	t.Parallel()
	var nilFiberErr *Error
	var nilCustomErr *typedNilBindError

	// A nil pointer in an error interface is no error and must not make the call panic.
	for _, tc := range []struct {
		call func(b *Bind) error
		name string
	}{
		{name: "nil", call: func(b *Bind) error { return b.WithAutoHandling().returnErr(nil) }},
		{name: "typed nil fiber error", call: func(b *Bind) error { return b.WithAutoHandling().returnErr(nilFiberErr) }},
		{name: "typed nil custom error", call: func(b *Bind) error { return b.WithAutoHandling().returnErr(nilCustomErr) }},
		{name: "returnBindErr with typed nil fiber error", call: func(b *Bind) error {
			return b.WithoutAutoHandling().returnBindErr(nilFiberErr, BindSourceBody)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())

			var err error
			require.NotPanics(t, func() { err = tc.call(c.Bind()) })
			require.NoError(t, err)
		})
	}
}

func Test_AcquireReleaseBind(t *testing.T) {
	b := AcquireBind()
	b.shouldSkipErrHandling = false
	b.shouldSkipValidation = true
	b.ctx = &DefaultCtx{}
	ReleaseBind(b)

	b2 := AcquireBind()
	require.Nil(t, b2.ctx)
	require.True(t, b2.shouldSkipErrHandling)
	require.False(t, b2.shouldSkipValidation)
	ReleaseBind(b2)
}

func Test_BindError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	be := &BindError{Source: BindSourceQuery, Err: inner}
	require.ErrorIs(t, be, inner)
	require.Equal(t, inner, errors.Unwrap(be))

	var extracted *BindError
	require.ErrorAs(t, be, &extracted)
	require.Equal(t, BindSourceQuery, extracted.Source)
}

func Test_BindError_ErrorFormat(t *testing.T) {
	t.Parallel()
	t.Run("with field", func(t *testing.T) {
		t.Parallel()
		be := &BindError{Source: BindSourceQuery, Field: "id", Err: errors.New("conversion failed")}
		require.Contains(t, be.Error(), `"id"`)
		require.Contains(t, be.Error(), "query")
		require.Contains(t, be.Error(), "conversion failed")
	})

	t.Run("without field", func(t *testing.T) {
		t.Parallel()
		be := &BindError{Source: BindSourceBody, Field: "", Err: errors.New("parse failed")}
		require.Equal(t, "bind from body: parse failed", be.Error())
	})
}

func Test_BindError_FieldExtraction(t *testing.T) {
	t.Parallel()
	t.Run("QueryConversionError", func(t *testing.T) {
		t.Parallel()
		type Q struct {
			ID int `query:"id"`
		}
		c := apptest.Ctx(t, New())
		c.Request().URI().SetQueryString("id=notanint")
		err := c.Bind().Query(new(Q))
		be := bindTestRequireBindError(t, err, BindSourceQuery)
		require.Equal(t, "id", be.Field)
		require.ErrorAs(t, err, &MultiError{})
	})

	// A custom binder fails with err; the field the BindError names depends on the type of that error.
	type D struct{ Name string }
	for _, tc := range []struct {
		err   error
		name  string
		mime  string
		field string
	}{
		{name: "ConversionError", err: schema.ConversionError{Key: "count"}, mime: "application/x-conversion-error-test", field: "count"},
		{name: "UnknownKeyError", err: schema.UnknownKeyError{Key: "extra_field"}, mime: "application/x-unknown-key-test", field: "extra_field"},
		{name: "EmptyFieldError", err: schema.EmptyFieldError{Key: "required_field"}, mime: "application/x-empty-field-test", field: "required_field"},
		{name: "NoRecognizedErrorType", err: errors.New("generic parse failure"), mime: "application/x-generic-error"}, // no field
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.RegisterCustomBinder(&customBinderReturningError{err: tc.err, mimeType: tc.mime})
			c := apptest.Ctx(t, app)
			c.Request().SetBody([]byte("{}"))
			c.Request().Header.SetContentType(tc.mime)
			be := bindTestRequireBindError(t, c.Bind().Body(new(D)), BindSourceBody)
			require.Equal(t, tc.field, be.Field)
		})
	}

	t.Run("JSONUnmarshalTypeError", func(t *testing.T) {
		t.Parallel()
		type J struct {
			Count int `json:"count"`
		}
		c := apptest.Ctx(t, New())
		c.Request().SetBody([]byte(`{"count":"notanint"}`))
		c.Request().Header.SetContentType(MIMEApplicationJSON)
		err := c.Bind().Body(new(J))
		be := bindTestRequireBindError(t, err, BindSourceBody)
		require.Equal(t, "count", be.Field)
		var unmarshalErr *json.UnmarshalTypeError
		require.ErrorAs(t, err, &unmarshalErr)
	})

	t.Run("MultiError", func(t *testing.T) {
		t.Parallel()
		type Q struct {
			A string `query:"a,required"`
			B string `query:"b,required"`
		}
		c := apptest.Ctx(t, New())
		c.Request().URI().SetQueryString("")
		err := c.Bind().Query(new(Q))
		be := bindTestRequireBindError(t, err, BindSourceQuery)
		require.Contains(t, []string{"a", "b"}, be.Field)
		require.ErrorAs(t, err, &MultiError{})
	})
}

func Test_BindError_Sources(t *testing.T) {
	t.Parallel()
	t.Run("URI", func(t *testing.T) {
		t.Parallel()
		type U struct {
			ID int `uri:"id"`
		}
		app := New()
		app.Get("/user/:id", func(ctx Ctx) error {
			err := ctx.Bind().URI(new(U))
			bindTestRequireBindError(t, err, BindSourceURI)
			require.ErrorAs(t, err, &MultiError{})
			return nil
		})
		apptest.Do(t, app, "/user/notanint")
	})

	// A required value that is missing fails with a MultiError that names the source.
	for _, src := range []bindTestSource{bindTestQuery, bindTestHeader, bindTestCookie} {
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			type Required struct {
				ID int `query:"id,required" header:"x-id,required" cookie:"id,required"`
			}
			c := apptest.Ctx(t, New())
			err := src.bind(c, new(Required))
			bindTestRequireBindError(t, err, src.source)
			require.ErrorAs(t, err, &MultiError{})
		})
	}

	t.Run("Body", func(t *testing.T) {
		t.Parallel()
		type J struct {
			X int `json:"x"`
		}
		c := apptest.Ctx(t, New())
		c.Request().SetBody([]byte(`{"x":"bad"}`))
		c.Request().Header.SetContentType(MIMEApplicationJSON)
		err := c.Bind().Body(new(J))
		bindTestRequireBindError(t, err, BindSourceBody)
		var unmarshalErr *json.UnmarshalTypeError
		require.ErrorAs(t, err, &unmarshalErr)
	})
}

func Test_BindError_All(t *testing.T) {
	t.Parallel()
	type Req struct {
		Name string `json:"name"`
		ID   int    `uri:"id" json:"id"`
	}

	t.Run("URIFailsFirst", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/users/:id", func(ctx Ctx) error {
			err := ctx.Bind().All(new(Req))
			bindTestRequireBindError(t, err, BindSourceURI)
			require.ErrorAs(t, err, &MultiError{})
			return nil
		})
		req := httptest.NewRequest(http.MethodGet, "/users/notanint", bytes.NewReader([]byte(`{"name":"ok"}`)))
		req.Header.Set("Content-Type", MIMEApplicationJSON)
		apptest.Do(t, app, req)
	})
}

// bindTestHobbyQueries are queries with the number of hobbies a query bind with splitting on commas finds in them.
var bindTestHobbyQueries = []struct {
	query   string
	hobbies int
}{
	{query: "id=1&name=tom&hobby=basketball&hobby=football", hobbies: 2},
	{query: "id=1&name=tom&hobby=basketball,football", hobbies: 2},
	{query: "id=1&name=tom&hobby=soccer&hobby=basketball,football", hobbies: 3},
}

func Test_Bind_Query(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{EnableSplittingOnParsers: true}))

	type Query struct {
		Name  string
		Hobby []string
		ID    int
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	for _, tc := range bindTestHobbyQueries {
		c.Request().URI().SetQueryString(tc.query)
		q := new(Query)
		require.NoError(t, c.Bind().Query(q))
		require.Len(t, q.Hobby, tc.hobbies, tc.query)
	}

	empty := new(Query)
	c.Request().URI().SetQueryString("")
	require.NoError(t, c.Bind().Query(empty))
	require.Empty(t, empty.Hobby)

	type Query2 struct {
		Name            string
		Hobby           string
		Default         string `query:"default,default:hello"`
		FavouriteDrinks []string
		Empty           []string
		Alloc           []string
		Defaults        []string `query:"defaults,default:hello|world"`
		No              []int64
		ID              int
		Bool            bool
	}

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1")
	q2 := new(Query2)
	q2.Bool = true
	q2.Name = helloWorld
	require.NoError(t, c.Bind().Query(q2))
	require.Equal(t, "basketball,football", q2.Hobby)
	require.True(t, q2.Bool)
	require.Equal(t, "tom", q2.Name) // check value get overwritten
	require.Equal(t, []string{"milo", "coke", "pepsi"}, q2.FavouriteDrinks)
	var nilSlice []string
	require.Equal(t, nilSlice, q2.Empty)
	require.Equal(t, []string{""}, q2.Alloc)
	require.Equal(t, []int64{1}, q2.No)
	require.Equal(t, "hello", q2.Default)
	require.Equal(t, []string{"hello", "world"}, q2.Defaults)

	type RequiredQuery struct {
		Name string `query:"name,required"`
	}
	rq := new(RequiredQuery)
	c.Request().URI().SetQueryString("")
	bindTestRequireMultiError(t, c.Bind().Query(rq), `bind "name" from query: name is empty`)

	type ArrayQuery struct {
		Data []string
	}
	aq := new(ArrayQuery)
	c.Request().URI().SetQueryString("data[]=john&data[]=doe")
	require.NoError(t, c.Bind().Query(aq))
	require.Len(t, aq.Data, 2)
}

func Test_Bind_Query_Map(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{EnableSplittingOnParsers: true}))

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	for _, tc := range bindTestHobbyQueries {
		c.Request().URI().SetQueryString(tc.query)
		q := make(map[string][]string)
		require.NoError(t, c.Bind().Query(&q))
		require.Len(t, q["hobby"], tc.hobbies, tc.query)
	}

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=soccer")
	qq := make(map[string]string)
	require.NoError(t, c.Bind().Query(&qq))
	require.Equal(t, "1", qq["id"])

	empty := make(map[string][]string)
	c.Request().URI().SetQueryString("")
	require.NoError(t, c.Bind().Query(&empty))
	require.Empty(t, empty["hobby"])

	em := make(map[string][]int)
	c.Request().URI().SetQueryString("")
	require.ErrorIs(t, c.Bind().Query(&em), binder.ErrMapNotConvertible)
}

func Test_Bind_Query_MapField(t *testing.T) {
	t.Parallel()
	type Filter struct {
		Attributes map[string]string   `query:"attributes"`
		Tags       map[string][]string `query:"tags"`
		Limits     map[string]int      `query:"limits"`
		Unused     map[string]string   `query:"unused"`
		Name       string              `query:"name"`
	}

	t.Run("brackets and dots", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New())
		c.Request().URI().SetQueryString("name=shoes&attributes[color]=red&attributes.size=m" +
			"&tags[sale]=yes&tags[sale]=new&limits[max]=5&attributes[color]=blue")

		f := new(Filter)
		require.NoError(t, c.Bind().Query(f))
		require.Equal(t, "shoes", f.Name)
		// the last value wins for a single-value map
		require.Equal(t, map[string]string{"color": "blue", "size": "m"}, f.Attributes)
		require.Equal(t, map[string][]string{"sale": {"yes", "new"}}, f.Tags)
		require.Equal(t, map[string]int{"max": 5}, f.Limits)
		require.Nil(t, f.Unused)
	})

	t.Run("comma splitting", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New(Config{EnableSplittingOnParsers: true}))
		c.Request().URI().SetQueryString("tags[sale]=yes,new&tags[a][b]=x,y&attributes[color]=red,blue")

		f := new(Filter)
		require.NoError(t, c.Bind().Query(f))
		require.Equal(t, map[string][]string{"sale": {"yes", "new"}, "a.b": {"x", "y"}}, f.Tags)
		// only slice values are split
		require.Equal(t, map[string]string{"color": "red,blue"}, f.Attributes)
	})

	t.Run("conversion error", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New())
		c.Request().URI().SetQueryString("limits[max]=many")

		err := c.Bind().Query(new(Filter))
		require.Error(t, err)
		require.Contains(t, err.Error(), `"limits.max"`)
	})
}

func Test_Bind_Body_MapField(t *testing.T) {
	t.Parallel()
	type Filter struct {
		Attributes map[string]string   `form:"attributes"`
		Tags       map[string][]string `form:"tags"`
		Limits     map[string]int      `form:"limits"`
	}
	want := Filter{
		Attributes: map[string]string{"color": "red", "size": "m"},
		Tags:       map[string][]string{"sale": {"yes", "new"}},
		Limits:     map[string]int{"max": 5},
	}
	fields := [][2]string{
		{"attributes[color]", "red"},
		{"attributes.size", "m"},
		{"tags[sale]", "yes"},
		{"tags[sale]", "new"},
		{"limits[max]", "5"},
	}

	t.Run("urlencoded", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New())
		pairs := make([]string, 0, len(fields))
		for _, field := range fields {
			pairs = append(pairs, field[0]+"="+field[1])
		}
		c.Request().Header.SetContentType(MIMEApplicationForm)
		c.Request().SetBodyString(strings.Join(pairs, "&"))

		f := new(Filter)
		require.NoError(t, c.Bind().Body(f))
		require.Equal(t, want, *f)

		f = new(Filter)
		require.NoError(t, c.Bind().Form(f))
		require.Equal(t, want, *f)
	})

	t.Run("multipart", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New())
		form := bindTestMultipart(t, "", fields...)
		c.Request().Header.SetContentType(form.ContentType)
		c.Request().SetBody(form.Body)

		f := new(Filter)
		require.NoError(t, c.Bind().Body(f))
		require.Equal(t, want, *f)
	})
}

func Test_Bind_Query_Schema(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	// bindQuery binds query into out and requires either success, or the failed check named by wantErr.
	bindQuery := func(out any, query, wantErr string) {
		t.Helper()
		c.Request().URI().SetQueryString(query)
		if wantErr == "" {
			require.NoError(t, c.Bind().Query(out))
			return
		}
		bindTestRequireMultiError(t, c.Bind().Query(out), wantErr)
	}

	type Query1 struct {
		Name   string `query:"name,required"`
		Nested struct {
			Age int `query:"age"`
		} `query:"nested,required"`
	}
	bindQuery(new(Query1), "name=tom&nested.age=10", "")
	bindQuery(new(Query1), "namex=tom&nested.age=10", `bind "name" from query: name is empty`)
	bindQuery(new(Query1), "name=tom&nested.agex=10", "")
	bindQuery(new(Query1), "name=tom&test.age=10", `bind "nested" from query: nested is empty`)

	type Query2 struct {
		Name   string `query:"name"`
		Nested struct {
			Age int `query:"age,required"`
		} `query:"nested"`
	}
	bindQuery(new(Query2), "name=tom&nested.age=10", "")
	bindQuery(new(Query2), "nested.age=10", "")
	bindQuery(new(Query2), "nested.agex=10", `bind "nested.age" from query: nested.age is empty`)
	bindQuery(new(Query2), "nested.agex=10", `bind "nested.age" from query: nested.age is empty`)

	type Node struct {
		Next  *Node `query:"next,required"`
		Value int   `query:"val,required"`
	}
	n := new(Node)
	bindQuery(n, "val=1&next.val=3", "")
	require.Equal(t, 1, n.Value)
	require.Equal(t, 3, n.Next.Value)

	bindQuery(new(Node), "next.val=2", `bind "val" from query: val is empty`)

	n = &Node{Next: new(Node)}
	bindQuery(n, "val=3&next.value=2", "")
	require.Equal(t, 3, n.Value)
	require.Equal(t, 0, n.Next.Value)

	type Person struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	type CollectionQuery struct {
		Data []Person `query:"data"`
	}

	// The same two elements, spelled in both notations the decoder understands.
	for _, tc := range []struct {
		name  string
		query string
	}{
		{name: "square brackets", query: "data[0][name]=john&data[0][age]=10&data[1][name]=doe&data[1][age]=12"},
		{name: "dot notation", query: "data.0.name=john&data.0.age=10&data.1.name=doe&data.1.age=12"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			c.Request().URI().SetQueryString(tc.query)
			cq := new(CollectionQuery)
			require.NoError(t, c.Bind().Query(cq))
			require.Len(t, cq.Data, 2)
			require.Equal(t, "john", cq.Data[0].Name)
			require.Equal(t, 10, cq.Data[0].Age)
			require.Equal(t, "doe", cq.Data[1].Name)
			require.Equal(t, 12, cq.Data[1].Age)
		})
	}
}

// Test_Bind_Sources binds structs from the sources that are filled one name at a time: the request header, the
// response header and the cookies. The query has its own scenarios in Test_Bind_Query.
func Test_Bind_Sources(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		src bindTestSource
		// splitting is EnableSplittingOnParsers: a value with commas then binds as several elements.
		splitting bool
	}{
		{src: bindTestHeader},
		{src: bindTestRespHeader, splitting: true},
		{src: bindTestCookie, splitting: true},
	} {
		t.Run(tc.src.name, func(t *testing.T) {
			t.Parallel()
			src := tc.src
			c := apptest.Ctx(t, New(Config{EnableSplittingOnParsers: tc.splitting}))

			// The elements "golang,fiber", "golang,fiber,go" and "milo,coke,pepsi" bind to.
			wantHobbies, wantMoreHobbies, wantDrinks := 1, 1, []string{"milo,coke,pepsi"}
			if tc.splitting {
				wantHobbies, wantMoreHobbies, wantDrinks = 2, 3, []string{"milo", "coke", "pepsi"}
			}

			type Fields struct {
				Name  string
				Hobby []string
				ID    int
			}
			c.Request().SetBody([]byte(``))
			c.Request().Header.SetContentType("")

			src.set(c, "id", "1", "Name", "John Doe", "Hobby", "golang,fiber")
			q := new(Fields)
			require.NoError(t, src.bind(c, q))
			require.Len(t, q.Hobby, wantHobbies)

			src.del(c, "hobby")
			src.add(c, "Hobby", "golang,fiber,go")
			q = new(Fields)
			require.NoError(t, src.bind(c, q))
			require.Len(t, q.Hobby, wantMoreHobbies)

			// The query is empty, so there is no hobby to bind.
			empty := new(Fields)
			src.del(c, "hobby")
			require.NoError(t, c.Bind().Query(empty))
			require.Empty(t, empty.Hobby)

			type Fields2 struct {
				Name            string
				Hobby           string
				FavouriteDrinks []string
				Empty           []string
				Alloc           []string
				No              []int64
				ID              int
				Bool            bool
			}

			src.set(c, "id", "2", "Name", "Jane Doe")
			src.del(c, "hobby")
			src.set(c, "Hobby", "go,fiber", "favouriteDrinks", "milo,coke,pepsi", "alloc", "", "no", "1")

			h2 := new(Fields2)
			h2.Bool = true
			h2.Name = helloWorld
			require.NoError(t, src.bind(c, h2))
			require.Equal(t, "go,fiber", h2.Hobby)
			require.True(t, h2.Bool)
			require.Equal(t, "Jane Doe", h2.Name) // check value get overwritten
			require.Equal(t, wantDrinks, h2.FavouriteDrinks)
			var nilSlice []string
			require.Equal(t, nilSlice, h2.Empty)
			require.Equal(t, []string{""}, h2.Alloc)
			require.Equal(t, []int64{1}, h2.No)

			type Required struct {
				Name string `header:"name,required" respHeader:"name,required" cookie:"name,required"`
			}
			src.del(c, "name")
			bindTestRequireMultiError(t, src.bind(c, new(Required)), `bind "name" from `+src.source+`: name is empty`)
		})
	}
}

// Test_Bind_Sources_Map binds maps from the same sources as Test_Bind_Sources.
func Test_Bind_Sources_Map(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		src bindTestSource
		// splitting is EnableSplittingOnParsers: a value with commas then binds as several elements.
		splitting bool
	}{
		{src: bindTestHeader},
		{src: bindTestRespHeader},
		{src: bindTestCookie, splitting: true},
	} {
		t.Run(tc.src.name, func(t *testing.T) {
			t.Parallel()
			src := tc.src
			c := apptest.Ctx(t, New(Config{EnableSplittingOnParsers: tc.splitting}))

			// The elements "golang,fiber" and "golang,fiber,go" bind to.
			wantHobbies, wantMoreHobbies := 1, 1
			if tc.splitting {
				wantHobbies, wantMoreHobbies = 2, 3
			}

			c.Request().SetBody([]byte(``))
			c.Request().Header.SetContentType("")

			src.set(c, "id", "1", "Name", "John Doe", "Hobby", "golang,fiber")
			q := make(map[string][]string)
			require.NoError(t, src.bind(c, &q))
			require.Len(t, q["Hobby"], wantHobbies)

			src.del(c, "hobby")
			src.add(c, "Hobby", "golang,fiber,go")
			q = make(map[string][]string)
			require.NoError(t, src.bind(c, &q))
			require.Len(t, q["Hobby"], wantMoreHobbies)

			// The query is empty, so there is no hobby to bind.
			empty := make(map[string][]string)
			src.del(c, "hobby")
			require.NoError(t, c.Bind().Query(&empty))
			require.Empty(t, empty["Hobby"])
		})
	}
}

// Test_Bind_Sources_Schema binds nested, required and recursive fields from the request header and the cookies.
func Test_Bind_Sources_Schema(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		src bindTestSource
		// nestedAge is the name of the required Fields2.Nested.Age as the source's tag spells it.
		nestedAge string
	}{
		{src: bindTestHeader, nestedAge: "Nested.age"},
		{src: bindTestCookie, nestedAge: "Nested.Age"},
	} {
		t.Run(tc.src.name, func(t *testing.T) {
			t.Parallel()
			src := tc.src
			c := apptest.Ctx(t, New())
			isEmpty := func(field string) string {
				return fmt.Sprintf("bind %q from %s: %s is empty", field, src.source, field)
			}

			type Fields1 struct {
				Name   string `header:"Name,required" cookie:"Name,required"`
				Nested struct {
					Age int `header:"Age" cookie:"Age"`
				} `header:"Nested,required" cookie:"Nested,required"`
			}
			c.Request().SetBody([]byte(``))
			c.Request().Header.SetContentType("")

			src.set(c, "Name", "tom", "Nested.Age", "10")
			q := new(Fields1)
			require.NoError(t, src.bind(c, q))

			src.del(c, "Name")
			q = new(Fields1)
			bindTestRequireMultiError(t, src.bind(c, q), isEmpty("Name"))

			src.add(c, "Name", "tom")
			src.del(c, "Nested.Age")
			src.add(c, "Nested.Agex", "10")
			q = new(Fields1)
			require.NoError(t, src.bind(c, q))

			src.del(c, "Nested.Agex")
			q = new(Fields1)
			bindTestRequireMultiError(t, src.bind(c, q), isEmpty("Nested"))

			src.del(c, "Nested.Agex")
			src.del(c, "Name")

			type Fields2 struct {
				Name   string `header:"Name" cookie:"Name"`
				Nested struct {
					Age int `header:"age,required" cookie:"Age,required"`
				} `header:"Nested" cookie:"Nested"`
			}

			src.set(c, "Name", "tom", "Nested.Age", "10")

			h2 := new(Fields2)
			require.NoError(t, src.bind(c, h2))

			src.del(c, "Name")
			h2 = new(Fields2)
			require.NoError(t, src.bind(c, h2))

			src.del(c, "Name")
			src.del(c, "Nested.Age")
			src.add(c, "Nested.Agex", "10")
			h2 = new(Fields2)
			bindTestRequireMultiError(t, src.bind(c, h2), isEmpty(tc.nestedAge))

			type Node struct {
				Next  *Node `header:"Next,required" cookie:"Next,required"`
				Value int   `header:"Val,required" cookie:"Val,required"`
			}
			src.set(c, "Val", "1", "Next.Val", "3")
			n := new(Node)
			require.NoError(t, src.bind(c, n))
			require.Equal(t, 1, n.Value)
			require.Equal(t, 3, n.Next.Value)

			src.del(c, "Val")
			n = new(Node)
			bindTestRequireMultiError(t, src.bind(c, n), isEmpty("Val"))

			src.add(c, "Val", "3")
			src.del(c, "Next.Val")
			src.add(c, "Next.Value", "2")
			n = new(Node)
			n.Next = new(Node)
			require.NoError(t, src.bind(c, n))
			require.Equal(t, 3, n.Value)
			require.Equal(t, 0, n.Next.Value)
		})
	}
}

// Test_Bind_WithSetParserDecoder binds with a decoder that was told how to parse a custom type, for each source
// that has its own decoder.
//
// Neither this test nor its subtests are parallel: binder.SetParserDecoder replaces the decoders every bind in the
// process uses, so no other test may bind while one of them is installed.
func Test_Bind_WithSetParserDecoder(t *testing.T) {
	type NonRFCTime time.Time

	setDecoder := func(aliasTag string) {
		binder.SetParserDecoder(binder.ParserConfig{
			IgnoreUnknownKeys: true,
			ParserType: []binder.ParserType{{
				CustomType: NonRFCTime{},
				Converter: func(value string) reflect.Value {
					if v, err := time.Parse("2006-01-02", value); err == nil {
						return reflect.ValueOf(v)
					}
					return reflect.Value{}
				},
			}},
			ZeroEmpty:   true,
			SetAliasTag: aliasTag,
		})
	}

	// Each binder reads the tag of its own source only: the header and the cookie binders match field names here.
	type NonRFCTimeInput struct {
		Date  NonRFCTime `query:"date" req:"date" cerez:"date"`
		Title string     `query:"title" req:"title" cerez:"title"`
		Body  string     `query:"body" req:"body" cerez:"body"`
	}

	for _, tc := range []struct {
		fill     func(c Ctx) // sets the date, the title and the body
		blank    func(c Ctx) // then gives the title an empty value
		src      bindTestSource
		aliasTag string
	}{
		{
			src: bindTestQuery, aliasTag: "query",
			fill:  func(c Ctx) { c.Request().URI().SetQueryString("date=2021-04-10&title=CustomDateTest&Body=October") },
			blank: func(c Ctx) { c.Request().URI().SetQueryString("date=2021-04-10&title&Body=October") },
		},
		{
			src: bindTestHeader, aliasTag: "req",
			fill:  func(c Ctx) { bindTestHeader.set(c, "Date", "2021-04-10", "Title", "CustomDateTest", "Body", "October") },
			blank: func(c Ctx) { bindTestHeader.add(c, "Title", "") },
		},
		{
			src: bindTestCookie, aliasTag: "cerez",
			fill:  func(c Ctx) { bindTestCookie.set(c, "Date", "2021-04-10", "Title", "CustomDateTest", "Body", "October") },
			blank: func(c Ctx) { bindTestCookie.add(c, "Title", "") },
		},
	} {
		t.Run(tc.src.name, func(t *testing.T) {
			setDecoder(tc.aliasTag)

			c := apptest.Ctx(t, New())

			c.Request().SetBody([]byte(``))
			c.Request().Header.SetContentType("")
			r := new(NonRFCTimeInput)

			tc.fill(c)
			require.NoError(t, tc.src.bind(c, r))
			require.Equal(t, "CustomDateTest", r.Title)
			date := fmt.Sprintf("%v", r.Date)
			require.Equal(t, "{0 63753609600 <nil>}", date)
			require.Equal(t, "October", r.Body)

			tc.blank(c)
			r = &NonRFCTimeInput{Title: "Existing title", Body: "Existing Body"}
			require.NoError(t, tc.src.bind(c, r))
			require.Empty(t, r.Title)
		})
	}

	t.Run("Body", func(t *testing.T) {
		setDecoder("form")

		c := apptest.Ctx(t, New())

		type Demo struct {
			Date  NonRFCTime `form:"date"`
			Title string     `form:"title"`
			Body  string     `form:"body"`
		}

		testDecodeParser := func(contentType, body string) {
			(&RequestConfig{ContentType: contentType, Body: []byte(body)}).ApplyTo(c)
			d := Demo{Title: "Existing title", Body: "Existing Body"}
			require.NoError(t, c.Bind().Body(&d))
			date := fmt.Sprintf("%v", d.Date)
			require.Equal(t, "{0 63743587200 <nil>}", date)
			require.Empty(t, d.Title)
			require.Equal(t, "New Body", d.Body)
		}

		testDecodeParser(MIMEApplicationForm, "date=2020-12-15&title=&body=New Body")
		testDecodeParser(MIMEMultipartForm+`; boundary="b"`, "--b\r\nContent-Disposition: form-data; name=\"date\"\r\n\r\n2020-12-15\r\n--b\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\n\r\n--b\r\nContent-Disposition: form-data; name=\"body\"\r\n\r\nNew Body\r\n--b--")
	})
}

func Benchmark_Bind_Query(b *testing.B) {
	var err error
	c := bindBenchQuery(b, New(), "id=1&name=tom&hobby=basketball&hobby=football")
	q := new(bindBenchFields)
	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().Query(q)
	}

	require.NoError(b, err)
	require.Equal(b, "tom", q.Name)
	require.Equal(b, 1, q.ID)
	require.Len(b, q.Hobby, 2)
}

func Benchmark_Bind_Query_Default(b *testing.B) {
	var err error
	c := bindBenchQuery(b, New(), "")
	type Query struct {
		Name  string   `query:"name,default:tom"`
		Hobby []string `query:"hobby,default:football|basketball"`
		ID    int      `query:"id,default:1"`
	}
	q := new(Query)
	b.ReportAllocs()
	for b.Loop() {
		*q = Query{}
		err = c.Bind().Query(q)
	}

	require.NoError(b, err)
	require.Equal(b, "tom", q.Name)
	require.Equal(b, 1, q.ID)
	require.Len(b, q.Hobby, 2)
}

func Benchmark_Bind_Query_Map(b *testing.B) {
	var err error
	c := bindBenchQuery(b, New(), "id=1&name=tom&hobby=basketball&hobby=football")
	q := make(map[string][]string)
	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().Query(&q)
	}
	require.NoError(b, err)
}

func Benchmark_Bind_Query_WithParseParam(b *testing.B) {
	var err error
	c := bindBenchQuery(b, New(), "data[0][name]=john&data[0][age]=10")
	type Person struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}
	type CollectionQuery struct {
		Data []Person `query:"data"`
	}
	cq := new(CollectionQuery)

	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().Query(cq)
	}

	require.NoError(b, err)
}

func Benchmark_Bind_Query_Comma(b *testing.B) {
	var err error
	c := bindBenchQuery(b, New(Config{EnableSplittingOnParsers: true}), "id=1&name=tom&hobby=basketball,football")
	q := new(bindBenchFields)
	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().Query(q)
	}
	require.NoError(b, err)
}

func Benchmark_Bind_Header(b *testing.B) {
	var err error
	c := bindBenchSource(b, bindTestHeader)
	q := new(bindBenchFields)
	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().Header(q)
	}
	require.NoError(b, err)
}

func Benchmark_Bind_Header_Map(b *testing.B) {
	var err error
	c := bindBenchSource(b, bindTestHeader)
	q := make(map[string][]string)
	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().Header(&q)
	}
	require.NoError(b, err)
}

func Benchmark_Bind_RespHeader(b *testing.B) {
	var err error
	c := bindBenchSource(b, bindTestRespHeader)
	q := new(bindBenchFields)
	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().RespHeader(q)
	}
	require.NoError(b, err)
}

func Benchmark_Bind_RespHeader_Map(b *testing.B) {
	var err error
	c := bindBenchSource(b, bindTestRespHeader)
	q := make(map[string][]string)
	b.ReportAllocs()
	for b.Loop() {
		err = c.Bind().RespHeader(&q)
	}
	require.NoError(b, err)
}

func Test_Bind_Body(t *testing.T) {
	t.Parallel()
	app := New(Config{
		MsgPackEncoder: msgpack.Marshal,
		MsgPackDecoder: msgpack.Unmarshal,
		CBOREncoder:    cbor.Marshal,
		CBORDecoder:    cbor.Unmarshal,
	})
	reqBody := []byte(`{"name":"john"}`)

	type Demo struct {
		Name  string   `json:"name" xml:"name" form:"name" query:"name" msgpack:"name"`
		Names []string `json:"names" xml:"names" form:"names" query:"names" msgpack:"names"`
	}

	// The body is compressed and its Content-Encoding says how, so Body decompresses it before it decodes the JSON.
	for _, tc := range []struct {
		compress func(dst, src []byte) []byte
		name     string
		encoding string
	}{
		{name: "Gzip", encoding: "gzip", compress: fasthttp.AppendGzipBytes},
		{name: "Deflate", encoding: "deflate", compress: fasthttp.AppendDeflateBytes},
		{name: "Brotli", encoding: "br", compress: fasthttp.AppendBrotliBytes},
		{name: "Zstd", encoding: "zstd", compress: fasthttp.AppendZstdBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compressedBody := tc.compress(nil, reqBody)
			require.NotEqual(t, reqBody, compressedBody)

			c := apptest.Ctx(t, app)
			c.Request().Header.SetContentType(MIMEApplicationJSON)
			c.Request().Header.Set(fasthttp.HeaderContentEncoding, tc.encoding)
			c.Request().SetBody(compressedBody)
			c.Request().Header.SetContentLength(len(compressedBody))
			d := new(Demo)
			require.NoError(t, c.Bind().Body(d))
			require.Equal(t, "john", d.Name)
		})
	}

	// bindBody binds the body of a request with the given Content-Type into out.
	bindBody := func(t *testing.T, contentType string, body []byte, out any) error {
		t.Helper()
		c := apptest.Ctx(t, app)
		(&RequestConfig{ContentType: contentType, Body: body}).ApplyTo(c)
		return c.Bind().Body(out)
	}

	testDecodeParser := func(t *testing.T, contentType string, body []byte) {
		t.Helper()
		d := new(Demo)
		require.NoError(t, bindBody(t, contentType, body, d))
		require.Equal(t, "john", d.Name)
	}

	t.Run("JSON", func(t *testing.T) { testDecodeParser(t, MIMEApplicationJSON, []byte(`{"name":"john"}`)) })
	t.Run("MsgPack", func(t *testing.T) {
		testDecodeParser(t, MIMEApplicationMsgPack, []byte{0x81, 0xa4, 0x6e, 0x61, 0x6d, 0x65, 0xa4, 0x6a, 0x6f, 0x68, 0x6e})
		require.Error(t, bindBody(t, MIMEApplicationMsgPack, []byte{0xFF, 0xFF}, new(Demo)))
	})
	t.Run("CBOR", func(t *testing.T) {
		enc, err := cbor.Marshal(&Demo{Name: "john"})
		require.NoError(t, err)
		testDecodeParser(t, MIMEApplicationCBOR, enc)

		t.Run("Invalid", func(t *testing.T) {
			c := apptest.Ctx(t, app)
			c.Request().Header.SetContentType(MIMEApplicationCBOR)
			c.Request().SetBody([]byte{0xFF, 0xFF}) // not CBOR
			require.Error(t, c.Bind().Body(new(Demo)))
		})
	})

	t.Run("XML", func(t *testing.T) {
		testDecodeParser(t, MIMEApplicationXML, []byte(`<Demo><name>john</name></Demo>`))
	})

	t.Run("Form", func(t *testing.T) { testDecodeParser(t, MIMEApplicationForm, []byte("name=john")) })

	t.Run("MultipartForm", func(t *testing.T) {
		testDecodeParser(t, MIMEMultipartForm+`;boundary="b"`, []byte("--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\njohn\r\n--b--"))
	})

	t.Run("ErrorInvalidContentType", func(t *testing.T) {
		require.Error(t, bindBody(t, "invalid-content-type", []byte(""), nil))
	})

	t.Run("ErrorUnknownContentTypeReturnsUnprocessableEntity", func(t *testing.T) {
		t.Parallel()
		err := bindBody(t, "application/unknown-type", []byte("some body"), new(Demo))
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessableEntity)
	})

	t.Run("ErrorMalformedMultipart", func(t *testing.T) {
		require.Error(t, bindBody(t, MIMEMultipartForm+`;boundary="b"`, []byte("--b"), nil))
	})

	type CollectionQuery struct {
		Data []Demo `query:"data"`
	}

	// Two elements of Data bound from a form, spelled in each notation the decoder understands.
	dotForm := bindTestMultipart(t, "", [2]string{"data.0.name", "john"}, [2]string{"data.1.name", "doe"})
	bracketForm := bindTestMultipart(t, "", [2]string{"data[0][name]", "john"}, [2]string{"data[1][name]", "doe"})
	for _, tc := range []struct {
		name        string
		contentType string
		body        []byte
	}{
		{name: "MultipartCollectionQueryDotNotation", contentType: dotForm.ContentType, body: dotForm.Body},
		{name: "MultipartCollectionQuerySquareBrackets", contentType: bracketForm.ContentType, body: bracketForm.Body},
		{name: "CollectionQuerySquareBrackets", contentType: MIMEApplicationForm, body: []byte("data[0][name]=john&data[1][name]=doe")},
		{name: "CollectionQueryDotNotation", contentType: MIMEApplicationForm, body: []byte("data.0.name=john&data.1.name=doe")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cq := new(CollectionQuery)
			require.NoError(t, bindBody(t, tc.contentType, tc.body, cq))
			require.Len(t, cq.Data, 2)
			require.Equal(t, "john", cq.Data[0].Name)
			require.Equal(t, "doe", cq.Data[1].Name)
		})
	}
}

func Benchmark_Bind_Body_JSON(b *testing.B) {
	body, err := json.Marshal(&bindBenchDemo{Name: "john"})
	require.NoError(b, err)
	c := bindBenchBody(b, New(), MIMEApplicationJSON, body)
	d := new(bindBenchDemo)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(d)
	}
	require.NoError(b, err)
	require.Equal(b, "john", d.Name)
}

func Benchmark_Bind_Body_MsgPack(b *testing.B) {
	var err error
	body := []byte{0x81, 0xa4, 0x6e, 0x61, 0x6d, 0x65, 0xa4, 0x6a, 0x6f, 0x68, 0x6e} // {"name":"john"}
	c := bindBenchBody(b, New(Config{
		MsgPackEncoder: msgpack.Marshal,
		MsgPackDecoder: msgpack.Unmarshal,
		CBOREncoder:    cbor.Marshal,
		CBORDecoder:    cbor.Unmarshal,
	}), MIMEApplicationMsgPack, body)
	d := new(bindBenchDemo)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(d)
	}
	require.NoError(b, err)
	require.Equal(b, "john", d.Name)
}

func Benchmark_Bind_Body_XML(b *testing.B) {
	var err error
	c := bindBenchBody(b, New(), MIMEApplicationXML, []byte("<Demo><name>john</name></Demo>"))
	d := new(bindBenchDemo)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(d)
	}
	require.NoError(b, err)
	require.Equal(b, "john", d.Name)
}

func Benchmark_Bind_Body_CBOR(b *testing.B) {
	body, err := cbor.Marshal(&bindBenchDemo{Name: "john"})
	require.NoError(b, err)
	c := bindBenchBody(b, New(Config{CBOREncoder: cbor.Marshal, CBORDecoder: cbor.Unmarshal}), MIMEApplicationCBOR, body)
	d := new(bindBenchDemo)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(d)
	}
	require.NoError(b, err)
	require.Equal(b, "john", d.Name)
}

func Benchmark_Bind_Body_Form(b *testing.B) {
	var err error
	c := bindBenchBody(b, New(), MIMEApplicationForm, []byte("name=john"))
	d := new(bindBenchDemo)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(d)
	}
	require.NoError(b, err)
	require.Equal(b, "john", d.Name)
}

func Benchmark_Bind_Body_MultipartForm(b *testing.B) {
	var err error
	form := bindTestMultipart(b, "", [2]string{"name", "john"})
	c := bindBenchBody(b, New(), form.ContentType, form.Body)
	d := new(bindBenchDemo)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(d)
	}

	require.NoError(b, err)
	require.Equal(b, "john", d.Name)
}

func Benchmark_Bind_Body_MultipartForm_Nested(b *testing.B) {
	var err error
	type Person struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}
	type Demo struct {
		Name    string   `form:"name"`
		Persons []Person `form:"persons"`
	}
	form := bindTestMultipart(b, "",
		[2]string{"name", "john"}, [2]string{"persons.0.name", "john"}, [2]string{"persons[0][age]", "10"},
		[2]string{"persons[1][name]", "doe"}, [2]string{"persons.1.age", "20"})
	c := bindBenchBody(b, New(), form.ContentType, form.Body)
	d := new(Demo)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(d)
	}

	require.NoError(b, err)
	require.Equal(b, "john", d.Name)
	require.Equal(b, "john", d.Persons[0].Name)
	require.Equal(b, 10, d.Persons[0].Age)
	require.Equal(b, "doe", d.Persons[1].Name)
	require.Equal(b, 20, d.Persons[1].Age)
}

func Benchmark_Bind_Body_Form_Map(b *testing.B) {
	var err error
	c := bindBenchBody(b, New(), MIMEApplicationForm, []byte("name=john"))
	d := make(map[string]string)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Body(&d)
	}
	require.NoError(b, err)
	require.Equal(b, "john", d["name"])
}

func Test_Bind_URI(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test1/userId/role/:roleId", func(c Ctx) error {
		type Demo struct {
			UserID uint `uri:"userId"`
			RoleID uint `uri:"roleId"`
		}
		d := new(Demo)
		if err := c.Bind().URI(d); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, uint(111), d.UserID)
		require.Equal(t, uint(222), d.RoleID)
		return nil
	})
	apptest.Do(t, app, "/test1/111/role/222")
	apptest.Do(t, app, "/test2/111/role/222")
}

func Test_Bind_URI_Map(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test1/userId/role/:roleId", func(c Ctx) error {
		d := make(map[string]string)

		if err := c.Bind().URI(&d); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, uint(111), d["userId"])
		require.Equal(t, uint(222), d["roleId"])
		return nil
	})
	apptest.Do(t, app, "/test1/111/role/222")
	apptest.Do(t, app, "/test2/111/role/222")
}

func Benchmark_Bind_URI(b *testing.B) {
	var err error

	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.route = &Route{Params: []string{"param1", "param2", "param3", "param4"}}
	c.values = [maxParams]string{"john", "doe", "is", "awesome"}

	var res struct {
		Param1 string `uri:"param1"`
		Param2 string `uri:"param2"`
		Param3 string `uri:"param3"`
		Param4 string `uri:"param4"`
	}

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().URI(&res)
	}

	require.NoError(b, err)
	require.Equal(b, "john", res.Param1)
	require.Equal(b, "doe", res.Param2)
	require.Equal(b, "is", res.Param3)
	require.Equal(b, "awesome", res.Param4)
}

func Benchmark_Bind_URI_Map(b *testing.B) {
	var err error

	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.route = &Route{Params: []string{"param1", "param2", "param3", "param4"}}
	c.values = [maxParams]string{"john", "doe", "is", "awesome"}

	res := make(map[string]string)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().URI(&res)
	}

	require.NoError(b, err)
	require.Equal(b, "john", res["param1"])
	require.Equal(b, "doe", res["param2"])
	require.Equal(b, "is", res["param3"])
	require.Equal(b, "awesome", res["param4"])
}

func Benchmark_Bind_Cookie(b *testing.B) {
	var err error
	c := bindBenchSource(b, bindTestCookie)
	q := new(bindBenchFields)
	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Cookie(q)
	}
	require.NoError(b, err)
}

func Benchmark_Bind_Cookie_Map(b *testing.B) {
	var err error
	c := bindBenchSource(b, bindTestCookie)
	q := make(map[string][]string)
	b.ReportAllocs()

	for b.Loop() {
		err = c.Bind().Cookie(&q)
	}
	require.NoError(b, err)
}

type customBinder struct{}

func (*customBinder) Name() string {
	return "custom"
}

func (*customBinder) MIMETypes() []string {
	return []string{"test", "test2"}
}

func (*customBinder) Parse(c Ctx, out any) error {
	return json.Unmarshal(c.Body(), out)
}

// customBinderReturningError returns a fixed error for testing extractFieldFromError branches.
type customBinderReturningError struct {
	err      error
	mimeType string
}

func (*customBinderReturningError) Name() string {
	return "error-binder"
}

func (b *customBinderReturningError) MIMETypes() []string {
	if b.mimeType != "" {
		return []string{b.mimeType}
	}
	return []string{"application/x-unknown-key-test", "application/x-empty-field-test"}
}

func (b *customBinderReturningError) Parse(_ Ctx, _ any) error {
	return b.err
}

func Test_Bind_CustomBinder(t *testing.T) {
	app := New()
	c := apptest.Ctx(t, app)

	customBinder := &customBinder{}
	app.RegisterCustomBinder(customBinder)

	type Demo struct {
		Name string `json:"name"`
	}
	(&RequestConfig{ContentType: "test", Body: []byte(`{"name":"john"}`)}).ApplyTo(c)
	d := new(Demo)

	require.NoError(t, c.Bind().Body(d))
	require.NoError(t, c.Bind().Custom("custom", d))
	require.Equal(t, ErrCustomBinderNotFound, c.Bind().Custom("not_custom", d))
	require.Equal(t, "john", d.Name)
}

func Test_Bind_CustomBinder_Source(t *testing.T) {
	t.Parallel()
	app := New()
	c := apptest.Ctx(t, app)

	app.RegisterCustomBinder(&customBinder{})

	type Demo struct {
		Name string `json:"name"`
	}
	c.Request().SetBody([]byte(`{invalid json`))
	c.Request().Header.SetContentLength(14)

	err := c.Bind().Custom("custom", new(Demo))
	require.Error(t, err)
	var be *BindError
	require.ErrorAs(t, err, &be)
	require.Equal(t, "custom", be.Source)
}

func Test_Bind_CustomBinder_Validation(t *testing.T) {
	t.Parallel()
	app := New(Config{StructValidator: &structValidator{}})
	app.RegisterCustomBinder(&customBinder{})

	bindBody := func(c Ctx, out any) error { return c.Bind().Body(out) }
	bindCustom := func(c Ctx, out any) error { return c.Bind().Custom("custom", out) }
	const invalidName = "you should have entered right name"

	for _, tc := range []struct {
		bind        func(c Ctx, out any) error
		name        string
		contentType string // selects the custom binder when Body binds
		body        string
		wantErr     string // empty when the struct validates
	}{
		{name: "Body_custom_binder_validation_pass", bind: bindBody, contentType: "test", body: `{"name":"john"}`},
		{name: "Body_custom_binder_validation_fail", bind: bindBody, contentType: "test", body: `{"name":"invalid"}`, wantErr: invalidName},
		{name: "Custom_binder_validation_pass", bind: bindCustom, body: `{"name":"john"}`},
		{name: "Custom_binder_validation_fail", bind: bindCustom, body: `{"name":"invalid"}`, wantErr: invalidName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			(&RequestConfig{ContentType: tc.contentType, Body: []byte(tc.body)}).ApplyTo(c)

			out := new(simpleQuery)
			err := tc.bind(c, out)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "john", out.Name)
		})
	}
}

func Test_Bind_WithAutoHandling(t *testing.T) {
	c := apptest.Ctx(t, New())

	type RequiredQuery struct {
		Name string `query:"name,required"`
	}
	rq := new(RequiredQuery)
	c.Request().URI().SetQueryString("")
	err := c.Bind().WithAutoHandling().Query(rq)
	require.Equal(t, StatusBadRequest, c.Response().StatusCode())
	require.Equal(t, "Bad request: name is empty", err.Error())
}

// structValidator accepts a simpleQuery only when its Name is john.
type structValidator struct{}

func (*structValidator) Validate(out any) error {
	out = reflect.ValueOf(out).Elem().Interface()
	sq, ok := out.(simpleQuery)
	if !ok {
		return errors.New("failed to type-assert to simpleQuery")
	}

	if sq.Name != "john" {
		return errors.New("you should have entered right name")
	}

	return nil
}

type simpleQuery struct {
	Name string `query:"name" json:"name"`
}

type countingStructValidator struct {
	calls int
}

func (v *countingStructValidator) Validate(_ any) error {
	v.calls++

	return nil
}

func Test_Bind_Form_Map_SkipsStructValidator(t *testing.T) {
	t.Parallel()
	form := &RequestConfig{ContentType: MIMEApplicationForm, Body: []byte("name=john")}

	t.Run("without validator", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New())

		form.ApplyTo(c)
		req := make(map[string]string)
		require.NoError(t, c.Bind().Form(&req))
		require.Equal(t, "john", req["name"])
	})

	t.Run("with struct validator configured", func(t *testing.T) {
		t.Parallel()
		validator := &countingStructValidator{}
		c := apptest.Ctx(t, New(Config{StructValidator: validator}))

		form.ApplyTo(c)
		req := make(map[string]string)
		require.NoError(t, c.Bind().Form(&req))
		require.Equal(t, "john", req["name"])
		require.Equal(t, 0, validator.calls)
	})
}

func Test_Bind_SkipValidation(t *testing.T) {
	t.Parallel()
	app := New(Config{StructValidator: &structValidator{}})

	t.Run("validation enabled", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)

		rq := new(simpleQuery)
		c.Request().URI().SetQueryString("name=efe")
		require.Equal(t, "you should have entered right name", c.Bind().SkipValidation(false).Query(rq).Error())
	})

	t.Run("validation skipped", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)

		rq := new(simpleQuery)
		c.Request().URI().SetQueryString("name=efe")
		require.NoError(t, c.Bind().SkipValidation(true).Query(rq))
		require.Equal(t, "efe", rq.Name)
	})
}

func Test_Bind_SkipValidation_Usage(t *testing.T) {
	t.Parallel()
	app := New(Config{StructValidator: &structValidator{}})
	// The validator rejects every name but john.
	efe := &RequestConfig{ContentType: MIMEApplicationJSON, Body: []byte(`{"name":"efe"}`)}

	t.Run("skip validation for json", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)
		efe.ApplyTo(c)

		req := new(simpleQuery)
		require.NoError(t, c.Bind().SkipValidation(true).JSON(req))
		require.Equal(t, "efe", req.Name)
	})

	t.Run("re-enable validation explicitly", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)
		efe.ApplyTo(c)

		req := new(simpleQuery)
		require.EqualError(t, c.Bind().SkipValidation(false).JSON(req), "you should have entered right name")
	})

	t.Run("toggle on same bind instance", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)
		efe.ApplyTo(c)

		req := new(simpleQuery)
		require.NoError(t, c.Bind().SkipValidation(true).JSON(req))

		efe.ApplyTo(c)
		req = new(simpleQuery)
		require.EqualError(t, c.Bind().SkipValidation(false).JSON(req), "you should have entered right name")
	})
}

func Test_Bind_ValidateStruct_NilTarget(t *testing.T) {
	t.Parallel()
	validator := &countingStructValidator{}
	c := apptest.Ctx(t, New(Config{StructValidator: validator}))

	require.NoError(t, c.Bind().validateStruct(nil))
	require.Equal(t, 0, validator.calls)
}

func Test_Bind_StructValidator(t *testing.T) {
	c := apptest.Ctx(t, New(Config{StructValidator: &structValidator{}}))

	rq := new(simpleQuery)
	c.Request().URI().SetQueryString("name=efe")
	require.Equal(t, "you should have entered right name", c.Bind().Query(rq).Error())

	rq = new(simpleQuery)
	c.Request().URI().SetQueryString("name=john")
	require.NoError(t, c.Bind().Query(rq))
}

func Test_Bind_RepeatParserWithSameStruct(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{CBOREncoder: cbor.Marshal, CBORDecoder: cbor.Unmarshal}))

	type Request struct {
		QueryParam  string `query:"query_param"`
		HeaderParam string `header:"header_param"`
		BodyParam   string `json:"body_param" xml:"body_param" form:"body_param"`
	}

	r := new(Request)

	c.Request().URI().SetQueryString("query_param=query_param")
	require.NoError(t, c.Bind().Query(r))
	require.Equal(t, "query_param", r.QueryParam)

	c.Request().Header.Add("header_param", "header_param")
	require.NoError(t, c.Bind().Header(r))
	require.Equal(t, "header_param", r.HeaderParam)

	var gzipJSON bytes.Buffer
	w := gzip.NewWriter(&gzipJSON)
	_, err := w.Write([]byte(`{"body_param":"body_param"}`))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	c.Request().Header.Set(HeaderContentEncoding, "gzip")
	(&RequestConfig{ContentType: MIMEApplicationJSON, Body: gzipJSON.Bytes()}).ApplyTo(c)
	require.NoError(t, c.Bind().Body(r))
	require.Equal(t, "body_param", r.BodyParam)
	c.Request().Header.Del(HeaderContentEncoding)

	testDecodeParser := func(contentType, body string) {
		(&RequestConfig{ContentType: contentType, Body: []byte(body)}).ApplyTo(c)
		require.NoError(t, c.Bind().Body(r))
		require.Equal(t, "body_param", r.BodyParam)
	}

	cb, err := cbor.Marshal(&Request{BodyParam: "body_param"})
	require.NoError(t, err, "Failed to marshal CBOR data")

	testDecodeParser(MIMEApplicationJSON, `{"body_param":"body_param"}`)
	testDecodeParser(MIMEApplicationXML, `<Demo><body_param>body_param</body_param></Demo>`)
	testDecodeParser(MIMEApplicationCBOR, string(cb))
	testDecodeParser(MIMEApplicationForm, "body_param=body_param")
	testDecodeParser(MIMEMultipartForm+`;boundary="b"`, "--b\r\nContent-Disposition: form-data; name=\"body_param\"\r\n\r\nbody_param\r\n--b--")
}

// RequestConfig describes the parts of a request a bind test sets on the request of a context. A body comes with its
// Content-Length, and the parts that are left out are not touched.
type RequestConfig struct {
	Headers     map[string]string
	Cookies     map[string]string
	ContentType string
	Query       string
	Body        []byte
}

func (rc *RequestConfig) ApplyTo(ctx Ctx) {
	if rc.Body != nil {
		ctx.Request().SetBody(rc.Body)
		ctx.Request().Header.SetContentLength(len(rc.Body))
	}
	if rc.ContentType != "" {
		ctx.Request().Header.SetContentType(rc.ContentType)
	}
	for k, v := range rc.Headers {
		ctx.Request().Header.Set(k, v)
	}
	for k, v := range rc.Cookies {
		ctx.Request().Header.SetCookie(k, v)
	}
	if rc.Query != "" {
		ctx.Request().URI().SetQueryString(rc.Query)
	}
}

func Test_Bind_All(t *testing.T) {
	t.Parallel()
	type User struct {
		Avatar    *multipart.FileHeader `form:"avatar"`
		Name      string                `query:"name" json:"name" form:"name"`
		Email     string                `json:"email" form:"email"`
		Role      string                `header:"X-User-Role"`
		SessionID string                `json:"session_id" cookie:"session_id"`
		ID        int                   `uri:"id" query:"id" json:"id" form:"id"`
	}
	newBind := func(app *App) *Bind { return &Bind{ctx: app.AcquireCtx(&fasthttp.RequestCtx{})} }

	tests := []struct {
		out      any
		expected *User
		config   *RequestConfig
		name     string
		wantErr  bool
	}{
		{name: "Invalid output type", out: 123, wantErr: true},
		{
			name:     "Successful binding",
			out:      new(User),
			config:   bindTestAllRequest(),
			expected: &User{ID: 1, Name: "john", Email: "john@doe.com", Role: "admin", SessionID: "abc1234"},
		},
		{
			name:     "Missing fields (partial JSON only)",
			out:      new(User),
			config:   &RequestConfig{ContentType: MIMEApplicationJSON, Body: []byte(`{"name":"partial"}`)},
			expected: &User{Name: "partial"},
		},
		{
			name: "Override query with JSON",
			out:  new(User),
			config: &RequestConfig{
				ContentType: MIMEApplicationJSON,
				Body:        []byte(`{"name":"fromjson", "id": 99}`),
				Query:       "id=1&name=queryname",
			},
			expected: &User{Name: "fromjson", ID: 99},
		},
		{
			name: "Form binding",
			out:  new(User),
			config: &RequestConfig{
				ContentType: MIMEApplicationForm,
				Body:        []byte("id=2&name=formname&email=form@doe.com"),
			},
			expected: &User{ID: 2, Name: "formname", Email: "form@doe.com"},
		},
		{
			name:     "Skip body when content-type missing",
			out:      new(User),
			config:   &RequestConfig{Body: []byte(`{"name":"bodyname"}`), Query: "name=queryname"},
			expected: &User{Name: "queryname"},
		},
		{
			name:     "Skip empty body despite content-type",
			out:      new(User),
			config:   &RequestConfig{ContentType: MIMEApplicationJSON, Body: []byte{}, Query: "name=queryname"},
			expected: &User{Name: "queryname"},
		},
	}

	app := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bind := newBind(app)

			if tt.config != nil {
				tt.config.ApplyTo(bind.ctx)
			}

			err := bind.All(tt.out)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.expected != nil {
				actual, ok := tt.out.(*User)
				require.True(t, ok)

				require.Equal(t, tt.expected.ID, actual.ID)
				require.Equal(t, tt.expected.Name, actual.Name)
				require.Equal(t, tt.expected.Email, actual.Email)
				require.Equal(t, tt.expected.Role, actual.Role)
				require.Equal(t, tt.expected.SessionID, actual.SessionID)
			}
		})
	}
}

func Test_Bind_All_Uri_Precedence(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		ID    int    `uri:"id" json:"id" query:"id" form:"id"`
	}

	app := New()

	app.Post("/test1/:id", func(c Ctx) error {
		d := new(User)
		if err := c.Bind().All(d); err != nil {
			t.Fatal(err)
		}

		require.Equal(t, 111, d.ID)
		require.Equal(t, "john", d.Name)
		require.Equal(t, "john@doe.com", d.Email)
		return nil
	})

	body := strings.NewReader(`{"id": 999, "name": "john", "email": "john@doe.com"}`)
	req := httptest.NewRequest(MethodPost, "/test1/111?id=888", body)
	req.Header.Set("Content-Type", "application/json")
	apptest.Expect(t, app, req, 200)
}

func Test_Bind_All_Query_Precedence(t *testing.T) {
	t.Parallel()
	type Data struct {
		ID int `query:"id" header:"id" cookie:"id"`
	}

	c := apptest.Ctx(t, New())
	c.Request().URI().SetQueryString("id=5")
	c.Request().Header.Set("id", "3")
	c.Request().Header.SetCookie("id", "2")

	d := new(Data)
	require.NoError(t, (&Bind{ctx: c}).All(d))
	require.Equal(t, 5, d.ID)
}

func Test_Bind_All_StructValidator(t *testing.T) {
	t.Parallel()
	app := New(Config{StructValidator: &structValidator{}})

	// Success case: name comes from body only
	ctx := apptest.Ctx(t, app)
	ctx.Request().Header.SetContentType(MIMEApplicationJSON)
	ctx.Request().SetBody([]byte(`{"name":"john"}`))
	sq := new(simpleQuery)
	require.NoError(t, (&Bind{ctx: ctx}).All(sq))
	require.Equal(t, "john", sq.Name)

	// Failure: missing name everywhere
	ctxFail := apptest.Ctx(t, app)
	ctxFail.Request().Header.SetContentType(MIMEApplicationJSON)
	ctxFail.Request().SetBody([]byte(`{}`))
	sqFail := new(simpleQuery)
	require.EqualError(t, (&Bind{ctx: ctxFail}).WithoutAutoHandling().All(sqFail), "you should have entered right name")
}

func BenchmarkBind_All(b *testing.B) {
	type User struct {
		SessionID string `json:"session_id" cookie:"session_id"`
		Name      string `query:"name" json:"name" form:"name"`
		Email     string `json:"email" form:"email"`
		Role      string `header:"X-User-Role"`
		ID        int    `uri:"id" query:"id" json:"id" form:"id"`
	}

	c := apptest.Ctx(b, New())
	config := bindTestAllRequest()
	bind := &Bind{ctx: c}

	for b.Loop() {
		user := &User{}
		config.ApplyTo(c)
		if err := bind.All(user); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func Test_Bind_All_CustomPrecedence(t *testing.T) {
	t.Parallel()
	app := New()

	type CustomPrecedenceReq struct {
		Name string `binding_source:"query,header,cookie,body,uri" query:"name" header:"x-name" cookie:"c-name" json:"name" uri:"name"`
	}

	// The sources in the order of the binding_source tag. A case sets the sources from its own to the last one, so
	// its own, which comes first in that precedence, must win.
	sources := [...]struct {
		set  func(ctx *DefaultCtx)
		name string
		want string
	}{
		{name: "query wins", want: "from_query", set: func(ctx *DefaultCtx) {
			ctx.Request().URI().SetQueryString("name=from_query")
		}},
		{name: "header wins without query", want: "from_header", set: func(ctx *DefaultCtx) {
			ctx.Request().Header.Set("x-name", "from_header")
		}},
		{name: "cookie wins without header", want: "from_cookie", set: func(ctx *DefaultCtx) {
			ctx.Request().Header.SetCookie("c-name", "from_cookie")
		}},
		{name: "body wins without cookie", want: "from_body", set: func(ctx *DefaultCtx) {
			ctx.Request().Header.SetContentType(MIMEApplicationJSON)
			ctx.Request().SetBody([]byte(`{"name":"from_body"}`))
		}},
		{name: "uri wins without body", want: "from_uri", set: func(ctx *DefaultCtx) {
			ctx.route = &Route{Params: []string{"name"}}
			ctx.values = [maxParams]string{"from_uri"}
		}},
	}

	for i, tc := range sources {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := apptest.CtxAs[*DefaultCtx](t, app)
			for _, source := range sources[i:] {
				source.set(ctx)
			}

			req := new(CustomPrecedenceReq)
			require.NoError(t, ctx.Bind().All(req))
			require.Equal(t, tc.want, req.Name)
		})
	}
}

func Test_Bind_All_CustomPrecedence_InvalidTags(t *testing.T) {
	t.Parallel()
	type InvalidPrecedenceReq struct {
		BindingSource struct{} `binding_source:"query,invalid,cookie"`
		Name          string   `query:"name"`
	}

	type MultipleTagsReq struct {
		Field1 struct{} `binding_source:"query"`
		Field2 struct{} `binding_source:"header"`
	}

	for _, tc := range []struct {
		out     any
		name    string
		wantErr string
	}{
		{name: "InvalidToken", out: new(InvalidPrecedenceReq), wantErr: `unknown binding_source "invalid"`},
		{name: "MultipleTags", out: new(MultipleTagsReq), wantErr: "multiple binding_source tags found on struct MultipleTagsReq"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := apptest.Ctx(t, New())

			require.EqualError(t, ctx.Bind().All(tc.out), tc.wantErr)
		})
	}
}

func Test_Bind_All_CustomPrecedence_OmittedSources(t *testing.T) {
	t.Parallel()
	app := New()

	type OmittedPrecedenceReq struct {
		// Only query is bound. Headers and Body are ignored.
		BindingSource struct{} `binding_source:"query"`
		Name          string   `query:"name" header:"x-name" json:"name"`
	}

	ctx := apptest.Ctx(t, app)

	ctx.Request().Header.Set("x-name", "from_header")
	ctx.Request().Header.SetContentType(MIMEApplicationJSON)
	ctx.Request().SetBody([]byte(`{"name":"from_body"}`))

	// Set an unrelated query param, meaning Name will not be found in query
	ctx.Request().URI().SetQueryString("other=from_query")

	req := new(OmittedPrecedenceReq)
	require.NoError(t, ctx.Bind().All(req))

	// Because query is the only bound source, and "name" wasn't in the query, Name should be empty
	require.Empty(t, req.Name)

	ctx.Request().URI().SetQueryString("name=from_query")
	req2 := new(OmittedPrecedenceReq)
	require.NoError(t, ctx.Bind().All(req2))
	require.Equal(t, "from_query", req2.Name)
}

func Test_Bind_All_CustomPrecedence_DoesNotReadOmittedBody(t *testing.T) {
	t.Parallel()
	app := New()

	type QueryOnlyReq struct {
		BindingSource struct{} `binding_source:"query"`
		Name          string   `query:"name"`
	}

	ctx := apptest.Ctx(t, app)

	ctx.Request().Header.SetContentType(MIMEApplicationJSON)
	ctx.Request().SetBodyStream(bytes.NewBufferString(`{"name":"from_body"}`), -1)
	ctx.Request().URI().SetQueryString("name=from_query")

	req := new(QueryOnlyReq)
	require.NoError(t, ctx.Bind().All(req))
	require.Equal(t, "from_query", req.Name)
	require.True(t, ctx.Request().IsBodyStream(), "query-only binding must not consume the request body stream")
}

// With StreamRequestBody the server passes on a body of any size, so the binders read it within BodyLimit.
func Test_Bind_StreamedBodyLimit(t *testing.T) {
	t.Parallel()
	const limit = 1024
	app := New(Config{StreamRequestBody: true, BodyLimit: limit})

	type Req struct {
		Name string `json:"name" form:"name"`
	}

	for _, tc := range []struct {
		bind        func(c Ctx, out any) error
		name        string
		contentType string
		format      string
	}{
		{name: "Body JSON", contentType: MIMEApplicationJSON, format: `{"name":%q}`, bind: func(c Ctx, out any) error { return c.Bind().Body(out) }},
		{name: "Body form", contentType: MIMEApplicationForm, format: "name=%s", bind: func(c Ctx, out any) error { return c.Bind().Body(out) }},
		{name: "Form", contentType: MIMEApplicationForm, format: "name=%s", bind: func(c Ctx, out any) error { return c.Bind().Form(out) }},
		{name: "All JSON", contentType: MIMEApplicationJSON, format: `{"name":%q}`, bind: func(c Ctx, out any) error { return c.Bind().All(out) }},
		{name: "All form", contentType: MIMEApplicationForm, format: "name=%s", bind: func(c Ctx, out any) error { return c.Bind().All(out) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			c.Request().Header.SetContentType(tc.contentType)
			c.Request().SetBodyStream(strings.NewReader(fmt.Sprintf(tc.format, "john")), -1)
			out := new(Req)
			require.NoError(t, tc.bind(c, out))
			require.Equal(t, "john", out.Name)

			tooLarge := apptest.Ctx(t, app)
			stream := strings.NewReader(fmt.Sprintf(tc.format, strings.Repeat("a", 100*limit)))
			size := stream.Len()
			tooLarge.Request().Header.SetContentType(tc.contentType)
			tooLarge.Request().SetBodyStream(stream, -1)
			out = new(Req)
			require.Error(t, tc.bind(tooLarge, out))
			require.Empty(t, out.Name)
			require.Equal(t, size-limit-1, stream.Len(), "the rest of the stream is not read")
		})
	}
}

// All binds the body only with a Content-Type, so without one a streamed body stays unread.
func Test_Bind_All_StreamedBodyWithoutContentType(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{StreamRequestBody: true, BodyLimit: 8}))

	type Req struct {
		ID string `query:"id"`
	}

	stream := strings.NewReader(strings.Repeat("a", 1024))
	c.Request().URI().SetQueryString("id=1")
	c.Request().SetBodyStream(stream, -1)

	out := new(Req)
	require.NoError(t, c.Bind().All(out))
	require.Equal(t, "1", out.ID)
	require.Equal(t, 1024, stream.Len(), "the body is not read")
}

func Test_Bind_All_CustomPrecedence_Duplicates(t *testing.T) {
	t.Parallel()
	app := New()

	type DuplicatePrecedenceReq struct {
		BindingSource struct{} `binding_source:"query,query,header"`
		Name          string   `query:"name" header:"x-name"`
	}

	ctx := apptest.Ctx(t, app)

	ctx.Request().Header.Set("x-name", "from_header")

	req := new(DuplicatePrecedenceReq)
	require.NoError(t, ctx.Bind().All(req))
	require.Equal(t, "from_header", req.Name)
}

func BenchmarkBind_All_CustomPrecedence(b *testing.B) {
	type User struct {
		BindingSource struct{} `binding_source:"query,header,cookie,body,uri"`
		SessionID     string   `json:"session_id" cookie:"session_id"`
		Name          string   `query:"name" json:"name" form:"name"`
		Email         string   `json:"email" form:"email"`
		Role          string   `header:"X-User-Role"`
		ID            int      `uri:"id" query:"id" json:"id" form:"id"`
	}

	c := apptest.Ctx(b, New())
	config := bindTestAllRequest()
	bind := &Bind{ctx: c}

	for b.Loop() {
		user := &User{}
		config.ApplyTo(c)
		if err := bind.All(user); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// Test_Bind_Body_ContentTypeNormalization pins both halves of the
// Content-Type contract. The media type must be folded in place, because
// fasthttp and binder.FormBinding locate the form body with case-sensitive
// prefix checks — so a legal "Multipart/Form-Data" has to still bind. The
// parameters must NOT be folded, because a multipart boundary is
// case-sensitive and anything replaying the request (a proxy, an adaptor)
// would no longer parse the body it forwarded.
func Test_Bind_Body_ContentTypeNormalization(t *testing.T) {
	t.Parallel()
	bindBody := func(c Ctx, out any) error { return c.Bind().Body(out) }

	// Both the media type and the parameter names are case-insensitive
	// (RFC 9110 Sections 8.3.1 and 5.6.6); only the boundary value is not.
	for _, ctype := range []string{
		"multipart/form-data; boundary=AbCdEfMixed12345",
		"Multipart/Form-Data; boundary=AbCdEfMixed12345",
		"multipart/form-data; BOUNDARY=AbCdEfMixed12345",
		"multipart/form-data; Boundary=AbCdEfMixed12345",
		"Multipart/Form-Data; BOUNDARY=AbCdEfMixed12345",
		`multipart/form-data; CHARSET="a;b"; BOUNDARY=AbCdEfMixed12345`,
	} {
		t.Run("case-insensitive "+ctype, func(t *testing.T) {
			t.Parallel()
			body := bindTestMultipart(t, "AbCdEfMixed12345", [2]string{"name", "john"}).Body
			bindTestPostName(t, bindBody, ctype, body, func(c Ctx) {
				// The boundary value keeps its case so the request can be replayed.
				require.Contains(t, c.Get(HeaderContentType), "AbCdEfMixed12345")
			})
		})
	}

	t.Run("mixed-case urlencoded still binds", func(t *testing.T) {
		t.Parallel()
		bindTestPostName(t, bindBody, "APPLICATION/X-WWW-FORM-URLENCODED", []byte("name=john"), nil)
	})

	t.Run("boundary case is preserved", func(t *testing.T) {
		t.Parallel()
		raw := bindTestMultipart(t, "AbCdEfMixedCase12345", [2]string{"name", "john"}).Body

		const wantCType = "multipart/form-data; boundary=AbCdEfMixedCase12345"

		bindTestPostName(t, bindBody, wantCType, raw, func(c Ctx) {
			require.Equal(t, wantCType, c.Get(HeaderContentType))

			// Replaying the request from its header and body — what forwarding it
			// upstream amounts to — must still parse.
			fwd := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(fwd)
			fwd.Header.SetMethod(MethodPost)
			fwd.SetRequestURI("/")
			fwd.Header.SetContentType(c.Get(HeaderContentType))
			fwd.SetBody(raw)

			form, err := fwd.MultipartForm()
			require.NoError(t, err)
			require.Equal(t, []string{"john"}, form.Value["name"])
		})
	})
}

// Test_Bind_Form_ContentTypeNormalization covers the Form entry point
// directly. Bind().Body() normalizes the Content-Type before dispatching, but
// Bind().Form() reaches the binder without going through it — so a legal
// mixed-case media type bound nothing here and reported no error.
func Test_Bind_Form_ContentTypeNormalization(t *testing.T) {
	t.Parallel()
	bindForm := func(c Ctx, out any) error { return c.Bind().Form(out) }

	t.Run("multipart", func(t *testing.T) {
		t.Parallel()
		body := bindTestMultipart(t, "AbCdEfMixed12345", [2]string{"name", "john"}).Body
		bindTestPostName(t, bindForm, "Multipart/Form-Data; BOUNDARY=AbCdEfMixed12345", body, nil)
	})

	t.Run("urlencoded", func(t *testing.T) {
		t.Parallel()
		bindTestPostName(t, bindForm, "APPLICATION/X-WWW-FORM-URLENCODED", []byte("name=john"), nil)
	})
}

// ctxTypeBinder records the concrete context type handed to Parse.
type ctxTypeBinder struct {
	seen string
}

func (*ctxTypeBinder) Name() string {
	return "ctxtype"
}

func (*ctxTypeBinder) MIMETypes() []string {
	return []string{"application/x-ctxtype"}
}

func (b *ctxTypeBinder) Parse(c Ctx, _ any) error {
	if _, ok := c.(*customCtx); ok {
		b.seen = "custom"
		return nil
	}
	b.seen = "default"
	return nil
}

func Test_Bind_CustomBinder_CustomCtx(t *testing.T) {
	t.Parallel()
	app := NewWithCustomCtx(func(app *App) CustomCtx { return &customCtx{DefaultCtx: *NewDefaultCtx(app)} })
	typeBinder := &ctxTypeBinder{}
	app.RegisterCustomBinder(typeBinder)

	app.Post("/", func(c Ctx) error {
		var d struct {
			Name string `json:"name"`
		}
		if err := c.Bind().Custom("ctxtype", &d); err != nil {
			return err
		}
		viaCustom := typeBinder.seen
		typeBinder.seen = ""
		if err := c.Bind().Body(&d); err != nil {
			return err
		}
		return c.SendString(viaCustom + "," + typeBinder.seen)
	})

	req := httptest.NewRequest(MethodPost, "/", strings.NewReader(`{"name":"john"}`))
	req.Header.Set(HeaderContentType, "application/x-ctxtype")
	resp := apptest.Do(t, app, req)
	require.Equal(t, "custom,custom", apptest.Body(t, resp))
}

func Test_Bind_Body_FormKeyTooDeep(t *testing.T) {
	t.Parallel()

	// A JSON type is decoded from a form body too, by its field names.
	type category struct {
		Name     string     `json:"name"`
		Children []category `json:"children"`
	}

	app := New()
	ctxWithBody := func(levels int) Ctx {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(c) })
		c.Request().Header.SetContentType(MIMEApplicationForm)
		c.Request().SetBodyString(strings.Repeat("children.0.", levels) + "name=x")
		return c
	}

	require.NoError(t, ctxWithBody(31).Bind().Body(new(category)))

	err := ctxWithBody(32).Bind().Body(new(category))
	require.ErrorIs(t, err, binder.ErrKeyTooDeep)
	var bindErr *BindError
	require.ErrorAs(t, err, &bindErr)
	require.Equal(t, BindSourceBody, bindErr.Source)

	c := ctxWithBody(32)
	require.Error(t, c.Bind().WithAutoHandling().Body(new(category)))
	require.Equal(t, StatusBadRequest, c.Response().StatusCode())
}
