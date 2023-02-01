package monitor

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

func Test_Config_Default(t *testing.T) {
	t.Parallel()

	t.Run("use default", func(t *testing.T) {
		t.Parallel()
		cfg := configDefault()

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJSURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set title", func(t *testing.T) {
		t.Parallel()
		title := "title"
		cfg := configDefault(Config{
			Title: title,
		})

		utils.AssertEqual(t, title, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{title, defaultRefresh, defaultFontURL, defaultChartJSURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set refresh less than default", func(t *testing.T) {
		t.Parallel()
		cfg := configDefault(Config{
			Refresh: 100 * time.Millisecond,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, minRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, minRefresh, defaultFontURL, defaultChartJSURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set refresh", func(t *testing.T) {
		t.Parallel()
		refresh := time.Second
		cfg := configDefault(Config{
			Refresh: refresh,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, refresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, refresh, defaultFontURL, defaultChartJSURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set font url", func(t *testing.T) {
		t.Parallel()
		fontURL := "https://example.com"
		cfg := configDefault(Config{
			FontURL: fontURL,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, fontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, fontURL, defaultChartJSURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set chart js url", func(t *testing.T) {
		t.Parallel()
		chartURL := "http://example.com"
		cfg := configDefault(Config{
			ChartJsURL: chartURL,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, chartURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, chartURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set custom head", func(t *testing.T) {
		t.Parallel()
		head := "head"
		cfg := configDefault(Config{
			CustomHead: head,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, head, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJSURL, head}), cfg.index)
	})

	t.Run("set api only", func(t *testing.T) {
		t.Parallel()
		cfg := configDefault(Config{
			APIOnly: true,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, true, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJSURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set next", func(t *testing.T) {
		t.Parallel()
		f := func(c *fiber.Ctx) bool {
			return true
		}
		cfg := configDefault(Config{
			Next: f,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJSURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, f(nil), cfg.Next(nil))
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJSURL, defaultCustomHead}), cfg.index)
	})
}
