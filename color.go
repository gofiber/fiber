// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
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
func defaultColors(colors Colors) Colors {
	if colors.Black == "" {
		colors.Black = DefaultColors.Black
	}

	if colors.Red == "" {
		colors.Red = DefaultColors.Red
	}

	if colors.Green == "" {
		colors.Green = DefaultColors.Green
	}

	if colors.Yellow == "" {
		colors.Yellow = DefaultColors.Yellow
	}

	if colors.Blue == "" {
		colors.Blue = DefaultColors.Blue
	}

	if colors.Magenta == "" {
		colors.Magenta = DefaultColors.Magenta
	}

	if colors.Cyan == "" {
		colors.Cyan = DefaultColors.Cyan
	}

	if colors.White == "" {
		colors.White = DefaultColors.White
	}

	if colors.Reset == "" {
		colors.Reset = DefaultColors.Reset
	}

	return colors
}
