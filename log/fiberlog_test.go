package log

import (
	"log" //nolint:depguard // stdlib log is only allowed through this package
	"os"
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

func Test_DefaultSystemLogger(t *testing.T) {
	t.Parallel()
	defaultL := DefaultLogger()
	utils.AssertEqual(t, logger, defaultL)
}

//nolint:paralleltest // TODO: Must be run sequentially due to overwriting the default logger global
func Test_SetLogger(t *testing.T) {
	setLog := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  6,
	}

	SetLogger(setLog)
	utils.AssertEqual(t, logger, setLog)
}
