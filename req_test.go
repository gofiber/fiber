// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://gitee.com/azhai/fiber-u8l
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

var expectBodies = []string{
	`{"code":200,"data":{"param":"john"}}`,
	`{"code":200,"data":{"page":1,"size":20}}`,
}

func CreateTestGroup() *Group {
	grp := New().Group("/test")
	grp.Get("/:param", func(c *Ctx) {
		param := c.Params("param")
		data := Map{"param": param}
		c.JSON(Map{"code": 200, "data": data})
	})
	grp.Get("/page/:page/:size", func(c *Ctx) {
		page, size := 1, 20
		if pageStr := c.FormValue("page"); pageStr != "" {
			page, _ = strconv.Atoi(pageStr)
		}
		if sizeStr := c.FormValue("size"); sizeStr != "" {
			size, _ = strconv.Atoi(sizeStr)
		}
		data := Map{"page": page, "size": size}
		c.JSON(Map{"code": 200, "data": data})
	})
	return grp
}

//func CreateSeniorGroup() *Group {
//	grp := New().Group("/test")
//	grp.Get("/:param", func(c *Ctx) {
//		param := c.ParamStr("param")
//		c.Reply(Map{"param": param})
//	})
//	grp.Get("/page/:page/:size", func(c *Ctx) {
//		page, size := c.FetchInt("page", 1), c.FetchInt("size", 20)
//		c.Reply(Map{"page": page, "size": size})
//	})
//	return grp
//}

func Test_Request01_ParamStr(t *testing.T) {
	grp := CreateTestGroup()
	req := httptest.NewRequest("GET", "/test/john", nil)
	resp, err := grp.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	assert.Greater(t, n, 0)
	assert.Equal(t, string(body[:n]), expectBodies[0])
}

func Test_Request02_FetchInt(t *testing.T) {
	grp := CreateTestGroup()
	req := httptest.NewRequest("GET", "/test/page/3/7", nil)
	resp, err := grp.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	assert.Greater(t, n, 0)
	assert.Equal(t, string(body[:n]), expectBodies[1])
}
