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
		cfg := configDefault()

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJsURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set title", func(t *testing.T) {
		title := "title"
		cfg := configDefault(Config{
			Title: title,
		})

		utils.AssertEqual(t, title, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{title, defaultRefresh, defaultFontURL, defaultChartJsURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set refresh less than default", func(t *testing.T) {
		cfg := configDefault(Config{
			Refresh: 100 * time.Millisecond,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, minRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, minRefresh, defaultFontURL, defaultChartJsURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set refresh", func(t *testing.T) {
		refresh := time.Second
		cfg := configDefault(Config{
			Refresh: refresh,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, refresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, refresh, defaultFontURL, defaultChartJsURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set font url", func(t *testing.T) {
		fontUrl := "https://example.com"
		cfg := configDefault(Config{
			FontURL: fontUrl,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, fontUrl, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, fontUrl, defaultChartJsURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set chart js url", func(t *testing.T) {
		chartUrl := "http://example.com"
		cfg := configDefault(Config{
			ChartJsURL: chartUrl,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, chartUrl, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, chartUrl, defaultCustomHead}), cfg.index)
	})

	t.Run("set custom head", func(t *testing.T) {
		head := "head"
		cfg := configDefault(Config{
			CustomHead: head,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, head, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJsURL, head}), cfg.index)
	})

	t.Run("set api only", func(t *testing.T) {
		cfg := configDefault(Config{
			APIOnly: true,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, true, cfg.APIOnly)
		utils.AssertEqual(t, (func(*fiber.Ctx) bool)(nil), cfg.Next)
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJsURL, defaultCustomHead}), cfg.index)
	})

	t.Run("set next", func(t *testing.T) {
		f := func(c *fiber.Ctx) bool {
			return true
		}
		cfg := configDefault(Config{
			Next: f,
		})

		utils.AssertEqual(t, defaultTitle, cfg.Title)
		utils.AssertEqual(t, defaultRefresh, cfg.Refresh)
		utils.AssertEqual(t, defaultFontURL, cfg.FontURL)
		utils.AssertEqual(t, defaultChartJsURL, cfg.ChartJsURL)
		utils.AssertEqual(t, defaultCustomHead, cfg.CustomHead)
		utils.AssertEqual(t, false, cfg.APIOnly)
		utils.AssertEqual(t, f(nil), cfg.Next(nil))
		utils.AssertEqual(t, newIndex(viewBag{defaultTitle, defaultRefresh, defaultFontURL, defaultChartJsURL, defaultCustomHead}), cfg.index)
	})
}
