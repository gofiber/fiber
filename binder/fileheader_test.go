package binder

import (
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_FileHeaders_UnchangedHeaderIsNotWritten pins that a bind writes a request's own *multipart.FileHeader only
// when schema changed it: the header is what c.FormFile and c.MultipartForm return, and an application may be
// reading it from another goroutine while the handler binds again.
func Test_FileHeaders_UnchangedHeaderIsNotWritten(t *testing.T) {
	t.Parallel()
	type upload struct {
		Avatar *multipart.FileHeader `form:"avatar" query:"avatar"`
		Name   string                `form:"name" query:"name"`
	}
	req := newMultipartRequestWith(t, func(mw *multipart.Writer) {
		require.NoError(t, mw.WriteField("name", "john"))
		writeFormFile(t, mw, "avatar", "avatar.txt", "avatar")
	})

	var first upload
	require.NoError(t, (&FormBinding{}).Bind(req, &first))
	require.NotNil(t, first.Avatar)
	header := reflect.ValueOf(first.Avatar.Header).Pointer()

	var second upload
	require.NoError(t, (&FormBinding{}).Bind(req, &second))
	require.Same(t, first.Avatar, second.Avatar)
	require.NoError(t, (&QueryBinding{}).Bind(req, &first))
	require.Equal(t, header, reflect.ValueOf(first.Avatar.Header).Pointer(), "a header schema did not change is not written back")

	// A header schema did change is put back as it was.
	req.URI().SetQueryString("avatar.filename=x&avatar.size=-1&avatar.header.X-Extra=y")
	require.NoError(t, (&QueryBinding{}).Bind(req, &first))
	requireFile(t, first.Avatar, "avatar.txt", "avatar")
	require.Equal(t, int64(len("avatar")), first.Avatar.Size)
	require.Empty(t, first.Avatar.Header.Get("X-Extra"))
}

// Test_FileHeaders_EarlierBind pins that a bind holds none of the files an earlier bind of the same type decoded.
func Test_FileHeaders_EarlierBind(t *testing.T) {
	t.Parallel()
	type upload struct {
		Avatar *multipart.FileHeader   `form:"avatar"`
		Docs   []*multipart.FileHeader `form:"docs"`
	}
	req := newMultipartRequestWith(t, func(mw *multipart.Writer) {
		writeFormFile(t, mw, "avatar", "avatar.txt", "avatar")
		writeFormFile(t, mw, "docs", "a.txt", "a")
		writeFormFile(t, mw, "docs", "b.txt", "b")
	})
	var first upload
	require.NoError(t, (&FormBinding{}).Bind(req, &first))
	require.Len(t, first.Docs, 2)
	// A few rounds, since a pooled value may be dropped instead of handed out again.
	for range 3 {
		req := newMultipartRequestWith(t, func(mw *multipart.Writer) { writeFormFile(t, mw, "docs", "c.txt", "c") })
		var out upload
		require.NoError(t, (&FormBinding{}).Bind(req, &out))
		require.Nil(t, out.Avatar)
		require.Len(t, out.Docs, 1)
		requireFile(t, out.Docs[0], "c.txt", "c")
	}
}

// Test_HeaderFields pins the fields a walk visits: the ones schema can fill, an embedded struct of an unexported
// type included, through which a *multipart.FileHeader can be reached.
func Test_HeaderFields(t *testing.T) {
	t.Parallel()
	type base struct{ Avatar *multipart.FileHeader }
	type node struct {
		Next *node
		File *multipart.FileHeader
	}
	type mixed struct {
		Meta   map[string]string
		Node   *node
		Docs   map[string][]*multipart.FileHeader
		hidden *multipart.FileHeader
		Avatar *multipart.FileHeader
		base
		Name    string
		Avatars []*multipart.FileHeader
	}
	require.Nil(t, mixed{}.hidden) // hidden is there to be skipped; reading it keeps the linter from calling it unused
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		want []int
	}{
		{"mixed", reflect.TypeFor[mixed](), []int{1, 2, 4, 5, 7}},
		{"a type that reaches itself", reflect.TypeFor[node](), []int{0, 1}},
		{"no header", reflect.TypeFor[struct{ Name string }](), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, headerFields(tc.typ))
			require.Equal(t, tc.want, headerFields(tc.typ), "the cached answer is the same")
		})
	}
}

