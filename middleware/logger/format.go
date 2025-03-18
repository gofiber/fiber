package logger

const (
	FormatDefault   = "${time}] ${ip} ${status} - ${latency} ${method} ${path} ${error}"
	FormatCommonLog = "${ip} - - [${time}] \"${method} ${url} ${protocol}\" ${status} ${bytesSent}\n"
	FormatCombined  = "${ip} - - [${time}] \"${method} ${url} ${protocol}\" ${status} ${bytesSent} \"${referer}\" \"${ua}\"\n"
	FormatJSON      = "{\"time\":\"${time}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"url\":\"${url}\",\"status\":${status},\"bytesSent\":${bytesSent}}\n"
)

// LoggerConfig provides a mapping of predefined formats
var LoggerConfig = map[string]string{
	"default":  FormatDefault,
	"common":   FormatCommonLog,
	"combined": FormatCombined,
	"json":     FormatJSON,
}
