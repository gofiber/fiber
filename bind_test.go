package fiber

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Bind_Query -v
func Test_Bind_Query(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	q := new(Query)
	utils.AssertEqual(t, nil, c.Binding().Query(q))
	utils.AssertEqual(t, 2, len(q.Hobby))

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football")
	q = new(Query)
	utils.AssertEqual(t, nil, c.Binding().Query(q))
	utils.AssertEqual(t, 2, len(q.Hobby))

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=scoccer&hobby=basketball,football")
	q = new(Query)
	utils.AssertEqual(t, nil, c.Binding().Query(q))
	utils.AssertEqual(t, 3, len(q.Hobby))

	empty := new(Query)
	c.Request().URI().SetQueryString("")
	utils.AssertEqual(t, nil, c.Binding().Query(empty))
	utils.AssertEqual(t, 0, len(empty.Hobby))

	type Query2 struct {
		Bool            bool
		ID              int
		Name            string
		Hobby           string
		FavouriteDrinks []string
		Empty           []string
		Alloc           []string
		No              []int64
	}

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1")
	q2 := new(Query2)
	q2.Bool = true
	q2.Name = "hello world"
	utils.AssertEqual(t, nil, c.Binding().Query(q2))
	utils.AssertEqual(t, "basketball,football", q2.Hobby)
	utils.AssertEqual(t, true, q2.Bool)
	utils.AssertEqual(t, "tom", q2.Name) // check value get overwritten
	utils.AssertEqual(t, []string{"milo", "coke", "pepsi"}, q2.FavouriteDrinks)
	var nilSlice []string
	utils.AssertEqual(t, nilSlice, q2.Empty)
	utils.AssertEqual(t, []string{""}, q2.Alloc)
	utils.AssertEqual(t, []int64{1}, q2.No)

	type RequiredQuery struct {
		Name string `query:"name,required"`
	}
	rq := new(RequiredQuery)
	c.Request().URI().SetQueryString("")
	utils.AssertEqual(t, "name is empty", c.Binding().Query(rq).Error())

	type ArrayQuery struct {
		Data []string
	}
	aq := new(ArrayQuery)
	c.Request().URI().SetQueryString("data[]=john&data[]=doe")
	utils.AssertEqual(t, nil, c.Binding().Query(aq))
	utils.AssertEqual(t, 2, len(aq.Data))
}

// go test -run Test_Bind_Query_WithSetParserDecoder -v
func Test_Bind_Query_WithSetParserDecoder(t *testing.T) {
	type NonRFCTime time.Time

	NonRFCConverter := func(value string) reflect.Value {
		if v, err := time.Parse("2006-01-02", value); err == nil {
			return reflect.ValueOf(v)
		}
		return reflect.Value{}
	}

	nonRFCTime := binder.ParserType{
		Customtype: NonRFCTime{},
		Converter:  NonRFCConverter,
	}

	binder.SetParserDecoder(binder.ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType:        []binder.ParserType{nonRFCTime},
		ZeroEmpty:         true,
		SetAliasTag:       "query",
	})

	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type NonRFCTimeInput struct {
		Date  NonRFCTime `query:"date"`
		Title string     `query:"title"`
		Body  string     `query:"body"`
	}

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	q := new(NonRFCTimeInput)

	c.Request().URI().SetQueryString("date=2021-04-10&title=CustomDateTest&Body=October")
	utils.AssertEqual(t, nil, c.Binding().Query(q))
	fmt.Println(q.Date, "q.Date")
	utils.AssertEqual(t, "CustomDateTest", q.Title)
	date := fmt.Sprintf("%v", q.Date)
	utils.AssertEqual(t, "{0 63753609600 <nil>}", date)
	utils.AssertEqual(t, "October", q.Body)

	c.Request().URI().SetQueryString("date=2021-04-10&title&Body=October")
	q = &NonRFCTimeInput{
		Title: "Existing title",
		Body:  "Existing Body",
	}
	utils.AssertEqual(t, nil, c.Binding().Query(q))
	utils.AssertEqual(t, "", q.Title)
}

