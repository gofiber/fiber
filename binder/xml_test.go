package binder

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/require"
)

// xmlUserDoc is the document the XML binder test and benchmark bind.
const xmlUserDoc = `
		<user>
			<name>john</name>
			<age>42</age>
			<ignore>ignore</ignore>
			<posts>
				<post>
					<title>post1</title>
				</post>
				<post>
					<title>post2</title>
				</post>
			</posts>
		</user>
	`

func Test_XMLBinding_Bind(t *testing.T) {
	t.Parallel()
	b := &XMLBinding{XMLDecoder: xml.Unmarshal}
	require.Equal(t, "xml", b.Name())

	type Posts struct {
		XMLName xml.Name `xml:"post"`
		Title   string   `xml:"title"`
	}

	type User struct {
		Name   string  `xml:"name"`
		Ignore string  `xml:"-"`
		Posts  []Posts `xml:"posts>post"`
		Age    int     `xml:"age"`
	}

	user := new(User)
	require.NoError(t, b.Bind([]byte(xmlUserDoc), user))
	post := xml.Name{Local: "post"}
	posts := []Posts{{XMLName: post, Title: "post1"}, {XMLName: post, Title: "post2"}}
	require.Equal(t, User{Name: "john", Posts: posts, Age: 42}, *user)

	b.Reset()
	require.Nil(t, b.XMLDecoder)
}

func Test_XMLBinding_Bind_error(t *testing.T) {
	t.Parallel()
	b := &XMLBinding{XMLDecoder: xml.Unmarshal}

	type User struct {
		Name string `xml:"name"`
		Age  int    `xml:"age"`
	}

	user := new(User)
	err := b.Bind([]byte(`
		<user>
			<name>john</name>
			<age>42</age>
			<unknown>unknown</unknown>
		</user
	`), user)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal xml")
}

func Benchmark_XMLBinding_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &XMLBinding{XMLDecoder: xml.Unmarshal}

	type Posts struct {
		XMLName xml.Name `xml:"post"`
		Title   string   `xml:"title"`
	}

	type User struct {
		Name  string  `xml:"name"`
		Posts []Posts `xml:"posts>post"`
		Age   int     `xml:"age"`
	}

	user := new(User)
	data := []byte(xmlUserDoc)

	var err error
	for b.Loop() {
		err = binder.Bind(data, user)
	}
	require.NoError(b, err)

	user = new(User)
	err = binder.Bind(data, user)
	require.NoError(b, err)

	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)

	require.Len(b, user.Posts, 2)
	require.Equal(b, "post1", user.Posts[0].Title)
	require.Equal(b, "post2", user.Posts[1].Title)
}