// Test_GuardFileHeaders_NothingToRecord pins that a guard with no file part and no header held yet allocates the
// guard alone: the record of the headers is made when there is one to put in it.
// Sequential: testing.AllocsPerRun panics during a parallel test.
func Test_GuardFileHeaders_NothingToRecord(t *testing.T) {
	var out struct{ Avatar *multipart.FileHeader }
	require.LessOrEqual(t, testing.AllocsPerRun(100, func() { guardFileHeaders(&out, nil).restore(&out) }), 1.0)
}

// Test_FileHeaders_Save pins that a header is recorded once, as it was first seen, and that a nil one is skipped.
func Test_FileHeaders_Save(t *testing.T) {
	t.Parallel()
	g := &fileHeaders{}
	g.save(nil)
	require.Nil(t, g.saved, "a nil header records nothing")
	fh := &multipart.FileHeader{Filename: "a.txt", Size: 1}
	g.save(fh)
	fh.Filename = "b.txt"
	g.save(fh)
	require.Len(t, g.saved, 1)
	require.Equal(t, "a.txt", g.saved[fh].Filename, "a header already recorded keeps its first record")

	// The same header twice among the file parts, and a nil one, make one record.
	twice := &multipart.FileHeader{Filename: "c.txt"}
	var out struct{ File *multipart.FileHeader }
	g = guardFileHeaders(&out, []map[string][]*multipart.FileHeader{{"file": {nil, twice, twice}}})
	require.NotNil(t, g)
	require.Len(t, g.saved, 1)
	require.True(t, g.known(twice))
}

// Test_FileHeaders_Shapes pins that the guard reaches a *multipart.FileHeader held in every shape schema can fill (a
// field, an element of a slice or an array, a value of a map), puts back one that was changed, clears one that was
// created, and ends at a pointer it has walked.
func Test_FileHeaders_Shapes(t *testing.T) {
	t.Parallel()
	type shapes struct {
		Next    *shapes
		Single  *multipart.FileHeader
		ByName  map[string]*multipart.FileHeader
		Grouped map[string][]*multipart.FileHeader
		Fixed   [2]*multipart.FileHeader
		List    []*multipart.FileHeader
	}
	header := func(name string) *multipart.FileHeader { return &multipart.FileHeader{Filename: name} }
	held := map[string]*multipart.FileHeader{
		"single": header("single"), "listed": header("listed"), "fixed": header("fixed"),
		"by name": header("by name"), "grouped": header("grouped"),
	}
	out := &shapes{
		Single: held["single"], List: []*multipart.FileHeader{held["listed"]},
		Fixed:   [2]*multipart.FileHeader{held["fixed"], nil},
		ByName:  map[string]*multipart.FileHeader{"k": held["by name"]},
		Grouped: map[string][]*multipart.FileHeader{"k": {held["grouped"]}},
	}
	out.Next = out // a pointer to the struct that holds it: the walk must end
	g := guardFileHeaders(out, nil)
	require.NotNil(t, g)
	for name, fh := range held {
		require.True(t, g.known(fh), name)
		fh.Filename = "changed" // what a decode could do to it
	}
	out.Fixed[1] = header("created in an array")
	out.List = append(out.List, header("created in a slice"))

	g.restore(out)
	for name, fh := range held {
		require.Equal(t, name, fh.Filename)
	}
	require.Nil(t, out.Fixed[1], "a header created in an array is cleared")
	require.Len(t, out.List, 2)
	require.Nil(t, out.List[1], "a header created in a slice is cleared")

	var created struct{ Avatar *multipart.FileHeader }
	g = guardFileHeaders(&created, nil)
	created.Avatar = header("created")
	g.restore(&created)
	require.Nil(t, created.Avatar, "a header created in a field is cleared")
}

// Test_FileHeaders_UnexportedEmbedded pins that the guard passes by an element of an unexported embedded slice or
// map: reflect does not let it read one as an interface, and schema does not set one.
func Test_FileHeaders_UnexportedEmbedded(t *testing.T) {
	t.Parallel()
	type hiddenList []*multipart.FileHeader
	type hiddenMap map[string]*multipart.FileHeader
	fh := &multipart.FileHeader{Filename: "a.txt"}
	out := &struct {
		hiddenMap
		hiddenList
	}{hiddenMap{"k": fh}, hiddenList{fh}}
	var g *fileHeaders
	require.NotPanics(t, func() { g = guardFileHeaders(out, nil) })
	require.NotNil(t, g)
	require.False(t, g.known(fh))
	require.NotPanics(t, func() { g.restore(out) })
	require.Equal(t, "a.txt", fh.Filename)
}

