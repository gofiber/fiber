package logger

import (
	"sync/atomic"
	"time"
)

// Data is a struct to define some variables to use in custom logger function.
type Data struct {
	Pid           string
	ErrPaddingStr string
	ChainErr      error
	TemplateChain [][]byte
	LogFuncChain  []LogFunc
	Start         time.Time
	Stop          time.Time
	Timestamp     atomic.Value
}
