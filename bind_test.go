//nolint:wrapcheck,tagliatelle,bodyclose // We must not wrap errors in tests
package fiber

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/url"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Binder(t *testing.T) {
	t.Parallel()
	app := New()

	ctx := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
	ctx.values = [maxParams]string{"id string"}
	ctx.route = &Route{Params: []string{"id"}}
	ctx.Request().SetBody([]byte(`{"name": "john doe"}`))
	ctx.Request().Header.Set("content-type", "application/json")

	var req struct {
		ID *string `param:"id"`
	}

	var body struct {
		Name string `json:"name"`
	}

	err := ctx.Bind().Req(&req).JSON(&body).Err()
	require.NoError(t, err)
	require.Equal(t, "id string", *req.ID)
	require.Equal(t, "john doe", body.Name)
}

func Test_Binder_Nested(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("name=tom&nested.and.age=10&nested.and.test=john")

	// TODO: pointer support for structs
	var req struct {
		Name   string `query:"name"`
		Nested struct {
			And struct {
				Age  *int   `query:"age"`
				Test string `query:"test"`
			} `query:"and"`
		} `query:"nested"`
	}

	err := c.Bind().Req(&req).Err()
	require.NoError(t, err)
	require.Equal(t, "tom", req.Name)
	require.Equal(t, "john", req.Nested.And.Test)
	require.Equal(t, 10, *req.Nested.And.Age)
}

func Test_Binder_Nested_Slice(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("name=tom&data[0][name]=john&data[0][age]=10&data[1][name]=doe&data[1][age]=12")

	var req struct {
		Name string `query:"name"`
		Data []struct {
			Name string `query:"name"`
			Age  int    `query:"age"`
		} `query:"data"`
	}

	err := c.Bind().Req(&req).Err()
	require.NoError(t, err)
	require.Equal(t, 2, len(req.Data))
	require.Equal(t, "john", req.Data[0].Name)
	require.Equal(t, 10, req.Data[0].Age)
	require.Equal(t, "doe", req.Data[1].Name)
	require.Equal(t, 12, req.Data[1].Age)
	require.Equal(t, "tom", req.Name)
}

/*func Test_Binder_Nested_Deeper_Slice(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("data[0][users][0][name]=john&data[0][users][0][age]=10&data[1][users][0][name]=doe&data[1][users][0][age]=12")

	var req struct {
		Data []struct {
			Users []struct {
				Name string `query:"name"`
				Age  int    `query:"age"`
			} `query:"subData"`
		} `query:"data"`
	}

	err := c.Bind().Req(&req).Err()
	require.NoError(t, err)
	require.Equal(t, 2, len(req.Data))
	require.Equal(t, "john", req.Data[0].Users[0].Name)
	require.Equal(t, 10, req.Data[0].Users[0].Age)
	require.Equal(t, "doe", req.Data[1].Users[0].Name)
	require.Equal(t, 12, req.Data[1].Users[0].Age)
}*/

// go test -run Test_Bind_BasicType -v
func Test_Bind_BasicType(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Query struct {
		Flag bool `query:"enable"`

		I8  int8  `query:"i8"`
		I16 int16 `query:"i16"`
		I32 int32 `query:"i32"`
		I64 int64 `query:"i64"`
		I   int   `query:"i"`

		U8  uint8  `query:"u8"`
		U16 uint16 `query:"u16"`
		U32 uint32 `query:"u32"`
		U64 uint64 `query:"u64"`
		U   uint   `query:"u"`

		S string `query:"s"`
	}

	var q Query

	const qs = "i8=88&i16=166&i32=322&i64=644&i=101&u8=77&u16=165&u32=321&u64=643&u=99&s=john&enable=true"
	c.Request().URI().SetQueryString(qs)
	require.NoError(t, c.Bind().Req(&q).Err())

	require.Equal(t, Query{
		Flag: true,
		I8:   88,
		I16:  166,
		I32:  322,
		I64:  644,
		I:    101,
		U8:   77,
		U16:  165,
		U32:  321,
		U64:  643,
		U:    99,
		S:    "john",
	}, q)

	type Query2 struct {
		Flag []bool `query:"enable"`

		I8  []int8  `query:"i8"`
		I16 []int16 `query:"i16"`
		I32 []int32 `query:"i32"`
		I64 []int64 `query:"i64"`
		I   []int   `query:"i"`

		U8  []uint8  `query:"u8"`
		U16 []uint16 `query:"u16"`
		U32 []uint32 `query:"u32"`
		U64 []uint64 `query:"u64"`
		U   []uint   `query:"u"`

		S []string `query:"s"`
	}

	var q2 Query2

	c.Request().URI().SetQueryString(qs)
	require.NoError(t, c.Bind().Req(&q2).Err())

	require.Equal(t, Query2{
		Flag: []bool{true},
		I8:   []int8{88},
		I16:  []int16{166},
		I32:  []int32{322},
		I64:  []int64{644},
		I:    []int{101},
		U8:   []uint8{77},
		U16:  []uint16{165},
		U32:  []uint32{321},
		U64:  []uint64{643},
		U:    []uint{99},
		S:    []string{"john"},
	}, q2)

}