// Test_FileHeaders_Cycles pins that the walk ends in a destination that holds itself through a slice or a map.
func Test_FileHeaders_Cycles(t *testing.T) {
	t.Parallel()
	type node struct {
		File     *multipart.FileHeader
		ByName   map[string]node
		Children []node
	}
	fh := &multipart.FileHeader{Filename: "a.txt"}
	nodes := []node{{File: fh}}
	nodes[0].Children = nodes
	byName := map[string]node{}
	byName["k"] = node{File: fh, ByName: byName}
	for name, out := range map[string]any{"slice": &nodes, "map": &byName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := guardFileHeaders(out, nil)
			require.NotNil(t, g)
			require.True(t, g.known(fh))
			g.restore(out)
		})
	}
}

// Test_FileHeaders_RestoresAReplacedHeader pins that a header the decode wrote to and then replaced in the destination
// is put back too: schema fills a field from the file parts of keys that differ in case, in the order of a map.
func Test_FileHeaders_RestoresAReplacedHeader(t *testing.T) {
	t.Parallel()
	first := &multipart.FileHeader{Filename: "first.txt", Size: 1}
	second := &multipart.FileHeader{Filename: "second.txt", Size: 2}
	var out struct{ Avatar *multipart.FileHeader }
	g := guardFileHeaders(&out, []map[string][]*multipart.FileHeader{{"avatar": {first}, "AVATAR": {second}}})
	require.NotNil(t, g)
	out.Avatar = first
	first.Filename = "changed.txt" // a form value written into the header the field holds
	out.Avatar = second            // and the field is filled again from the other file part

	g.restore(&out)
	require.Equal(t, "first.txt", first.Filename)
	require.Equal(t, "second.txt", second.Filename)
	require.Same(t, second, out.Avatar)
}

// Test_FileHeaders_AliasOfAnUnexportedEmbedded pins that a slice or a map the walk meets through an unexported
// embedded field, whose headers it cannot reach, does not hide the same one held by an exported field.
func Test_FileHeaders_AliasOfAnUnexportedEmbedded(t *testing.T) {
	t.Parallel()
	type node struct{ File *multipart.FileHeader }
	type hiddenNodes []node
	type hiddenByName map[string]node
	inSlice, inMap := &multipart.FileHeader{Filename: "slice.txt"}, &multipart.FileHeader{Filename: "map.txt"}
	nodes, byName := hiddenNodes{{File: inSlice}}, hiddenByName{"k": {File: inMap}}
	out := &struct {
		hiddenByName
		VisibleMap hiddenByName
		hiddenNodes
		Visible hiddenNodes
	}{byName, byName, nodes, nodes}
	g := guardFileHeaders(out, nil)
	require.NotNil(t, g)
	require.True(t, g.known(inSlice), "the header is reachable through the exported slice")
	require.True(t, g.known(inMap), "the header is reachable through the exported map")
}

// benchUpload is benchUser with a file to bind, which makes a bind walk its fields.
type benchUpload struct {
	Avatar *multipart.FileHeader `form:"avatar" query:"avatar"`
	Name   string                `form:"name" query:"name"`
	Posts  []string              `form:"posts" query:"posts"`
	Age    int                   `form:"age" query:"age"`
}

func Benchmark_QueryBinder_Bind_FileHeader(b *testing.B) {
	b.ReportAllocs()

	binder := &QueryBinding{EnableSplitting: true}

	var user benchUpload

	req := newQueryRequest(b, "name=john&age=42&posts=post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}

func Benchmark_FormBinder_BindMultipart_FileHeader(b *testing.B) {
	b.ReportAllocs()

	binder := &FormBinding{EnableSplitting: true}

	var user benchUpload

	req := newMultipartRequestWith(b, func(mw *multipart.Writer) {
		require.NoError(b, mw.WriteField("name", "john"))
		require.NoError(b, mw.WriteField("age", "42"))
		require.NoError(b, mw.WriteField("posts", "post1"))
		require.NoError(b, mw.WriteField("posts", "post2"))
		require.NoError(b, mw.WriteField("posts", "post3"))
		writeFormFile(b, mw, "avatar", "avatar.txt", "avatar")
	})

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
	require.NotNil(b, user.Avatar)
}
