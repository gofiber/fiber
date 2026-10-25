package client

import (
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/internal/crosshost"
)

// redirectOptions is passed by pointer: by value it widens the frames of every transport.
type redirectOptions struct {
	sensitiveHeaders crosshost.HeaderNames
	maxRedirects     int
}

// requestBackup is the request as sent; a redirect chain rewrites it, so a retry restarts from the backup.
type requestBackup struct{ original *fasthttp.Request }

// keep skips body streams, which cannot be rewound.
func (b *requestBackup) keep(req *fasthttp.Request) {
	if req.IsBodyStream() {
		return
	}
	b.original = fasthttp.AcquireRequest()
	req.CopyTo(b.original)
}

func (b *requestBackup) restore(req *fasthttp.Request) {
	if b.original != nil {
		b.original.CopyTo(req)
	}
}

func (b *requestBackup) release() {
	if b.original != nil {
		fasthttp.ReleaseRequest(b.original)
		b.original = nil
	}
}

// redirectPlan is nil for a request that follows no redirects; every method accepts nil.
type redirectPlan struct {
	backup requestBackup
	opts   redirectOptions
}

func (p *redirectPlan) keep(req *fasthttp.Request) {
	if p != nil {
		p.backup.keep(req)
	}
}

func (p *redirectPlan) send(client *Client, req *fasthttp.Request, resp *fasthttp.Response) error {
	if p == nil {
		return client.Do(req, resp)
	}
	err := client.transport.DoRedirects(req, resp, &p.opts)
	if err != nil {
		p.backup.restore(req)
	}
	return err
}

func (p *redirectPlan) release() {
	if p != nil {
		p.backup.release()
	}
}
