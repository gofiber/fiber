package minify

var filedata struct {
	html             []byte
	htmlWithCss      []byte
	htmlWithJs       []byte
	htmlWithCssAndJs []byte
}

var expectedData struct {
	html             []byte
	htmlWithCss      []byte
	htmlWithJs       []byte
	htmlWithCssAndJs []byte
}