// go test -run Test_Bind_Query -v
func Test_Bind_Query(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Query struct {
		ID    int      `query:"id"`
		Name  string   `query:"name"`
		Hobby []string `query:"hobby"`
	}

	var q Query

	c.Request().SetBody([]byte{})
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")

	require.NoError(t, c.Bind().Req(&q).Err())
	require.Equal(t, 2, len(q.Hobby))

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football")
	require.NoError(t, c.Bind().Req(&q).Err())
	require.Equal(t, 1, len(q.Hobby))

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=scoccer&hobby=basketball,football")
	require.NoError(t, c.Bind().Req(&q).Err())
	require.Equal(t, 2, len(q.Hobby))

	c.Request().URI().SetQueryString("")
	require.NoError(t, c.Bind().Req(&q).Err())
	require.Equal(t, 0, len(q.Hobby))

	type Query2 struct {
		Bool            bool     `query:"bool"`
		ID              int      `query:"id"`
		Name            string   `query:"name"`
		Hobby           string   `query:"hobby"`
		FavouriteDrinks string   `query:"favouriteDrinks"`
		Empty           []string `query:"empty"`
		Alloc           []string `query:"alloc"`
		No              []int64  `query:"no"`
	}

	var q2 Query2

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1")
	require.NoError(t, c.Bind().Req(&q2).Err())
	require.Equal(t, "basketball,football", q2.Hobby)
	require.Equal(t, "tom", q2.Name) // check value get overwritten
	require.Equal(t, "milo,coke,pepsi", q2.FavouriteDrinks)
	require.Equal(t, []string{}, q2.Empty)
	require.Equal(t, []string{""}, q2.Alloc)
	require.Equal(t, []int64{1}, q2.No)

	type ArrayQuery struct {
		Data []string `query:"data[]"`
	}
	var aq ArrayQuery
	c.Request().URI().SetQueryString("data[]=john&data[]=doe")
	require.NoError(t, c.Bind().Req(&aq).Err())
	require.Equal(t, ArrayQuery{Data: []string{"john", "doe"}}, aq)
}

// go test -run Test_Bind_Resp_Header -v
func Test_Bind_Resp_Header(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type resHeader struct {
		Key string `respHeader:"k"`

		Keys []string `respHeader:"keys"`
	}

	c.Set("k", "vv")
	c.Response().Header.Add("keys", "v1")
	c.Response().Header.Add("keys", "v2")

	var q resHeader
	require.NoError(t, c.Bind().Req(&q).Err())
	require.Equal(t, "vv", q.Key)
	require.Equal(t, []string{"v1", "v2"}, q.Keys)
}

var _ Binder = (*userCtxUnmarshaler)(nil)

type userCtxUnmarshaler struct {
	V int
}

func (u *userCtxUnmarshaler) UnmarshalFiberCtx(ctx Ctx) error {
	u.V++
	return nil
}

// go test -run Test_Bind_CustomizedUnmarshaler -v
func Test_Bind_CustomizedUnmarshaler(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Req struct {
		Key userCtxUnmarshaler
	}

	var r Req
	require.NoError(t, c.Bind().Req(&r).Err())
	require.Equal(t, 1, r.Key.V)

	require.NoError(t, c.Bind().Req(&r).Err())
	require.Equal(t, 1, r.Key.V)
}

// go test -run Test_Bind_TextUnmarshaler -v
func Test_Bind_TextUnmarshaler(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Req struct {
		Time time.Time `query:"time"`
	}

	now := time.Now()

	c.Request().URI().SetQueryString(url.Values{
		"time": []string{now.Format(time.RFC3339Nano)},
	}.Encode())

	var q Req
	require.NoError(t, c.Bind().Req(&q).Err())
	require.Equal(t, false, q.Time.IsZero(), "time should not be zero")
	require.Equal(t, true, q.Time.Before(now.Add(time.Second)))
	require.Equal(t, true, q.Time.After(now.Add(-time.Second)))
}

