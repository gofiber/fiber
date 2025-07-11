package main

import (
	"flag"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
)

type options struct {
	Dir       string
	Path      string
	Logger    bool
	Cors      bool
	Health    bool
	Browse    bool
	Download  bool
	Compress  bool
	Cache     time.Duration
	MaxAge    int
	Index     string
	ByteRange bool
}

func newApp(o options) *fiber.App {
	app := fiber.New()

	// Recover should be registered first to handle panics from later middleware
	app.Use(recover.New())

	if o.Logger {
		app.Use(logger.New())
	}
	if o.Cors {
		app.Use(cors.New())
	}

	if o.Health {
		app.Get(healthcheck.LivenessEndpoint, healthcheck.New())
		app.Get(healthcheck.ReadinessEndpoint, healthcheck.New())
		app.Get(healthcheck.StartupEndpoint, healthcheck.New())
	}

	cfgStatic := static.Config{
		Browse:        o.Browse,
		Download:      o.Download,
		Compress:      o.Compress,
		ByteRange:     o.ByteRange,
		CacheDuration: o.Cache,
		MaxAge:        o.MaxAge,
	}
	if o.Index != "" {
		cfgStatic.IndexNames = strings.Split(o.Index, ",")
	}
	app.Use(o.Path, static.New(o.Dir, cfgStatic))

	return app
}

func main() {
	dir := flag.String("dir", ".", "directory to serve")
	addr := flag.String("addr", ":3000", "address to listen on")
	path := flag.String("path", "/", "request path to serve")
	enableLogger := flag.Bool("logger", true, "enable logger middleware")
	enableCors := flag.Bool("cors", false, "enable CORS middleware")
	enableHealth := flag.Bool("health", true, "enable health check endpoints")
	cert := flag.String("cert", "", "TLS certificate file")
	key := flag.String("key", "", "TLS private key file")
	browse := flag.Bool("browse", false, "enable directory browsing")
	download := flag.Bool("download", false, "force file downloads")
	compress := flag.Bool("compress", false, "enable compression")
	cache := flag.Duration("cache", 10*time.Second, "cache duration")
	maxAge := flag.Int("maxage", 0, "Cache-Control max-age header in seconds")
	index := flag.String("index", "index.html", "comma-separated list of index files")
	byteRange := flag.Bool("range", false, "enable byte range requests")
	prefork := flag.Bool("prefork", false, "enable prefork mode")
	disableStartup := flag.Bool("quiet", false, "disable startup message")
	flag.Parse()

	app := newApp(options{
		Dir:       *dir,
		Path:      *path,
		Logger:    *enableLogger,
		Cors:      *enableCors,
		Health:    *enableHealth,
		Browse:    *browse,
		Download:  *download,
		Compress:  *compress,
		Cache:     *cache,
		MaxAge:    *maxAge,
		Index:     *index,
		ByteRange: *byteRange,
	})

	cfg := fiber.ListenConfig{EnablePrefork: *prefork, DisableStartupMessage: *disableStartup}
	if *cert != "" && *key != "" {
		cfg.CertFile = *cert
		cfg.CertKeyFile = *key
	}

	if err := app.Listen(*addr, cfg); err != nil {
		fiberlog.Fatalf("failed to start server: %v", err)
	}
}
