package client

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/internal/crosshost"
)

// followTransport records whether it was asked to follow redirects.
type followTransport struct {
	*delayedTransport

	followed bool
}

func (f *followTransport) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, opts *redirectOptions) error {
	f.followed = true
	return f.delayedTransport.DoRedirects(req, resp, opts)
}

// sentRequest is a request with a header and a body, as a caller sends one.
func sentRequest(t *testing.T) *fasthttp.Request {
	t.Helper()

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	req.SetRequestURI("http://example.com/start")
	req.Header.Set("X-Api-Key", "key")
	req.SetBodyString("payload")
	return req
}

// rewrite does to req what a redirect chain that failed part-way would leave behind.
func rewrite(req *fasthttp.Request) {
	req.SetRequestURI("http://attacker.example/collect")
	req.Header.Del("X-Api-Key")
	req.ResetBody()
}

func Test_RequestBackup(t *testing.T) {
	t.Parallel()

	t.Run("restores the request as it was kept", func(t *testing.T) {
		t.Parallel()

		req := sentRequest(t)
		var backup requestBackup
		backup.keep(req)
		defer backup.release()

		rewrite(req)
		backup.restore(req)

		require.Equal(t, "http://example.com/start", req.URI().String())
		require.Equal(t, "key", string(req.Header.Peek("X-Api-Key")))
		require.Equal(t, "payload", string(req.Body()))
	})

	t.Run("a stream is not kept, so restoring leaves the request alone", func(t *testing.T) {
		t.Parallel()

		req := sentRequest(t)
		req.SetBodyStream(bytes.NewBufferString("payload"), len("payload"))
		var backup requestBackup
		backup.keep(req)
		require.Nil(t, backup.original)

		rewrite(req)
		backup.restore(req)
		require.Equal(t, "http://attacker.example/collect", req.URI().String())
		require.Empty(t, req.Header.Peek("X-Api-Key"))
	})

	t.Run("release gives the copy back once", func(t *testing.T) {
		t.Parallel()

		var backup requestBackup
		backup.keep(sentRequest(t))
		require.NotNil(t, backup.original)

		backup.release()
		require.Nil(t, backup.original)
		backup.release()
	})
}

func Test_RedirectPlan(t *testing.T) {
	t.Parallel()

	t.Run("a nil plan sends the request without following redirects", func(t *testing.T) {
		t.Parallel()

		transport := &followTransport{delayedTransport: newDelayedTransport(0)}
		client := New()
		client.transport = transport

		var plan *redirectPlan
		req := sentRequest(t)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		plan.keep(req)
		defer plan.release()
		require.NoError(t, plan.send(client, req, resp))
		require.False(t, transport.followed)
		require.Equal(t, fasthttp.StatusOK, resp.StatusCode())
	})

	t.Run("a plan hands its options to the transport and puts back a request a failed chain rewrote", func(t *testing.T) {
		t.Parallel()

		transport := &rewritingTransport{delayedTransport: newDelayedTransport(0)}
		client := New()
		client.transport = transport

		plan := &redirectPlan{opts: redirectOptions{maxRedirects: 3, sensitiveHeaders: crosshost.NewHeaderNames([]string{"X-Api-Key"})}}
		req := sentRequest(t)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		plan.keep(req)
		defer plan.release()
		require.Error(t, plan.send(client, req, resp))

		require.Equal(t, [][]string{{"X-Api-Key"}}, transport.names)
		require.Equal(t, "http://example.com/start", req.URI().String())
		require.Equal(t, "key", string(req.Header.Peek("X-Api-Key")))
	})

	t.Run("without a backup a failed chain leaves the request where it got to", func(t *testing.T) {
		t.Parallel()

		transport := &rewritingTransport{delayedTransport: newDelayedTransport(0)}
		client := New()
		client.transport = transport

		plan := &redirectPlan{}
		req := sentRequest(t)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		require.Error(t, plan.send(client, req, resp))
		require.Equal(t, "http://attacker.example/collect", req.URI().String())
	})
}
