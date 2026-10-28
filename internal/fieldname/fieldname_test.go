package fieldname

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// readRequestHeader parses raw as a request, which is the only way to build the
// two-line shapes below: they are what a peer or an intermediary puts on the
// wire, not something the setters express directly.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func readRequestHeader(tb testing.TB, raw string, canonical bool) *fasthttp.RequestHeader {
	tb.Helper()

	h := &fasthttp.RequestHeader{}
	if !canonical {
		h.DisableNormalizing()
	}
	require.NoError(tb, h.Read(bufio.NewReader(bytes.NewBufferString(raw))))
	return h
}

// Test_First_EmptyLineIsNotNil pins the fasthttp behavior First's fast path
// rests on: Peek answers nil for a field that is absent and a non-nil empty
// slice for one that is present and empty. First skips its second walk on the
// nil answer, so were the two to become indistinguishable it would read only
// the first line again — silently, and only for the messages this guards
// against. This test is what makes that loud instead.
func Test_First_EmptyLineIsNotNil(t *testing.T) {
	t.Parallel()

	h := readRequestHeader(t, "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\n\r\n", true)

	require.Nil(t, h.Peek("Absent"), "Peek must answer nil for a field that is not there")

	present := h.Peek("Origin")
	require.Empty(t, present)
	require.NotNil(t, present, "Peek must answer a non-nil empty slice for a field line that is present and empty")
}

// Test_First_EmptyLineDoesNotHideTheValue covers the read First exists for: a
// field name carried twice, the first line empty. Peek reports that first line
// whatever it holds, so the value beside it was invisible to every caller — a
// response's "Cache-Control: private" behind an empty line of the same name
// read as a response that said nothing about caching.
//
// Origin is used here only because it is a name fasthttp keeps no slot for, the
// same as the response fields this now serves; the request-side reads that must
// refuse a repeated field rather than resolve it go through headerlookup.Value.
func Test_First_EmptyLineDoesNotHideTheValue(t *testing.T) {
	t.Parallel()

	const origin = "http://evil.example"

	tests := []struct {
		name      string
		raw       string
		want      string
		canonical bool
	}{
		{
			name:      "empty line ahead of the value",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\nOrigin: " + origin + "\r\n\r\n",
			canonical: true,
			want:      origin,
		},
		{
			name:      "empty line ahead of another spelling",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\norigin: " + origin + "\r\n\r\n",
			canonical: false,
			want:      origin,
		},
		{
			name:      "another spelling alone",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\norigin: " + origin + "\r\n\r\n",
			canonical: false,
			want:      origin,
		},
		{
			name:      "the ordinary single line",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin: " + origin + "\r\n\r\n",
			canonical: true,
			want:      origin,
		},
		{
			// Present and empty is still nothing a caller can act on, so it
			// reads as absent rather than as a value of "".
			name:      "only an empty line",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\n\r\n",
			canonical: true,
			want:      "",
		},
		{
			name:      "absent",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\n\r\n",
			canonical: true,
			want:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := readRequestHeader(t, tc.raw, tc.canonical)
			require.Equal(t, tc.want, string(First(h, fasthttp.HeaderOrigin, tc.canonical)))
		})
	}
}

// Test_First_Lines_SkipTrailerSection pins that a field sent after the last
// chunk is not read as a header: fasthttp's Peek and PeekAll fall through to it.
// Nor is a header line of its name, which a Set may not have replaced.
func Test_First_Lines_SkipTrailerSection(t *testing.T) {
	t.Parallel()

	const (
		chunked = "POST / HTTP/1.1\r\nHost: app.example\r\nTransfer-Encoding: chunked\r\n"
		body    = "\r\n5\r\nhello\r\n0\r\n"
	)

	tests := []struct {
		name      string
		raw       string
		wantFirst string
		wantLines []string
	}{
		{name: "trailer only", raw: chunked + body + "X-User: admin\r\n\r\n"},
		{name: "announced trailer only", raw: chunked + "Trailer: X-User\r\n" + body + "X-User: admin\r\n\r\n"},
		{name: "empty header line ahead of a trailer", raw: chunked + "X-User:\r\n" + body + "X-User: admin\r\n\r\n"},
		{name: "header line beside a trailer", raw: chunked + "X-User: alice\r\n" + body + "X-User: admin\r\n\r\n"},
		{
			name: "header line beside another trailer", raw: chunked + "X-User: alice\r\n" + body + "X-Other: admin\r\n\r\n",
			wantFirst: "alice", wantLines: []string{"alice"},
		},
		{
			name: "no trailer section", raw: chunked + "X-User: alice\r\n" + body + "\r\n",
			wantFirst: "alice", wantLines: []string{"alice"},
		},
	}

	for _, tc := range tests {
		for _, canonical := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s canonical=%v", tc.name, canonical), func(t *testing.T) {
				t.Parallel()

				req := &fasthttp.Request{}
				if !canonical {
					req.Header.DisableNormalizing()
				}
				require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(tc.raw))))

				require.Equal(t, tc.wantFirst, string(First(&req.Header, "X-User", canonical)))
				var lines []string
				for _, line := range Lines(&req.Header, "X-User", canonical) {
					lines = append(lines, string(line))
				}
				require.Equal(t, tc.wantLines, lines)
			})
		}
	}
}

