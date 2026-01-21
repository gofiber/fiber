package favicon

import (
	"io/fs"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// FileSystem is an optional alternate filesystem to search for the favicon in.
	// An example of this could be an embedded or network filesystem
	//
	// Optional. Default: nil
	FileSystem fs.FS `json:"-"`

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// File holds the path to an actual favicon that will be cached
	//
	// Optional. Default: ""
	File string `json:"file"`

	// URL for favicon handler
	//
	// Optional. Default: "/favicon.ico"
	URL string `json:"url"`

	// CacheControl defines how the Cache-Control header in the response should be set
	//
	// Optional. Default: "public, max-age=31536000"
	CacheControl string `json:"cache_control"`

	// Raw data of the favicon file
	//
	// Optional. Default: nil
	Data []byte `json:"-"`

	// MaxBytes limits the maximum size of the cached favicon asset.
	//
	// Optional. Default: 1048576
	MaxBytes int64 `json:"max_bytes"`
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	File:         "",
	URL:          fPath,
	CacheControl: "public, max-age=31536000",
	MaxBytes:     1024 * 1024,
}

func configDefault(config ...Config) Config {
	if len(config) == 0 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.URL == "" {
		cfg.URL = ConfigDefault.URL
	}
	if cfg.File == "" {
		cfg.File = ConfigDefault.File
	}
	if cfg.CacheControl == "" {
		cfg.CacheControl = ConfigDefault.CacheControl
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = ConfigDefault.MaxBytes
	}

	return cfg
}
