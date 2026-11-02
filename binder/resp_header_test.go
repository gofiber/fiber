package binder

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func Benchmark_RespHeaderBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &RespHeaderBinding{EnableSplitting: true}

	var user benchUser

	resp := fasthttp.AcquireResponse()
	resp.Header.Set("name", "john")
	resp.Header.Set("age", "42")
	resp.Header.Set("posts", "post1,post2,post3")

	b.Cleanup(func() { fasthttp.ReleaseResponse(resp) })

	var err error
	for b.Loop() {
		err = binder.Bind(resp, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}