// Test_First_Lines_ResponseLineUnderTrailerName pins that a response header line is read although the response's
// Trailer field names it: the cache and etag middleware read Cache-Control, Vary and ETag this way.
func Test_First_Lines_ResponseLineUnderTrailerName(t *testing.T) {
	t.Parallel()

	const raw = "HTTP/1.1 200 OK\r\nCache-Control: private, no-store\r\nTrailer: Cache-Control\r\n" +
		"Transfer-Encoding: chunked\r\n\r\n6\r\nsecret\r\n0\r\nCache-Control: public\r\n\r\n"

	for _, canonical := range []bool{true, false} {
		t.Run(fmt.Sprintf("canonical=%v", canonical), func(t *testing.T) {
			t.Parallel()

			resp := &fasthttp.Response{}
			if !canonical {
				resp.Header.DisableNormalizing()
			}
			require.NoError(t, resp.Read(bufio.NewReader(strings.NewReader(raw))))
			require.NotEmpty(t, resp.Header.PeekTrailerKeys())

			require.Equal(t, "private, no-store", string(First(&resp.Header, fasthttp.HeaderCacheControl, canonical)))
			require.Equal(t, [][]byte{[]byte("private, no-store")}, Lines(&resp.Header, fasthttp.HeaderCacheControl, canonical))
		})
	}
}

// Test_DropTrailers pins that the header lines stay and that a later Set lands
// in the header section, where fasthttp filed it with the trailer section before.
func Test_DropTrailers(t *testing.T) {
	t.Parallel()

	const (
		post    = "POST / HTTP/1.1\r\nHost: app.example\r\n"
		chunked = "Transfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n"
	)

	tests := []struct {
		name      string
		raw       string
		wantUA    string
		wantLines []string
	}{
		{name: "trailer only", raw: post + chunked + "X-User: admin\r\n\r\n"},
		{name: "announced only", raw: "GET / HTTP/1.1\r\nHost: app.example\r\nTrailer: X-User\r\n\r\n"},
		{
			name: "header lines beside a trailer", raw: post + "X-User: alice\r\nX-User: bob\r\n" + chunked + "X-User: admin\r\n\r\n",
			wantLines: []string{"alice", "bob"},
		},
		{
			name: "header line announced on a sized body", raw: post + "X-User: alice\r\nTrailer: X-User\r\nContent-Length: 0\r\n\r\n",
			wantLines: []string{"alice"},
		},
		{
			name: "own slot", raw: post + "User-Agent: early\r\n" + chunked + "User-Agent: late\r\nX-User: admin\r\n\r\n",
			wantUA: "early",
		},
	}

	for _, tc := range tests {
		for _, canonical := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s canonical=%v", tc.name, canonical), func(t *testing.T) {
				t.Parallel()

				req := &fasthttp.Request{}
				if !canonical {
					req.Header.DisableNormalizing()
				}
				require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(tc.raw))))

				require.True(t, DropTrailers(&req.Header))

				require.Empty(t, req.Header.PeekTrailerKeys())
				require.Empty(t, req.Header.Peek(fasthttp.HeaderTrailer))
				require.Equal(t, tc.wantUA, string(req.Header.UserAgent()))
				var lines []string
				for _, line := range req.Header.PeekAll("X-User") {
					lines = append(lines, string(line))
				}
				require.Equal(t, tc.wantLines, lines)

				req.Header.Set("X-User", "app")
				require.Equal(t, "app", string(First(&req.Header, "X-User", canonical)))
			})
		}
	}

	t.Run("too many names", func(t *testing.T) {
		t.Parallel()

		var trailers strings.Builder
		for i := range MaxTrailers + 1 {
			fmt.Fprintf(&trailers, "X-%d: v\r\n", i)
		}
		req := &fasthttp.Request{}
		raw := post + "X-0: header\r\n" + chunked + trailers.String() + "\r\n"
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))
		before := req.Header.String()

		require.False(t, DropTrailers(&req.Header))
		require.Equal(t, before, req.Header.String())
		require.Len(t, req.Header.PeekTrailerKeys(), MaxTrailers+1)
	})
}

