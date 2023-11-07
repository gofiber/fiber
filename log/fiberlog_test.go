package log

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DefaultSystemLogger(t *testing.T) {
	defaultL := DefaultLogger()
	require.Equal(t, logger, defaultL)
}

func Test_SetLogger(t *testing.T) {
	setLog := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  6,
	}

	SetLogger(setLog)
	require.Equal(t, logger, setLog)
}
