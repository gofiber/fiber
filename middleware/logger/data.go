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
	Timestamp     string
	Pid           string
	ErrPaddingStr string
	TemplateChain [][]byte
	LogFuncChain  []LogFunc
}
