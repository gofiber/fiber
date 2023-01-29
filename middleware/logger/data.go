package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

var DataPool = sync.Pool{New: func() interface{} { return new(Data) }}

// Data is a struct to define some variables to use in custom logger function.
type Data struct {
	Pid           string
	ErrPaddingStr string
	ChainErr      error
	Start         time.Time
	Stop          time.Time
	Timestamp     atomic.Value
}