// go test -run Test_Bind_Query_Schema -v
func Test_Bind_Query_Schema(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Query1 struct {
		Name   string `query:"name,required"`
		Nested struct {
			Age int `query:"age"`
		} `query:"nested,required"`
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("name=tom&nested.age=10")
	q := new(Query1)
	utils.AssertEqual(t, nil, c.Binding().Query(q))

	c.Request().URI().SetQueryString("namex=tom&nested.age=10")
	q = new(Query1)
	utils.AssertEqual(t, "name is empty", c.Binding().Query(q).Error())

	c.Request().URI().SetQueryString("name=tom&nested.agex=10")
	q = new(Query1)
	utils.AssertEqual(t, nil, c.Binding().Query(q))

	c.Request().URI().SetQueryString("name=tom&test.age=10")
	q = new(Query1)
	utils.AssertEqual(t, "nested is empty", c.Binding().Query(q).Error())

	type Query2 struct {
		Name   string `query:"name"`
		Nested struct {
			Age int `query:"age,required"`
		} `query:"nested"`
	}
	c.Request().URI().SetQueryString("name=tom&nested.age=10")
	q2 := new(Query2)
	utils.AssertEqual(t, nil, c.Binding().Query(q2))

	c.Request().URI().SetQueryString("nested.age=10")
	q2 = new(Query2)
	utils.AssertEqual(t, nil, c.Binding().Query(q2))

	c.Request().URI().SetQueryString("nested.agex=10")
	q2 = new(Query2)
	utils.AssertEqual(t, "nested.age is empty", c.Binding().Query(q2).Error())

	c.Request().URI().SetQueryString("nested.agex=10")
	q2 = new(Query2)
	utils.AssertEqual(t, "nested.age is empty", c.Binding().Query(q2).Error())

	type Node struct {
		Value int   `query:"val,required"`
		Next  *Node `query:"next,required"`
	}
	c.Request().URI().SetQueryString("val=1&next.val=3")
	n := new(Node)
	utils.AssertEqual(t, nil, c.Binding().Query(n))
	utils.AssertEqual(t, 1, n.Value)
	utils.AssertEqual(t, 3, n.Next.Value)

	c.Request().URI().SetQueryString("next.val=2")
	n = new(Node)
	utils.AssertEqual(t, "val is empty", c.Binding().Query(n).Error())

	c.Request().URI().SetQueryString("val=3&next.value=2")
	n = new(Node)
	n.Next = new(Node)
	utils.AssertEqual(t, nil, c.Binding().Query(n))
	utils.AssertEqual(t, 3, n.Value)
	utils.AssertEqual(t, 0, n.Next.Value)

	type Person struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	type CollectionQuery struct {
		Data []Person `query:"data"`
	}

	c.Request().URI().SetQueryString("data[0][name]=john&data[0][age]=10&data[1][name]=doe&data[1][age]=12")
	cq := new(CollectionQuery)
	utils.AssertEqual(t, nil, c.Binding().Query(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, 10, cq.Data[0].Age)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)
	utils.AssertEqual(t, 12, cq.Data[1].Age)

	c.Request().URI().SetQueryString("data.0.name=john&data.0.age=10&data.1.name=doe&data.1.age=12")
	cq = new(CollectionQuery)
	utils.AssertEqual(t, nil, c.Binding().Query(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, 10, cq.Data[0].Age)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)
	utils.AssertEqual(t, 12, cq.Data[1].Age)
}

// go test -run Test_Bind_Header -v
func Test_Bind_Header(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Header struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	c.Request().Header.Add("id", "1")
	c.Request().Header.Add("Name", "John Doe")
	c.Request().Header.Add("Hobby", "golang,fiber")
	q := new(Header)
	utils.AssertEqual(t, nil, c.Binding().Header(q))
	utils.AssertEqual(t, 2, len(q.Hobby))

	c.Request().Header.Del("hobby")
	c.Request().Header.Add("Hobby", "golang,fiber,go")
	q = new(Header)
	utils.AssertEqual(t, nil, c.Binding().Header(q))
	utils.AssertEqual(t, 3, len(q.Hobby))

	empty := new(Header)
	c.Request().Header.Del("hobby")
	utils.AssertEqual(t, nil, c.Binding().Query(empty))
	utils.AssertEqual(t, 0, len(empty.Hobby))

	type Header2 struct {
		Bool            bool
		ID              int
		Name            string
		Hobby           string
		FavouriteDrinks []string
		Empty           []string
		Alloc           []string
		No              []int64
	}

	c.Request().Header.Add("id", "2")
	c.Request().Header.Add("Name", "Jane Doe")
	c.Request().Header.Del("hobby")
	c.Request().Header.Add("Hobby", "go,fiber")
	c.Request().Header.Add("favouriteDrinks", "milo,coke,pepsi")
	c.Request().Header.Add("alloc", "")
	c.Request().Header.Add("no", "1")

	h2 := new(Header2)
	h2.Bool = true
	h2.Name = "hello world"
	utils.AssertEqual(t, nil, c.Binding().Header(h2))
	utils.AssertEqual(t, "go,fiber", h2.Hobby)
	utils.AssertEqual(t, true, h2.Bool)
	utils.AssertEqual(t, "Jane Doe", h2.Name) // check value get overwritten
	utils.AssertEqual(t, []string{"milo", "coke", "pepsi"}, h2.FavouriteDrinks)
	var nilSlice []string
	utils.AssertEqual(t, nilSlice, h2.Empty)
	utils.AssertEqual(t, []string{""}, h2.Alloc)
	utils.AssertEqual(t, []int64{1}, h2.No)

	type RequiredHeader struct {
		Name string `header:"name,required"`
	}
	rh := new(RequiredHeader)
	c.Request().Header.Del("name")
	utils.AssertEqual(t, "name is empty", c.Binding().Header(rh).Error())
}

// go test -run Test_Bind_Header_WithSetParserDecoder -v
func Test_Bind_Header_WithSetParserDecoder(t *testing.T) {
	type NonRFCTime time.Time

	NonRFCConverter := func(value string) reflect.Value {
		if v, err := time.Parse("2006-01-02", value); err == nil {
			return reflect.ValueOf(v)
		}
		return reflect.Value{}
	}

	nonRFCTime := binder.ParserType{
		Customtype: NonRFCTime{},
		Converter:  NonRFCConverter,
	}

	binder.SetParserDecoder(binder.ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType:        []binder.ParserType{nonRFCTime},
		ZeroEmpty:         true,
		SetAliasTag:       "req",
	})

	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type NonRFCTimeInput struct {
		Date  NonRFCTime `req:"date"`
		Title string     `req:"title"`
		Body  string     `req:"body"`
	}

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	r := new(NonRFCTimeInput)

	c.Request().Header.Add("Date", "2021-04-10")
	c.Request().Header.Add("Title", "CustomDateTest")
	c.Request().Header.Add("Body", "October")

	utils.AssertEqual(t, nil, c.Binding().Header(r))
	fmt.Println(r.Date, "q.Date")
	utils.AssertEqual(t, "CustomDateTest", r.Title)
	date := fmt.Sprintf("%v", r.Date)
	utils.AssertEqual(t, "{0 63753609600 <nil>}", date)
	utils.AssertEqual(t, "October", r.Body)

	c.Request().Header.Add("Title", "")
	r = &NonRFCTimeInput{
		Title: "Existing title",
		Body:  "Existing Body",
	}
	utils.AssertEqual(t, nil, c.Binding().Header(r))
	utils.AssertEqual(t, "", r.Title)
}

