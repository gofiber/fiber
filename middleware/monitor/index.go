package monitor

import (
	"strconv"
	"strings"
	"time"
)

type viewBag struct {
	title      string
	refresh    time.Duration
	fontUrl    string
	chartJsUrl string
	customHead string
}

// returns index with new title/refresh
func newIndex(dat viewBag) string {

	timeout := dat.refresh.Milliseconds() - timeoutDiff
	if timeout < timeoutDiff {
		timeout = timeoutDiff
	}
	ts := strconv.FormatInt(timeout, 10)
	replacer := strings.NewReplacer("$TITLE", dat.title, "$TIMEOUT", ts,
		"$FONT_URL", dat.fontUrl, "$CHART_JS_URL", dat.chartJsUrl, "$CUSTOM_HEAD", dat.customHead,
	)
	return replacer.Replace(indexHtml)
}

const (
	defaultTitle = "Fiber Monitor"

	defaultRefresh    = 3 * time.Second
	timeoutDiff       = 200 // timeout will be Refresh (in milliseconds) - timeoutDiff
	minRefresh        = timeoutDiff * time.Millisecond
	defaultFontURL    = `https://fonts.googleapis.com/css2?family=Roboto:wght@400;900&display=swap`
	defaultChartJsURL = `https://cdn.jsdelivr.net/npm/chart.js@2.9/dist/Chart.bundle.min.js`
	defaultCustomHead = ``

	// parametrized by $TITLE and $TIMEOUT

)

// go:embed index.html.tpl
var indexHtml string
