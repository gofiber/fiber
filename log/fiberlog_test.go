package log

import (
	"log"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

func Test_DefaultSystemLogger(t *testing.T) {
	defaultL := DefaultLogger()
	utils.AssertEqual(t, logger, defaultL)
}

func Test_SetLogger(t *testing.T) {
	setLog := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  6,
	}

	SetLogger(setLog)
	utils.AssertEqual(t, logger, setLog)
}