// go test -run Test_Bind_Header_Schema -v
func Test_Bind_Header_Schema(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Header1 struct {
		Name   string `header:"Name,required"`
		Nested struct {
			Age int `header:"Age"`
		} `header:"Nested,required"`
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	c.Request().Header.Add("Name", "tom")
	c.Request().Header.Add("Nested.Age", "10")
	q := new(Header1)
	utils.AssertEqual(t, nil, c.Binding().Header(q))

	c.Request().Header.Del("Name")
	q = new(Header1)
	utils.AssertEqual(t, "Name is empty", c.Binding().Header(q).Error())

	c.Request().Header.Add("Name", "tom")
	c.Request().Header.Del("Nested.Age")
	c.Request().Header.Add("Nested.Agex", "10")
	q = new(Header1)
	utils.AssertEqual(t, nil, c.Binding().Header(q))

	c.Request().Header.Del("Nested.Agex")
	q = new(Header1)
	utils.AssertEqual(t, "Nested is empty", c.Binding().Header(q).Error())

	c.Request().Header.Del("Nested.Agex")
	c.Request().Header.Del("Name")

	type Header2 struct {
		Name   string `header:"Name"`
		Nested struct {
			Age int `header:"age,required"`
		} `header:"Nested"`
	}

	c.Request().Header.Add("Name", "tom")
	c.Request().Header.Add("Nested.Age", "10")

	h2 := new(Header2)
	utils.AssertEqual(t, nil, c.Binding().Header(h2))

	c.Request().Header.Del("Name")
	h2 = new(Header2)
	utils.AssertEqual(t, nil, c.Binding().Header(h2))

	c.Request().Header.Del("Name")
	c.Request().Header.Del("Nested.Age")
	c.Request().Header.Add("Nested.Agex", "10")
	h2 = new(Header2)
	utils.AssertEqual(t, "Nested.age is empty", c.Binding().Header(h2).Error())

	type Node struct {
		Value int   `header:"Val,required"`
		Next  *Node `header:"Next,required"`
	}
	c.Request().Header.Add("Val", "1")
	c.Request().Header.Add("Next.Val", "3")
	n := new(Node)
	utils.AssertEqual(t, nil, c.Binding().Header(n))
	utils.AssertEqual(t, 1, n.Value)
	utils.AssertEqual(t, 3, n.Next.Value)

	c.Request().Header.Del("Val")
	n = new(Node)
	utils.AssertEqual(t, "Val is empty", c.Binding().Header(n).Error())

	c.Request().Header.Add("Val", "3")
	c.Request().Header.Del("Next.Val")
	c.Request().Header.Add("Next.Value", "2")
	n = new(Node)
	n.Next = new(Node)
	utils.AssertEqual(t, nil, c.Binding().Header(n))
	utils.AssertEqual(t, 3, n.Value)
	utils.AssertEqual(t, 0, n.Next.Value)
}

// go test -v  -run=^$ -bench=Benchmark_Bind_Query -benchmem -count=4
func Benchmark_Bind_Query(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	q := new(Query)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Binding().Query(q)
	}
	utils.AssertEqual(b, nil, c.Binding().Query(q))
}

