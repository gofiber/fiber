package client

import (
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/internal/crosshost"
)

// redirectOptions is how a chain of redirects is followed. Transports take it by pointer: it is only read,
// and by value it widens the frames of every transport and of the redirect loop.
type redirectOptions struct {
	sensitiveHeaders crosshost.HeaderNames // dropped with the built-in set once a hop leaves the initial host
	maxRedirects     int
}

// requestBackup is a request as it was sent. A redirect chain rewrites the request it is given, so a retry
// restarts from the backup instead of from the hop the failed attempt reached.
type requestBackup struct{ original *fasthttp.Request }

// keep copies req, unless its body is a stream, which cannot be rewound.
func (b *requestBackup) keep(req *fasthttp.Request) {
	if req.IsBodyStream() {
		return
	}
	b.original = fasthttp.AcquireRequest()
	req.CopyTo(b.original)
}

// restore puts req back as it was kept. Without a copy it leaves req alone.
func (b *requestBackup) restore(req *fasthttp.Request) {
	if b.original != nil {
		b.original.CopyTo(req)
	}
}

// release returns the copy to the pool.
func (b *requestBackup) release() {
	if b.original != nil {
		fasthttp.ReleaseRequest(b.original)
		b.original = nil
	}
}

// redirectPlan is what a request that follows redirects needs once its Request may have been recycled.
// It is one object, built only for such requests, so the goroutine that carries it stays small. A nil
// plan is a request that follows none, and every method accepts it.
type redirectPlan struct {
	backup requestBackup
	opts   redirectOptions
}

// keep backs req up for a retry to restart from.
func (p *redirectPlan) keep(req *fasthttp.Request) {
	if p != nil {
		p.backup.keep(req)
	}
}

// send sends req and follows its redirects. If that fails, req is put back as it was kept.
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

// release returns the backup to the pool.
func (p *redirectPlan) release() {
	if p != nil {
		p.backup.release()
	}
}
