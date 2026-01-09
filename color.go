// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// Colors is a struct to define custom colors for Fiber app and middlewares.
type Colors struct {
	// Black color.
	//
	// Optional. Default: "\u001b[90m"
	Black string

	// Red color.
	//
	// Optional. Default: "\u001b[91m"
	Red string

	// Green color.
	//
	// Optional. Default: "\u001b[92m"
	Green string

	// Yellow color.
	//
	// Optional. Default: "\u001b[93m"
	Yellow string

	// Blue color.
	//
	// Optional. Default: "\u001b[94m"
	Blue string

	// Magenta color.
	//
	// Optional. Default: "\u001b[95m"
	Magenta string

	// Cyan color.
	//
	// Optional. Default: "\u001b[96m"
	Cyan string

	// White color.
	//
	// Optional. Default: "\u001b[97m"
	White string

	// Reset color.
	//
	// Optional. Default: "\u001b[0m"
	Reset string
}

// DefaultColors Default color codes
var DefaultColors = Colors{
	Black:   "\u001b[90m",
	Red:     "\u001b[91m",
	Green:   "\u001b[92m",
	Yellow:  "\u001b[93m",
	Blue:    "\u001b[94m",
	Magenta: "\u001b[95m",
	Cyan:    "\u001b[96m",
	White:   "\u001b[97m",
	Reset:   "\u001b[0m",
}

// defaultColors is a function to override default colors to config
func defaultColors(colors *Colors) Colors {
	if colors == nil {
		return DefaultColors
	}

	cfg := *colors

	if cfg.Black == "" {
		cfg.Black = DefaultColors.Black
	}

	if cfg.Red == "" {
		cfg.Red = DefaultColors.Red
	}

	if cfg.Green == "" {
		cfg.Green = DefaultColors.Green
	}

	if cfg.Yellow == "" {
		cfg.Yellow = DefaultColors.Yellow
	}

	if cfg.Blue == "" {
		cfg.Blue = DefaultColors.Blue
	}

	if cfg.Magenta == "" {
		cfg.Magenta = DefaultColors.Magenta
	}

	if cfg.Cyan == "" {
		cfg.Cyan = DefaultColors.Cyan
	}

	if cfg.White == "" {
		cfg.White = DefaultColors.White
	}

	if cfg.Reset == "" {
		cfg.Reset = DefaultColors.Reset
	}

	return cfg
}
