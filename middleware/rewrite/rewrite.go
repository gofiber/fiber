package rewrite

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

const (
	rewriteReplaceDefaultCap = 8
	rewriteReplaceMaxCap     = 128
)

var replacerArgPool = sync.Pool{ //nolint:gochecknoglobals // shared argument pool
	New: func() any {
		slice := make([]string, 0, rewriteReplaceDefaultCap)
		return &slice
	},
}

func acquireReplacerArgs(pairCount int) *[]string {
	argsAny := replacerArgPool.Get()
	argsPtr, ok := argsAny.(*[]string)
	if !ok {
		panic(errors.New("failed to type-assert to *[]string"))
	}

	needed := pairCount * 2
	args := *argsPtr

	if cap(args) < needed {
		args = make([]string, needed)
	} else {
		args = args[:needed]
	}

	*argsPtr = args

	return argsPtr
}

func releaseReplacerArgs(argsPtr *[]string) {
	if argsPtr == nil {
		return
	}

	args := *argsPtr
	if len(args) > 0 {
		clear(args)
	}

	if cap(args) > rewriteReplaceMaxCap {
		args = make([]string, 0, rewriteReplaceDefaultCap)
	} else {
		args = args[:0]
	}

	*argsPtr = args
	replacerArgPool.Put(argsPtr)
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Initialize
	cfg.rulesRegex = map[*regexp.Regexp]string{}
	for k, v := range cfg.Rules {
		k = strings.ReplaceAll(k, "*", "(.*)")
		k += "$"
		cfg.rulesRegex[regexp.MustCompile(k)] = v
	}
	// Middleware function
	return func(c fiber.Ctx) error {
		// Next request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Rewrite
		for k, v := range cfg.rulesRegex {
			replacer := captureTokens(k, c.Path())
			if replacer != nil {
				c.Path(replacer.Replace(v))
				break
			}
		}
		return c.Next()
	}
}

// https://github.com/labstack/echo/blob/master/middleware/rewrite.go
func captureTokens(pattern *regexp.Regexp, input string) *strings.Replacer {
	groups := pattern.FindAllStringSubmatch(input, -1)
	if groups == nil {
		return nil
	}
	values := groups[0][1:]
	if len(values) == 0 {
		return strings.NewReplacer()
	}

	replacePtr := acquireReplacerArgs(len(values))
	replace := *replacePtr
	for i, v := range values {
		j := 2 * i
		replace[j] = "$" + strconv.Itoa(i+1)
		replace[j+1] = v
	}
	replacer := strings.NewReplacer(replace...)
	releaseReplacerArgs(replacePtr)
	return replacer
}
