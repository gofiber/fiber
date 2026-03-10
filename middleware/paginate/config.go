package paginate

import (
	"github.com/gofiber/fiber/v3"
)

// Config defines the config for the pagination middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	Next func(c fiber.Ctx) bool

	// PageKey is the query string key for page number.
	//
	// Optional. Default: "page"
	PageKey string

	// LimitKey is the query string key for limit.
	//
	// Optional. Default: "limit"
	LimitKey string

	// SortKey is the query string key for sort.
	//
	// Optional. Default: ""
	SortKey string

	// DefaultSort is the default sort field.
	//
	// Optional. Default: "id"
	DefaultSort string

	// CursorKey is the query string key for cursor-based pagination.
	//
	// Optional. Default: "cursor"
	CursorKey string

	// OffsetKey is the query string key for offset.
	//
	// Optional. Default: "offset"
	OffsetKey string

	// CursorParam is an optional alias for the cursor query key.
	//
	// Optional. Default: ""
	CursorParam string

	// AllowedSorts is the list of allowed sort fields.
	//
	// Optional. Default: nil
	AllowedSorts []string

	// DefaultPage is the default page number.
	//
	// Optional. Default: 1
	DefaultPage int

	// DefaultLimit is the default items per page.
	//
	// Optional. Default: 10
	DefaultLimit int

	// MaxLimit is the maximum items per page.
	//
	// Optional. Default: 100
	MaxLimit int
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:         nil,
	PageKey:      "page",
	DefaultPage:  1,
	LimitKey:     "limit",
	DefaultLimit: 10,
	MaxLimit:     DefaultMaxLimit,
	DefaultSort:  "id",
	OffsetKey:    "offset",
	CursorKey:    "cursor",
}

func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.PageKey == "" {
		cfg.PageKey = ConfigDefault.PageKey
	}
	if cfg.DefaultLimit < 1 {
		cfg.DefaultLimit = ConfigDefault.DefaultLimit
	}
	if cfg.LimitKey == "" {
		cfg.LimitKey = ConfigDefault.LimitKey
	}
	if cfg.DefaultPage < 1 {
		cfg.DefaultPage = ConfigDefault.DefaultPage
	}
	if cfg.CursorKey == "" {
		cfg.CursorKey = ConfigDefault.CursorKey
	}
	if cfg.DefaultSort == "" {
		cfg.DefaultSort = ConfigDefault.DefaultSort
	}
	if cfg.OffsetKey == "" {
		cfg.OffsetKey = ConfigDefault.OffsetKey
	}
	if cfg.MaxLimit < 1 {
		cfg.MaxLimit = ConfigDefault.MaxLimit
	}

	return cfg
}