// Test_ClearTrailers pins that the header comes back as it was without the trailer section, whatever the number of
// names, with every value read before still holding what it held.
func Test_ClearTrailers(t *testing.T) {
	t.Parallel()

	const head = "POST / HTTP/1.1\r\nHost: app.example\r\nUser-Agent: ua\r\nCookie: a=1\r\nX-Role: user\r\nConnection: x-a\r\n" +
		"Cookie: b=2\r\nConnection: x-b\r\nConnection: close\r\nX-Evil: root\r\nX-Forwarded-For: client\r\n" +
		"Transfer-Encoding: chunked\r\nX-Forwarded-For: proxy\r\nX-After: zz\r\n\r\n5\r\nhello\r\n0\r\n"

	for _, names := range []int{2, MaxTrailers + 1} {
		for _, canonical := range []bool{true, false} {
			t.Run(fmt.Sprintf("names=%d canonical=%v", names, canonical), func(t *testing.T) {
				t.Parallel()

				read := func(trailer string) *fasthttp.Request {
					req := &fasthttp.Request{}
					if !canonical {
						req.Header.DisableNormalizing()
					}
					require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(head+trailer+"\r\n"))))
					return req
				}
				var extra strings.Builder
				for i := range names - 2 {
					fmt.Fprintf(&extra, "X-T%d: v\r\n", i)
				}
				req, want := read("X-Role: x\r\nX-Evil: x\r\n"+extra.String()), read("")
				require.Len(t, req.Header.PeekTrailerKeys(), names)
				raw := string(req.Header.RawHeaders())

				var aliased [][]byte
				for _, name := range []string{"X-Role", "X-Evil", "X-Forwarded-For", "X-After"} {
					aliased = append(aliased, req.Header.PeekAll(name)...)
				}
				var held []string
				for _, v := range aliased {
					held = append(held, string(v))
				}

				require.Equal(t, names <= MaxTrailers, ClearTrailers(req))
				require.Equal(t, -1, req.Header.ContentLength())

				require.Empty(t, req.Header.PeekTrailerKeys())
				want.Header.Cookie("") // collected, as the clear collects them
				require.Equal(t, want.Header.String(), req.Header.String())
				require.Equal(t, raw, string(Received(req)))
				for i, v := range aliased {
					require.Equal(t, held[i], string(v))
				}
				req.Header.Set("X-Role", "app")
				require.Equal(t, "app", string(First(&req.Header, "X-Role", canonical)))
				req.Header.Set("x-probe", "1")
				require.Equal(t, !canonical, strings.Contains(req.Header.String(), "x-probe:"))
			})
		}
	}
}