// go test -run Test_Bind_error_message -v
func Test_Bind_error_message(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Req struct {
		Time time.Time `query:"time"`
	}

	c.Request().URI().SetQueryString("time=john")

	err := c.Bind().Req(&Req{}).Err()

	require.Error(t, err)
	require.Regexp(t, regexp.MustCompile(`unable to decode 'john' as time`), err.Error())
}

func Test_Bind_Form(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	c.Context().Request.Header.Set(HeaderContentType, MIMEApplicationForm)
	c.Context().Request.SetBody([]byte(url.Values{
		"username": {"u"},
		"password": {"p"},
		"likes":    {"apple", "banana"},
	}.Encode()))

	type Req struct {
		Username string   `form:"username"`
		Password string   `form:"password"`
		Likes    []string `form:"likes"`
	}

	var r Req
	err := c.Bind().Form(&r).Err()

	require.NoError(t, err)
	require.Equal(t, "u", r.Username)
	require.Equal(t, "p", r.Password)
	require.Equal(t, []string{"apple", "banana"}, r.Likes)
}

func Test_Bind_Multipart(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	buf := bytes.NewBuffer(nil)
	boundary := multipart.NewWriter(nil).Boundary()
	err := fasthttp.WriteMultipartForm(buf, &multipart.Form{
		Value: map[string][]string{
			"username": {"u"},
			"password": {"p"},
			"likes":    {"apple", "banana"},
		},
	}, boundary)

	require.NoError(t, err)

	c.Context().Request.Header.Set(HeaderContentType, fmt.Sprintf("%s; boundary=%s", MIMEMultipartForm, boundary))
	c.Context().Request.SetBody(buf.Bytes())

	type Req struct {
		Username string   `multipart:"username"`
		Password string   `multipart:"password"`
		Likes    []string `multipart:"likes"`
	}

	var r Req
	err = c.Bind().Multipart(&r).Err()
	require.NoError(t, err)

	require.Equal(t, "u", r.Username)
	require.Equal(t, "p", r.Password)
	require.Equal(t, []string{"apple", "banana"}, r.Likes)
}

type Req struct {
	ID string `params:"id"`

	I int `query:"I"`
	J int `query:"j"`
	K int `query:"k"`

	Token string `header:"x-auth"`
}

func getBenchCtx() Ctx {
	app := New()

	ctx := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
	ctx.values = [maxParams]string{"id string"}
	ctx.route = &Route{Params: []string{"id"}}

	var u = fasthttp.URI{}
	u.SetQueryString("j=1&j=123&k=-1")
	ctx.Request().SetURI(&u)

	ctx.Request().Header.Set("a-auth", "bearer tt")

	return ctx
}

func Benchmark_Bind_by_hand(b *testing.B) {
	ctx := getBenchCtx()
	for i := 0; i < b.N; i++ {
		var req Req
		var err error

		if raw := ctx.Params("id"); raw != "" {
			req.ID = raw
		}

		if raw := ctx.Query("i"); raw != "" {
			req.I, err = strconv.Atoi(raw)
			if err != nil {
				b.Error(err)
				b.FailNow()
			}
		}

		if raw := ctx.Query("j"); raw != "" {
			req.J, err = strconv.Atoi(raw)
			if err != nil {
				b.Error(err)
				b.FailNow()
			}
		}

		if raw := ctx.Query("k"); raw != "" {
			req.K, err = strconv.Atoi(raw)
			if err != nil {
				b.Error(err)
				b.FailNow()
			}
		}

		req.Token = ctx.Get("x-auth")
	}
}

func Benchmark_Bind(b *testing.B) {
	ctx := getBenchCtx()
	for i := 0; i < b.N; i++ {
		var v = Req{}
		err := ctx.Bind().Req(&v).Err()
		if err != nil {
			b.Error(err)
			b.FailNow()
		}
	}
}

func Test_Binder_Float(t *testing.T) {
	t.Parallel()
	app := New()

	ctx := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
	ctx.values = [maxParams]string{"3.14"}
	ctx.route = &Route{Params: []string{"id"}}

	var req struct {
		ID1 float32 `param:"id"`
		ID2 float64 `param:"id"`
	}

	err := ctx.Bind().Req(&req).Err()
	require.NoError(t, err)
	require.Equal(t, float32(3.14), req.ID1)
	require.Equal(t, float64(3.14), req.ID2)
}
