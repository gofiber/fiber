package logger

const (
	// Fiber's default logger `
	FormatDefault = "[${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}\n"
	// Common log format
	FormatCommonLog = "${ip} - - [${time}] \"${method} ${url} ${protocol}\" ${status} ${bytesSent}\n"
	// Combined log format
	FormatCombined = "${ip} - - [${time}] \"${method} ${url} ${protocol}\" ${status} ${bytesSent} \"${referer}\" \"${ua}\"\n"
	// JSON log formats
	FormatJSON = "{\"time\":\"${time}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"url\":\"${url}\",\"status\":${status},\"bytesSent\":${bytesSent}}\n"
)

// LoggerConfig provides a mapping of predefined log format configurations
// that can be used to customize log output styles. The map keys represent
// different log format types, and the values are the corresponding format strings.
var LoggerConfig = map[string]string{
	"default":  FormatDefault,
	"common":   FormatCommonLog,
	"combined": FormatCombined,
	"json":     FormatJSON,
}