// Test_KeepReceivedOrder pins that repeated lines go back to received order after
// fasthttp's Del moved the last line into the removed one's slot.
func Test_KeepReceivedOrder(t *testing.T) {
	t.Parallel()

	forwarding := func(name []byte) bool { return bytes.HasPrefix(bytes.ToLower(name), []byte("x-forwarded-")) }
	lines := func(h *fasthttp.RequestHeader, name string, canonical bool) []string {
		var values []string
		for _, v := range Lines(h, name, canonical) {
			values = append(values, string(v))
		}
		return values
	}

	for _, canonical := range []bool{true, false} {
		t.Run(fmt.Sprintf("canonical=%v", canonical), func(t *testing.T) {
			t.Parallel()

			const raw = "GET / HTTP/1.1\r\nHost: app.example\r\nX-Junk: 1\r\nX-Forwarded-For: client\r\nX-Forwarded-Proto: http\r\n" +
				"X-Other: o\r\nX-Forwarded-For: proxy\r\nX-Forwarded-Proto: https\r\n\r\n"

			h := readRequestHeader(t, raw, canonical)
			before := h.String()
			require.True(t, KeepReceivedOrder(h, h.RawHeaders(), forwarding))
			require.Equal(t, before, h.String(), "nothing to restore")

			Del(h, "X-Junk", canonical)
			require.Equal(t, []string{"https", "http"}, lines(h, "X-Forwarded-Proto", canonical))
			require.True(t, KeepReceivedOrder(h, h.RawHeaders(), forwarding))
			require.Equal(t, []string{"client", "proxy"}, lines(h, "X-Forwarded-For", canonical))
			require.Equal(t, []string{"http", "https"}, lines(h, "X-Forwarded-Proto", canonical))
			require.Equal(t, []string{"o"}, lines(h, "X-Other", canonical))

			// An added line, which a later Del moves in front of the received ones, follows them.
			h = readRequestHeader(t, raw, canonical)
			h.Add("X-Forwarded-For", "app")
			Del(h, "X-Junk", canonical)
			require.Equal(t, []string{"app", "client", "proxy"}, lines(h, "X-Forwarded-For", canonical))
			require.True(t, KeepReceivedOrder(h, h.RawHeaders(), forwarding))
			require.Equal(t, []string{"client", "proxy", "app"}, lines(h, "X-Forwarded-For", canonical))
			require.Equal(t, []string{"http", "https"}, lines(h, "X-Forwarded-Proto", canonical))

			// So does a changed one.
			h = readRequestHeader(t, raw, canonical)
			h.Set("X-Forwarded-For", "app")
			require.True(t, KeepReceivedOrder(h, h.RawHeaders(), forwarding))
			require.Equal(t, []string{"proxy", "app"}, lines(h, "X-Forwarded-For", canonical))
		})
	}

	t.Run("folded", func(t *testing.T) {
		t.Parallel()

		h := readRequestHeader(t, "GET / HTTP/1.1\r\nHost: app.example\r\nX-Junk: 1\r\nX-Forwarded-For: client,\r\n 6.6.6.6\r\n"+
			"X-Forwarded-For: proxy\r\n\r\n", true)
		Del(h, "X-Junk", true)
		require.True(t, KeepReceivedOrder(h, h.RawHeaders(), forwarding))
		require.Equal(t, []string{"client, 6.6.6.6", "proxy"}, lines(h, "X-Forwarded-For", true))
	})

	t.Run("too many lines", func(t *testing.T) {
		t.Parallel()

		var raw strings.Builder
		fmt.Fprint(&raw, "GET / HTTP/1.1\r\nHost: app.example\r\nX-Junk: 1\r\n")
		for i := range MaxOrderedLines + 1 {
			fmt.Fprintf(&raw, "X-Forwarded-%d: v\r\n", i%2)
		}
		h := readRequestHeader(t, raw.String()+"\r\n", true)
		Del(h, "X-Junk", true)
		before := h.String()
		require.False(t, KeepReceivedOrder(h, h.RawHeaders(), forwarding))
		require.Equal(t, before, h.String())
	})
}

// Test_KeepReceivedOrder_InOrderDoesNotAllocate pins the common shape, a client line and the line a front
// proxy appended, already in received order. Not parallel: AllocsPerRun panics in a parallel test.
func Test_KeepReceivedOrder_InOrderDoesNotAllocate(t *testing.T) {
	h := readRequestHeader(t, "GET / HTTP/1.1\r\nHost: app.example\r\nX-Forwarded-For: client\r\nAccept: */*\r\n"+
		"X-Forwarded-For: proxy\r\nX-Forwarded-Proto: https\r\n\r\n", true)
	forwarding := func(name []byte) bool { return len(name) > 12 && (name[0] == 'X' || name[0] == 'x') }
	allocs := testing.AllocsPerRun(100, func() {
		if !KeepReceivedOrder(h, h.RawHeaders(), forwarding) {
			t.Fatal("refused")
		}
	})
	require.Zero(t, allocs)
}

