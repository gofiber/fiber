package logger

import (
	"time"

	"github.com/valyala/fasthttp"
)

// Data is a struct to define some variables to use in custom logger function.
// TemplateChain and LogFuncChain are compiled once per middleware instance and
// shared across requests; custom LoggerFunc implementations must treat them as
// read-only.
type Data struct {
	Start         time.Time
	Stop          time.Time
	ChainErr      error
	timeoutResp   *fasthttp.Response // what fasthttp sends for a request timeout.New gave up on, or nil
	req           requestLine        // the request line as it reached the logger, logged once timeoutResp is set
	Timestamp     string
	Pid           string
	ErrPaddingStr string
	TemplateChain [][]byte
	LogFuncChain  []LogFunc
}

// requestLine is what the tags log of the request line and its host, copied before c.Next: the handler of a
// timed-out request may still override or rewrite them (c.Path, c.Method, rewrite, adaptor).
type requestLine struct {
	method   string
	path     []byte
	url      []byte
	protocol []byte
	host     []byte
}