// go test -v  -run=^$ -bench=Benchmark_Bind_Query_WithParseParam -benchmem -count=4
func Benchmark_Bind_Query_WithParseParam(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Person struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	type CollectionQuery struct {
		Data []Person `query:"data"`
	}

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("data[0][name]=john&data[0][age]=10")
	cq := new(CollectionQuery)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Binding().Query(cq)
	}

	utils.AssertEqual(b, nil, c.Binding().Query(cq))
}

// go test -v  -run=^$ -bench=Benchmark_Bind_Query_Comma -benchmem -count=4
func Benchmark_Bind_Query_Comma(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	// c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football")
	q := new(Query)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Binding().Query(q)
	}
	utils.AssertEqual(b, nil, c.Binding().Query(q))
}

// go test -v  -run=^$ -bench=Benchmark_Bind_Header -benchmem -count=4
func Benchmark_Bind_Header(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type ReqHeader struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	c.Request().Header.Add("id", "1")
	c.Request().Header.Add("Name", "John Doe")
	c.Request().Header.Add("Hobby", "golang,fiber")

	q := new(ReqHeader)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Binding().Header(q)
	}
	utils.AssertEqual(b, nil, c.Binding().Header(q))
}

// go test -run Test_Bind_Body
func Test_Bind_Body(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Demo struct {
		Name string `json:"name" xml:"name" form:"name" query:"name"`
	}

	{
		var gzipJSON bytes.Buffer
		w := gzip.NewWriter(&gzipJSON)
		_, _ = w.Write([]byte(`{"name":"john"}`))
		_ = w.Close()

		c.Request().Header.SetContentType(MIMEApplicationJSON)
		c.Request().Header.Set(HeaderContentEncoding, "gzip")
		c.Request().SetBody(gzipJSON.Bytes())
		c.Request().Header.SetContentLength(len(gzipJSON.Bytes()))
		d := new(Demo)
		utils.AssertEqual(t, nil, c.Binding().Body(d))
		utils.AssertEqual(t, "john", d.Name)
		c.Request().Header.Del(HeaderContentEncoding)
	}

	testDecodeParser := func(contentType, body string) {
		c.Request().Header.SetContentType(contentType)
		c.Request().SetBody([]byte(body))
		c.Request().Header.SetContentLength(len(body))
		d := new(Demo)
		utils.AssertEqual(t, nil, c.Binding().Body(d))
		utils.AssertEqual(t, "john", d.Name)
	}

	testDecodeParser(MIMEApplicationJSON, `{"name":"john"}`)
	testDecodeParser(MIMEApplicationXML, `<Demo><name>john</name></Demo>`)
	testDecodeParser(MIMEApplicationForm, "name=john")
	testDecodeParser(MIMEMultipartForm+`;boundary="b"`, "--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\njohn\r\n--b--")

	testDecodeParserError := func(contentType, body string) {
		c.Request().Header.SetContentType(contentType)
		c.Request().SetBody([]byte(body))
		c.Request().Header.SetContentLength(len(body))
		utils.AssertEqual(t, false, c.Binding().Body(nil) == nil)
	}

	testDecodeParserError("invalid-content-type", "")
	testDecodeParserError(MIMEMultipartForm+`;boundary="b"`, "--b")

	type CollectionQuery struct {
		Data []Demo `query:"data"`
	}

	c.Request().Reset()
	c.Request().Header.SetContentType(MIMEApplicationForm)
	c.Request().SetBody([]byte("data[0][name]=john&data[1][name]=doe"))
	c.Request().Header.SetContentLength(len(c.Body()))
	cq := new(CollectionQuery)
	utils.AssertEqual(t, nil, c.Binding().Body(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)

	c.Request().Reset()
	c.Request().Header.SetContentType(MIMEApplicationForm)
	c.Request().SetBody([]byte("data.0.name=john&data.1.name=doe"))
	c.Request().Header.SetContentLength(len(c.Body()))
	cq = new(CollectionQuery)
	utils.AssertEqual(t, nil, c.Binding().Body(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)
}

// go test -run Test_Bind_Body_WithSetParserDecoder
func Test_Bind_Body_WithSetParserDecoder(t *testing.T) {
	type CustomTime time.Time

	timeConverter := func(value string) reflect.Value {
		if v, err := time.Parse("2006-01-02", value); err == nil {
			return reflect.ValueOf(v)
		}
		return reflect.Value{}
	}

	customTime := binder.ParserType{
		Customtype: CustomTime{},
		Converter:  timeConverter,
	}

	binder.SetParserDecoder(binder.ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType:        []binder.ParserType{customTime},
		ZeroEmpty:         true,
		SetAliasTag:       "form",
	})

	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Demo struct {
		Date  CustomTime `form:"date"`
		Title string     `form:"title"`
		Body  string     `form:"body"`
	}

	testDecodeParser := func(contentType, body string) {
		c.Request().Header.SetContentType(contentType)
		c.Request().SetBody([]byte(body))
		c.Request().Header.SetContentLength(len(body))
		d := Demo{
			Title: "Existing title",
			Body:  "Existing Body",
		}
		utils.AssertEqual(t, nil, c.Binding().Body(&d))
		date := fmt.Sprintf("%v", d.Date)
		utils.AssertEqual(t, "{0 63743587200 <nil>}", date)
		utils.AssertEqual(t, "", d.Title)
		utils.AssertEqual(t, "New Body", d.Body)
	}

	testDecodeParser(MIMEApplicationForm, "date=2020-12-15&title=&body=New Body")
	testDecodeParser(MIMEMultipartForm+`; boundary="b"`, "--b\r\nContent-Disposition: form-data; name=\"date\"\r\n\r\n2020-12-15\r\n--b\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\n\r\n--b\r\nContent-Disposition: form-data; name=\"body\"\r\n\r\nNew Body\r\n--b--")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_JSON -benchmem -count=4
func Benchmark_Ctx_Body_JSON(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Demo struct {
		Name string `json:"name"`
	}
	body := []byte(`{"name":"john"}`)
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEApplicationJSON)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.Binding().Body(d)
	}
	utils.AssertEqual(b, nil, c.Binding().Body(d))
	utils.AssertEqual(b, "john", d.Name)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_XML -benchmem -count=4
func Benchmark_Ctx_Body_XML(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Demo struct {
		Name string `xml:"name"`
	}
	body := []byte("<Demo><name>john</name></Demo>")
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEApplicationXML)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.Binding().Body(d)
	}
	utils.AssertEqual(b, nil, c.Binding().Body(d))
	utils.AssertEqual(b, "john", d.Name)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_Form -benchmem -count=4
func Benchmark_Ctx_Body_Form(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Demo struct {
		Name string `form:"name"`
	}
	body := []byte("name=john")
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEApplicationForm)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.Binding().Body(d)
	}
	utils.AssertEqual(b, nil, c.Binding().Body(d))
	utils.AssertEqual(b, "john", d.Name)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_MultipartForm -benchmem -count=4
func Benchmark_Ctx_Body_MultipartForm(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Demo struct {
		Name string `form:"name"`
	}

	body := []byte("--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\njohn\r\n--b--")
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEMultipartForm + `;boundary="b"`)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.Binding().Body(d)
	}
	utils.AssertEqual(b, nil, c.Binding().Body(d))
	utils.AssertEqual(b, "john", d.Name)
}