// Test_ReceivedOrder pins the order a name's lines are read in after fasthttp's Del moved one of them.
func Test_ReceivedOrder(t *testing.T) {
	t.Parallel()

	const head = "GET / HTTP/1.1\r\nHost: app.example\r\nX-Junk: 1\r\n"
	read := func(h *fasthttp.RequestHeader, raw []byte) []string {
		var values []string
		ReceivedOrder(h, raw, "x-forwarded-for", func(v []byte) { values = append(values, string(v)) })
		return values
	}

	for _, canonical := range []bool{true, false} {
		t.Run(fmt.Sprintf("canonical=%v", canonical), func(t *testing.T) {
			t.Parallel()

			h := readRequestHeader(t, head+"X-Forwarded-For: client\r\nAccept: */*\r\nX-Forwarded-For:\tproxy \r\n\r\n", canonical)
			Del(h, "X-Junk", canonical)
			require.Equal(t, []string{"proxy", "client"}, read(h, nil), "the store lost the order")
			require.Equal(t, []string{"client", "proxy"}, read(h, h.RawHeaders()))

			// A line added in code follows the received ones, even once a Del moved it in front of them.
			h = readRequestHeader(t, head+"X-Forwarded-For: client\r\nX-Forwarded-For: proxy\r\n\r\n", canonical)
			h.Add("X-Forwarded-For", "app")
			Del(h, "X-Junk", canonical)
			require.Equal(t, []string{"client", "proxy", "app"}, read(h, h.RawHeaders()))

			// Without a received block the stored order stands.
			require.Equal(t, []string{"app", "client", "proxy"}, read(h, nil))
		})
	}

	t.Run("folded", func(t *testing.T) {
		t.Parallel()

		h := readRequestHeader(t, head+"X-Forwarded-For: client,\r\n 6.6.6.6\r\nX-Forwarded-For: proxy\r\n\r\n", true)
		Del(h, "X-Junk", true)
		require.Equal(t, []string{"client, 6.6.6.6", "proxy"}, read(h, h.RawHeaders()))
	})

	t.Run("recorded block", func(t *testing.T) {
		t.Parallel()

		req := &fasthttp.Request{}
		req.Header.Set("X-Junk", "1")
		req.Header.Add("X-Forwarded-For", "client")
		req.Header.Add("X-Forwarded-For", "proxy")
		block := []byte("X-Forwarded-For: client\r\nX-Forwarded-For: proxy\r\n")
		SetReceived(req, &block)
		require.Equal(t, block, Received(req))
		req.Header.Del("X-Junk")
		require.Equal(t, []string{"client", "proxy"}, read(&req.Header, Received(req)))
	})

	t.Run("too many lines", func(t *testing.T) {
		t.Parallel()

		var raw strings.Builder
		fmt.Fprint(&raw, head)
		for i := range MaxOrderedLines + 1 {
			fmt.Fprintf(&raw, "X-Forwarded-For: %d\r\n", i)
		}
		h := readRequestHeader(t, raw.String()+"\r\n", true)
		h.Add("X-Forwarded-For", "app")
		Del(h, "X-Junk", true)
		values := read(h, h.RawHeaders())
		require.Len(t, values, MaxOrderedLines+1, "the received lines alone")
		require.Equal(t, "0", values[0])
		require.Equal(t, strconv.Itoa(MaxOrderedLines), values[MaxOrderedLines])
	})
}

// Test_ReceivedOrder_DoesNotAllocate pins the reordered shape c.IP() reads on.
// Not parallel: AllocsPerRun panics in a parallel test.
func Test_ReceivedOrder_DoesNotAllocate(t *testing.T) {
	h := readRequestHeader(t, "GET / HTTP/1.1\r\nHost: app.example\r\nX-Junk: 1\r\nX-Forwarded-For: client\r\n"+
		"X-Forwarded-For: proxy\r\n\r\n", true)
	Del(h, "X-Junk", true)
	raw := h.RawHeaders()
	var last []byte
	allocs := testing.AllocsPerRun(100, func() {
		ReceivedOrder(h, raw, "X-Forwarded-For", func(v []byte) { last = v })
	})
	require.Zero(t, allocs)
	require.Equal(t, "proxy", string(last))
}

