package logger

import (
	"sync/atomic"
	"time"
)

// Data is a struct to define some variables to use in custom logger function.
// TemplateChain and LogFuncChain are compiled once per middleware instance and
// shared across requests; custom LoggerFunc implementations must treat them as
// read-only.
type Data struct {
	Start         time.Time
	Stop          time.Time
	ChainErr      error
	Timestamp     atomic.Value
	Pid           string
	ErrPaddingStr string
	TemplateChain [][]byte
	LogFuncChain  []LogFunc
}
