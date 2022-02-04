package decoder

import "context"

type OptionFlags uint8

const (
	FirstWinOption OptionFlags = 1 << iota
	ContextOption
)

type Option struct {
	Flags   OptionFlags
	Context context.Context
}