func Test_Del_TerminatesOnSynthesizedContentType(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h := &fasthttp.ResponseHeader{}
		Del(h, fasthttp.HeaderContentType, false)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Del did not return: ResponseHeader.All reports a default Content-Type once the slot is empty")
	}
}

func Test_Del_TerminatesOnNonCanonicalNameInNormalizingStore(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h := &fasthttp.ResponseHeader{}
		h.SetNoDefaultContentType(true)
		h.SetCanonical([]byte("x-session-id"), []byte("v"))
		Del(h, "X-Session-Id", false)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Del did not return: the store normalizes the key it is given, so it never matches the stored spelling")
	}
}

func Test_Del_RemovesEverySpelling(t *testing.T) {
	t.Parallel()

	h := &fasthttp.ResponseHeader{}
	h.SetNoDefaultContentType(true)
	h.DisableNormalizing()
	h.Add("x-trace", "a")
	h.Add("X-Trace", "b")
	h.Add("X-TRACE", "c")
	h.Add("X-Keep", "k")

	Del(h, "X-Trace", false)

	require.Empty(t, Lines(h, "X-Trace", false))
	require.Equal(t, "k", string(First(h, "X-Keep", false)))
}

// Test_IsCanonical_MatchesFasthttp checks the form against fasthttp itself: a
// token name is canonical exactly when Set stores it unchanged, and any other
// name is not, because fasthttp stores such a key as sent.
func Test_IsCanonical_MatchesFasthttp(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"Origin", "Content-Type", "X-Request-Id", "Sec-Websocket-Key", "A", "X-1", "Etag",
		"Www-Authenticate", "X-_a", "X--Y", "-", "X_Under.Score", "origin", "ORIGIN", "content-type",
		"Content-type", "content-Type", "X-Request-ID", "ETag", "WWW-Authenticate", "Sec-WebSocket-Key",
		"x-request-id", "X-REQUEST-ID", "cONTENT-tYPE", "x--y", "a-b-c-d-e-f", "1-a", "*-a", "X-Real-IP",
		"Content-MD5", "DNT", "TE", "X-A", "X-a", "Ab", "aB", strings.Repeat("Ab-", 21) + "C",
	} {
		h := &fasthttp.ResponseHeader{}
		h.Set(name, "v")
		stored := ""
		for k, v := range h.All() {
			if string(v) == "v" {
				stored = string(k)
			}
		}
		require.Equal(t, stored == name, IsCanonical(name), "IsCanonical(%q); fasthttp stores it as %q", name, stored)
	}

	for _, name := range []string{"", "Origin:", "Origin ", " Origin", "Ori gin", "Origin\r\nX", "Or\u00edgin", "Origin\x00", "X/Y", "(A)"} {
		require.False(t, IsCanonical(name), "IsCanonical(%q)", name)
	}
}

// Test_Peek_MatchesFasthttp is the contract behind the canonical fast path:
// Peek must answer exactly what fasthttp's Peek answers, for every spelling of
// the name, in a normalizing store and in one that keeps the wire spelling.
func Test_Peek_MatchesFasthttp(t *testing.T) {
	t.Parallel()

	const raw = "GET / HTTP/1.1\r\nHost: h\r\nX-Request-Id: a\r\nx-lower: b\r\nContent-Type: text/plain\r\nEmpty:\r\n\r\n"
	names := []string{
		"X-Request-Id", "x-request-id", "X-REQUEST-ID", "x-lower", "X-Lower", "Content-Type", "content-type",
		"Host", "Empty", "empty", "Missing", "", "Bad Key", "X-Request-Id\r\n", "X-Request-Id:",
		strings.Repeat("x-lower", 10),
	}
	for _, canonical := range []bool{true, false} {
		req := readRequestHeader(t, raw, canonical)
		res := &fasthttp.ResponseHeader{}
		if !canonical {
			res.DisableNormalizing()
		}
		res.Set("X-Request-Id", "a")
		res.Set("x-lower", "b")
		res.Set("Empty", "")
		for _, name := range names {
			require.Equal(t, req.Peek(name), Peek(req, name), "request, canonical=%v, %q", canonical, name)
			require.Equal(t, res.Peek(name), Peek(res, name), "response, canonical=%v, %q", canonical, name)
		}
	}
}
