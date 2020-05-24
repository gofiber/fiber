// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler -benchmem -count=3

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

func Test_Route_Match_SameLength(t *testing.T) {
	app := New()

	app.Get("/:param", func(ctx *Ctx) {
		ctx.Send(ctx.Params("param"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/:param", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, ":param", getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "test", getString(body))
}

func Test_Route_Match_Star(t *testing.T) {
	app := New()

	app.Get("/*", func(ctx *Ctx) {
		ctx.Send(ctx.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/*", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "*", getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "test", getString(body))
}

func Test_Route_Match_Root(t *testing.T) {
	app := New()

	app.Get("/", func(ctx *Ctx) {
		ctx.Send("root")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "root", getString(body))
}

func Test_Route_Match_Parser(t *testing.T) {
	app := New()

	app.Get("/foo/:Param", func(ctx *Ctx) {
		ctx.Send(ctx.Params("Param"))
	})
	app.Get("/Foobar/*", func(ctx *Ctx) {
		ctx.Send(ctx.Params("*"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "bar", getString(body))

	// with star
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/Foobar/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "test", getString(body))
}

func Test_Route_Match_Middleware(t *testing.T) {
	app := New()

	app.Use("/foo/*", func(ctx *Ctx) {
		ctx.Send(ctx.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/*", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "*", getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/bar/fasel", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "bar/fasel", getString(body))
}

func Test_Route_Match_Middleware_HasPrefix(t *testing.T) {
	app := New()

	app.Use("/foo", func(ctx *Ctx) {
		ctx.Send("middleware")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "middleware", getString(body))
}

func Test_Route_Match_Middleware_Root(t *testing.T) {
	app := New()

	app.Use("/", func(ctx *Ctx) {
		ctx.Send("middleware")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/everything", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "middleware", getString(body))
}

//////////////////////////////////////////////
///////////////// BENCHMARKS /////////////////
//////////////////////////////////////////////

func registerDummyRoutes(app *App) {
	h := func(c *Ctx) {}
	for _, r := range githubAPI {
		switch r.method {
		case "GET":
			app.Get(r.path, h)
		case "POST":
			app.Post(r.path, h)
		case "PUT":
			app.Put(r.path, h)
		case "PATCH":
			app.Patch(r.path, h)
		case "DELETE":
			app.Delete(r.path, h)
		default:
			panic("Unknow HTTP method: " + r.method)
		}
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler -benchmem -count=4
func Benchmark_Router_Handler(b *testing.B) {
	app := New()
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

func Benchmark_Router_Handler_Strict_Case(b *testing.B) {
	app := New(&Settings{
		StrictRouting: true,
		CaseSensitive: true,
	})
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Chain -benchmem -count=4
func Benchmark_Router_Chain(b *testing.B) {
	app := New()
	handler := func(c *Ctx) {
		c.Next()
	}
	app.Get("/", handler, handler, handler, handler, handler, handler)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("GET")
	c.URI().SetPath("/")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Next -benchmem -count=4
func Benchmark_Router_Next(b *testing.B) {
	app := New()
	registerDummyRoutes(app)

	request := &fasthttp.RequestCtx{}

	request.Request.Header.SetMethod("DELETE")
	request.URI().SetPath("/user/keys/1337")
	var res bool

	c := app.AcquireCtx(request)
	defer app.ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.index = -1
		res = app.next(c)
	}
	utils.AssertEqual(b, true, res)
	utils.AssertEqual(b, 31, c.index)
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match -benchmem -count=4
func Benchmark_Route_Match(b *testing.B) {
	var match bool
	var params []string

	parsed := parseRoute("/user/keys/:id")
	route := &Route{
		use:         false,
		root:        false,
		star:        false,
		routeParser: parsed,
		routeParams: parsed.params,
		path:        "/user/keys/:id",

		Path:   "/user/keys/:id",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) {})
	for n := 0; n < b.N; n++ {
		match, params = route.match("/user/keys/1337", "/user/keys/1337")
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string{"1337"}, params)
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Star -benchmem -count=4
func Benchmark_Route_Match_Star(b *testing.B) {
	var match bool
	var params []string

	parsed := parseRoute("/*")
	route := &Route{
		use:         false,
		root:        false,
		star:        true,
		routeParser: parsed,
		routeParams: parsed.params,
		path:        "/user/keys/bla",

		Path:   "/user/keys/bla",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) {})
	for n := 0; n < b.N; n++ {
		match, params = route.match("/user/keys/bla", "/user/keys/bla")
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string{"user/keys/bla"}, params)
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Root -benchmem -count=4
func Benchmark_Route_Match_Root(b *testing.B) {
	var match bool
	var params []string

	parsed := parseRoute("/")
	route := &Route{
		use:         false,
		root:        true,
		star:        false,
		path:        "/",
		routeParser: parsed,
		routeParams: parsed.params,

		Path:   "/",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) {})
	for n := 0; n < b.N; n++ {
		match, params = route.match("/", "/")
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string(nil), params)
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_CaseSensitive -benchmem -count=4
func Benchmark_Router_Handler_CaseSensitive(b *testing.B) {
	app := New()
	app.Settings.CaseSensitive = true
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_StrictRouting -benchmem -count=4
func Benchmark_Router_Handler_StrictRouting(b *testing.B) {
	app := New()
	app.Settings.CaseSensitive = true
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Github_API -benchmem -count=4
func Benchmark_Router_Github_API(b *testing.B) {
	app := New()
	registerDummyRoutes(app)

	var match bool
	var params []string

	for n := 0; n < b.N; n++ {
		for i := range testRoutes {

			mINT := methodINT[testRoutes[i].method]
			path := testRoutes[i].path

			for i := range app.stack[mINT] {
				match, params = app.stack[mINT][i].match(path, path)
			}
		}
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, true, params != nil)
}

type testRoute struct {
	method string
	path   string
}

var testRoutes = []testRoute{
	// OAuth Authorizations
	{"GET", "/authorizations"},
	{"GET", "/authorizations/1337"},
	{"POST", "/authorizations"},
	{"PUT", "/authorizations/clients/inf1nd873nf8912g9t"},
	{"PATCH", "/authorizations/1337"},
	{"DELETE", "/authorizations/1337"},
	{"GET", "/applications/2nds981mng6azl127y/tokens/sn108hbe1geheibf13f"},
	{"DELETE", "/applications/2nds981mng6azl127y/tokens"},
	{"DELETE", "/applications/2nds981mng6azl127y/tokens/sn108hbe1geheibf13f"},

	// Activity
	{"GET", "/events"},
	{"GET", "/repos/fenny/fiber/events"},
	{"GET", "/networks/fenny/fiber/events"},
	{"GET", "/orgs/gofiber/events"},
	{"GET", "/users/fenny/received_events"},
	{"GET", "/users/fenny/received_events/public"},
	{"GET", "/users/fenny/events"},
	{"GET", "/users/fenny/events/public"},
	{"GET", "/users/fenny/events/orgs/gofiber"},
	{"GET", "/feeds"},
	{"GET", "/notifications"},
	{"GET", "/repos/fenny/fiber/notifications"},
	{"PUT", "/notifications"},
	{"PUT", "/repos/fenny/fiber/notifications"},
	{"GET", "/notifications/threads/1337"},
	{"PATCH", "/notifications/threads/1337"},
	{"GET", "/notifications/threads/1337/subscription"},
	{"PUT", "/notifications/threads/1337/subscription"},
	{"DELETE", "/notifications/threads/1337/subscription"},
	{"GET", "/repos/fenny/fiber/stargazers"},
	{"GET", "/users/fenny/starred"},
	{"GET", "/user/starred"},
	{"GET", "/user/starred/fenny/fiber"},
	{"PUT", "/user/starred/fenny/fiber"},
	{"DELETE", "/user/starred/fenny/fiber"},
	{"GET", "/repos/fenny/fiber/subscribers"},
	{"GET", "/users/fenny/subscriptions"},
	{"GET", "/user/subscriptions"},
	{"GET", "/repos/fenny/fiber/subscription"},
	{"PUT", "/repos/fenny/fiber/subscription"},
	{"DELETE", "/repos/fenny/fiber/subscription"},
	{"GET", "/user/subscriptions/fenny/fiber"},
	{"PUT", "/user/subscriptions/fenny/fiber"},
	{"DELETE", "/user/subscriptions/fenny/fiber"},

	// Gists
	{"GET", "/users/fenny/gists"},
	{"GET", "/gists"},
	{"GET", "/gists/public"},
	{"GET", "/gists/starred"},
	{"GET", "/gists/1337"},
	{"POST", "/gists"},
	{"PATCH", "/gists/1337"},
	{"PUT", "/gists/1337/star"},
	{"DELETE", "/gists/1337/star"},
	{"GET", "/gists/1337/star"},
	{"POST", "/gists/1337/forks"},
	{"DELETE", "/gists/1337"},

	// Git Data
	{"GET", "/repos/fenny/fiber/git/blobs/v948b24g98ubngw9082bn02giub"},
	{"POST", "/repos/fenny/fiber/git/blobs"},
	{"GET", "/repos/fenny/fiber/git/commits/v948b24g98ubngw9082bn02giub"},
	{"POST", "/repos/fenny/fiber/git/commits"},
	{"GET", "/repos/fenny/fiber/git/refs/im/a/wildcard"},
	{"GET", "/repos/fenny/fiber/git/refs"},
	{"POST", "/repos/fenny/fiber/git/refs"},
	{"PATCH", "/repos/fenny/fiber/git/refs/im/a/wildcard"},
	{"DELETE", "/repos/fenny/fiber/git/refs/im/a/wildcard"},
	{"GET", "/repos/fenny/fiber/git/tags/v948b24g98ubngw9082bn02giub"},
	{"POST", "/repos/fenny/fiber/git/tags"},
	{"GET", "/repos/fenny/fiber/git/trees/v948b24g98ubngw9082bn02giub"},
	{"POST", "/repos/fenny/fiber/git/trees"},

	// Issues
	{"GET", "/issues"},
	{"GET", "/user/issues"},
	{"GET", "/orgs/gofiber/issues"},
	{"GET", "/repos/fenny/fiber/issues"},
	{"GET", "/repos/fenny/fiber/issues/1000"},
	{"POST", "/repos/fenny/fiber/issues"},
	{"PATCH", "/repos/fenny/fiber/issues/1000"},
	{"GET", "/repos/fenny/fiber/assignees"},
	{"GET", "/repos/fenny/fiber/assignees/nic"},
	{"GET", "/repos/fenny/fiber/issues/1000/comments"},
	{"GET", "/repos/fenny/fiber/issues/comments"},
	{"GET", "/repos/fenny/fiber/issues/comments/1337"},
	{"POST", "/repos/fenny/fiber/issues/1000/comments"},
	{"PATCH", "/repos/fenny/fiber/issues/comments/1337"},
	{"DELETE", "/repos/fenny/fiber/issues/comments/1337"},
	{"GET", "/repos/fenny/fiber/issues/1000/events"},
	{"GET", "/repos/fenny/fiber/issues/events"},
	{"GET", "/repos/fenny/fiber/issues/events/1337"},
	{"GET", "/repos/fenny/fiber/labels"},
	{"GET", "/repos/fenny/fiber/labels/john"},
	{"POST", "/repos/fenny/fiber/labels"},
	{"PATCH", "/repos/fenny/fiber/labels/john"},
	{"DELETE", "/repos/fenny/fiber/labels/john"},
	{"GET", "/repos/fenny/fiber/issues/1000/labels"},
	{"POST", "/repos/fenny/fiber/issues/1000/labels"},
	{"DELETE", "/repos/fenny/fiber/issues/1000/labels/john"},
	{"PUT", "/repos/fenny/fiber/issues/1000/labels"},
	{"DELETE", "/repos/fenny/fiber/issues/1000/labels"},
	{"GET", "/repos/fenny/fiber/milestones/1000/labels"},
	{"GET", "/repos/fenny/fiber/milestones"},
	{"GET", "/repos/fenny/fiber/milestones/1000"},
	{"POST", "/repos/fenny/fiber/milestones"},
	{"PATCH", "/repos/fenny/fiber/milestones/1000"},
	{"DELETE", "/repos/fenny/fiber/milestones/1000"},

	// Miscellaneous
	{"GET", "/emojis"},
	{"GET", "/gitignore/templates"},
	{"GET", "/gitignore/templates/john"},
	{"POST", "/markdown"},
	{"POST", "/markdown/raw"},
	{"GET", "/meta"},
	{"GET", "/rate_limit"},

	// Organizations
	{"GET", "/users/fenny/orgs"},
	{"GET", "/user/orgs"},
	{"GET", "/orgs/gofiber"},
	{"PATCH", "/orgs/gofiber"},
	{"GET", "/orgs/gofiber/members"},
	{"GET", "/orgs/gofiber/members/fenny"},
	{"DELETE", "/orgs/gofiber/members/fenny"},
	{"GET", "/orgs/gofiber/public_members"},
	{"GET", "/orgs/gofiber/public_members/fenny"},
	{"PUT", "/orgs/gofiber/public_members/fenny"},
	{"DELETE", "/orgs/gofiber/public_members/fenny"},
	{"GET", "/orgs/gofiber/teams"},
	{"GET", "/teams/1337"},
	{"POST", "/orgs/gofiber/teams"},
	{"PATCH", "/teams/1337"},
	{"DELETE", "/teams/1337"},
	{"GET", "/teams/1337/members"},
	{"GET", "/teams/1337/members/fenny"},
	{"PUT", "/teams/1337/members/fenny"},
	{"DELETE", "/teams/1337/members/fenny"},
	{"GET", "/teams/1337/repos"},
	{"GET", "/teams/1337/repos/fenny/fiber"},
	{"PUT", "/teams/1337/repos/fenny/fiber"},
	{"DELETE", "/teams/1337/repos/fenny/fiber"},
	{"GET", "/user/teams"},

	// Pull Requests
	{"GET", "/repos/fenny/fiber/pulls"},
	{"GET", "/repos/fenny/fiber/pulls/1000"},
	{"POST", "/repos/fenny/fiber/pulls"},
	{"PATCH", "/repos/fenny/fiber/pulls/1000"},
	{"GET", "/repos/fenny/fiber/pulls/1000/commits"},
	{"GET", "/repos/fenny/fiber/pulls/1000/files"},
	{"GET", "/repos/fenny/fiber/pulls/1000/merge"},
	{"PUT", "/repos/fenny/fiber/pulls/1000/merge"},
	{"GET", "/repos/fenny/fiber/pulls/1000/comments"},
	{"GET", "/repos/fenny/fiber/pulls/comments"},
	{"GET", "/repos/fenny/fiber/pulls/comments/1000"},
	{"PUT", "/repos/fenny/fiber/pulls/1000/comments"},
	{"PATCH", "/repos/fenny/fiber/pulls/comments/1000"},
	{"DELETE", "/repos/fenny/fiber/pulls/comments/1000"},

	// Repositories
	{"GET", "/user/repos"},
	{"GET", "/users/fenny/repos"},
	{"GET", "/orgs/gofiber/repos"},
	{"GET", "/repositories"},
	{"POST", "/user/repos"},
	{"POST", "/orgs/gofiber/repos"},
	{"GET", "/repos/fenny/fiber"},
	{"PATCH", "/repos/fenny/fiber"},
	{"GET", "/repos/fenny/fiber/contributors"},
	{"GET", "/repos/fenny/fiber/languages"},
	{"GET", "/repos/fenny/fiber/teams"},
	{"GET", "/repos/fenny/fiber/tags"},
	{"GET", "/repos/fenny/fiber/branches"},
	{"GET", "/repos/fenny/fiber/branches/master"},
	{"DELETE", "/repos/fenny/fiber"},
	{"GET", "/repos/fenny/fiber/collaborators"},
	{"GET", "/repos/fenny/fiber/collaborators/fenny"},
	{"PUT", "/repos/fenny/fiber/collaborators/fenny"},
	{"DELETE", "/repos/fenny/fiber/collaborators/fenny"},
	{"GET", "/repos/fenny/fiber/comments"},
	{"GET", "/repos/fenny/fiber/commits/v948b24g98ubngw9082bn02giub/comments"},
	{"POST", "/repos/fenny/fiber/commits/v948b24g98ubngw9082bn02giub/comments"},
	{"GET", "/repos/fenny/fiber/comments/1337"},
	{"PATCH", "/repos/fenny/fiber/comments/1337"},
	{"DELETE", "/repos/fenny/fiber/comments/1337"},
	{"GET", "/repos/fenny/fiber/commits"},
	{"GET", "/repos/fenny/fiber/commits/v948b24g98ubngw9082bn02giub"},
	{"GET", "/repos/fenny/fiber/readme"},
	{"GET", "/repos/fenny/fiber/contents/im/a/wildcard"},
	{"PUT", "/repos/fenny/fiber/contents/im/a/wildcard"},
	{"DELETE", "/repos/fenny/fiber/contents/im/a/wildcard"},
	{"GET", "/repos/fenny/fiber/gzip/google"},
	{"GET", "/repos/fenny/fiber/keys"},
	{"GET", "/repos/fenny/fiber/keys/1337"},
	{"POST", "/repos/fenny/fiber/keys"},
	{"PATCH", "/repos/fenny/fiber/keys/1337"},
	{"DELETE", "/repos/fenny/fiber/keys/1337"},
	{"GET", "/repos/fenny/fiber/downloads"},
	{"GET", "/repos/fenny/fiber/downloads/1337"},
	{"DELETE", "/repos/fenny/fiber/downloads/1337"},
	{"GET", "/repos/fenny/fiber/forks"},
	{"POST", "/repos/fenny/fiber/forks"},
	{"GET", "/repos/fenny/fiber/hooks"},
	{"GET", "/repos/fenny/fiber/hooks/1337"},
	{"POST", "/repos/fenny/fiber/hooks"},
	{"PATCH", "/repos/fenny/fiber/hooks/1337"},
	{"POST", "/repos/fenny/fiber/hooks/1337/tests"},
	{"DELETE", "/repos/fenny/fiber/hooks/1337"},
	{"POST", "/repos/fenny/fiber/merges"},
	{"GET", "/repos/fenny/fiber/releases"},
	{"GET", "/repos/fenny/fiber/releases/1337"},
	{"POST", "/repos/fenny/fiber/releases"},
	{"PATCH", "/repos/fenny/fiber/releases/1337"},
	{"DELETE", "/repos/fenny/fiber/releases/1337"},
	{"GET", "/repos/fenny/fiber/releases/1337/assets"},
	{"GET", "/repos/fenny/fiber/stats/contributors"},
	{"GET", "/repos/fenny/fiber/stats/commit_activity"},
	{"GET", "/repos/fenny/fiber/stats/code_frequency"},
	{"GET", "/repos/fenny/fiber/stats/participation"},
	{"GET", "/repos/fenny/fiber/stats/punch_card"},
	{"GET", "/repos/fenny/fiber/statuses/google"},
	{"POST", "/repos/fenny/fiber/statuses/google"},

	// Search
	{"GET", "/search/repositories"},
	{"GET", "/search/code"},
	{"GET", "/search/issues"},
	{"GET", "/search/users"},
	{"GET", "/legacy/issues/search/fenny/fibersitory/locked/finish"},
	{"GET", "/legacy/repos/search/finish"},
	{"GET", "/legacy/user/search/finish"},
	{"GET", "/legacy/user/email/info@gofiber.io"},

	// Users
	{"GET", "/users/fenny"},
	{"GET", "/user"},
	{"PATCH", "/user"},
	{"GET", "/users"},
	{"GET", "/user/emails"},
	{"POST", "/user/emails"},
	{"DELETE", "/user/emails"},
	{"GET", "/users/fenny/followers"},
	{"GET", "/user/followers"},
	{"GET", "/users/fenny/following"},
	{"GET", "/user/following"},
	{"GET", "/user/following/fenny"},
	{"GET", "/users/fenny/following/renan"},
	{"PUT", "/user/following/fenny"},
	{"DELETE", "/user/following/fenny"},
	{"GET", "/users/fenny/keys"},
	{"GET", "/user/keys"},
	{"GET", "/user/keys/1337"},
	{"POST", "/user/keys"},
	{"PATCH", "/user/keys/1337"},
	{"DELETE", "/user/keys/1337"},
}

var githubAPI = []testRoute{
	// OAuth Authorizations
	{"GET", "/authorizations"},
	{"GET", "/authorizations/:id"},
	{"POST", "/authorizations"},
	{"PUT", "/authorizations/clients/:client_id"},
	{"PATCH", "/authorizations/:id"},
	{"DELETE", "/authorizations/:id"},
	{"GET", "/applications/:client_id/tokens/:access_token"},
	{"DELETE", "/applications/:client_id/tokens"},
	{"DELETE", "/applications/:client_id/tokens/:access_token"},

	// Activity
	{"GET", "/events"},
	{"GET", "/repos/:owner/:repo/events"},
	{"GET", "/networks/:owner/:repo/events"},
	{"GET", "/orgs/:org/events"},
	{"GET", "/users/:user/received_events"},
	{"GET", "/users/:user/received_events/public"},
	{"GET", "/users/:user/events"},
	{"GET", "/users/:user/events/public"},
	{"GET", "/users/:user/events/orgs/:org"},
	{"GET", "/feeds"},
	{"GET", "/notifications"},
	{"GET", "/repos/:owner/:repo/notifications"},
	{"PUT", "/notifications"},
	{"PUT", "/repos/:owner/:repo/notifications"},
	{"GET", "/notifications/threads/:id"},
	{"PATCH", "/notifications/threads/:id"},
	{"GET", "/notifications/threads/:id/subscription"},
	{"PUT", "/notifications/threads/:id/subscription"},
	{"DELETE", "/notifications/threads/:id/subscription"},
	{"GET", "/repos/:owner/:repo/stargazers"},
	{"GET", "/users/:user/starred"},
	{"GET", "/user/starred"},
	{"GET", "/user/starred/:owner/:repo"},
	{"PUT", "/user/starred/:owner/:repo"},
	{"DELETE", "/user/starred/:owner/:repo"},
	{"GET", "/repos/:owner/:repo/subscribers"},
	{"GET", "/users/:user/subscriptions"},
	{"GET", "/user/subscriptions"},
	{"GET", "/repos/:owner/:repo/subscription"},
	{"PUT", "/repos/:owner/:repo/subscription"},
	{"DELETE", "/repos/:owner/:repo/subscription"},
	{"GET", "/user/subscriptions/:owner/:repo"},
	{"PUT", "/user/subscriptions/:owner/:repo"},
	{"DELETE", "/user/subscriptions/:owner/:repo"},

	// Gists
	{"GET", "/users/:user/gists"},
	{"GET", "/gists"},
	{"GET", "/gists/public"},
	{"GET", "/gists/starred"},
	{"GET", "/gists/:id"},
	{"POST", "/gists"},
	{"PATCH", "/gists/:id"},
	{"PUT", "/gists/:id/star"},
	{"DELETE", "/gists/:id/star"},
	{"GET", "/gists/:id/star"},
	{"POST", "/gists/:id/forks"},
	{"DELETE", "/gists/:id"},

	// Git Data
	{"GET", "/repos/:owner/:repo/git/blobs/:sha"},
	{"POST", "/repos/:owner/:repo/git/blobs"},
	{"GET", "/repos/:owner/:repo/git/commits/:sha"},
	{"POST", "/repos/:owner/:repo/git/commits"},
	{"GET", "/repos/:owner/:repo/git/refs/*"},
	{"GET", "/repos/:owner/:repo/git/refs"},
	{"POST", "/repos/:owner/:repo/git/refs"},
	{"PATCH", "/repos/:owner/:repo/git/refs/*"},
	{"DELETE", "/repos/:owner/:repo/git/refs/*"},
	{"GET", "/repos/:owner/:repo/git/tags/:sha"},
	{"POST", "/repos/:owner/:repo/git/tags"},
	{"GET", "/repos/:owner/:repo/git/trees/:sha"},
	{"POST", "/repos/:owner/:repo/git/trees"},

	// Issues
	{"GET", "/issues"},
	{"GET", "/user/issues"},
	{"GET", "/orgs/:org/issues"},
	{"GET", "/repos/:owner/:repo/issues"},
	{"GET", "/repos/:owner/:repo/issues/:number"},
	{"POST", "/repos/:owner/:repo/issues"},
	{"PATCH", "/repos/:owner/:repo/issues/:number"},
	{"GET", "/repos/:owner/:repo/assignees"},
	{"GET", "/repos/:owner/:repo/assignees/:assignee"},
	{"GET", "/repos/:owner/:repo/issues/:number/comments"},
	{"GET", "/repos/:owner/:repo/issues/comments"},
	{"GET", "/repos/:owner/:repo/issues/comments/:id"},
	{"POST", "/repos/:owner/:repo/issues/:number/comments"},
	{"PATCH", "/repos/:owner/:repo/issues/comments/:id"},
	{"DELETE", "/repos/:owner/:repo/issues/comments/:id"},
	{"GET", "/repos/:owner/:repo/issues/:number/events"},
	{"GET", "/repos/:owner/:repo/issues/events"},
	{"GET", "/repos/:owner/:repo/issues/events/:id"},
	{"GET", "/repos/:owner/:repo/labels"},
	{"GET", "/repos/:owner/:repo/labels/:name"},
	{"POST", "/repos/:owner/:repo/labels"},
	{"PATCH", "/repos/:owner/:repo/labels/:name"},
	{"DELETE", "/repos/:owner/:repo/labels/:name"},
	{"GET", "/repos/:owner/:repo/issues/:number/labels"},
	{"POST", "/repos/:owner/:repo/issues/:number/labels"},
	{"DELETE", "/repos/:owner/:repo/issues/:number/labels/:name"},
	{"PUT", "/repos/:owner/:repo/issues/:number/labels"},
	{"DELETE", "/repos/:owner/:repo/issues/:number/labels"},
	{"GET", "/repos/:owner/:repo/milestones/:number/labels"},
	{"GET", "/repos/:owner/:repo/milestones"},
	{"GET", "/repos/:owner/:repo/milestones/:number"},
	{"POST", "/repos/:owner/:repo/milestones"},
	{"PATCH", "/repos/:owner/:repo/milestones/:number"},
	{"DELETE", "/repos/:owner/:repo/milestones/:number"},

	// Miscellaneous
	{"GET", "/emojis"},
	{"GET", "/gitignore/templates"},
	{"GET", "/gitignore/templates/:name"},
	{"POST", "/markdown"},
	{"POST", "/markdown/raw"},
	{"GET", "/meta"},
	{"GET", "/rate_limit"},

	// Organizations
	{"GET", "/users/:user/orgs"},
	{"GET", "/user/orgs"},
	{"GET", "/orgs/:org"},
	{"PATCH", "/orgs/:org"},
	{"GET", "/orgs/:org/members"},
	{"GET", "/orgs/:org/members/:user"},
	{"DELETE", "/orgs/:org/members/:user"},
	{"GET", "/orgs/:org/public_members"},
	{"GET", "/orgs/:org/public_members/:user"},
	{"PUT", "/orgs/:org/public_members/:user"},
	{"DELETE", "/orgs/:org/public_members/:user"},
	{"GET", "/orgs/:org/teams"},
	{"GET", "/teams/:id"},
	{"POST", "/orgs/:org/teams"},
	{"PATCH", "/teams/:id"},
	{"DELETE", "/teams/:id"},
	{"GET", "/teams/:id/members"},
	{"GET", "/teams/:id/members/:user"},
	{"PUT", "/teams/:id/members/:user"},
	{"DELETE", "/teams/:id/members/:user"},
	{"GET", "/teams/:id/repos"},
	{"GET", "/teams/:id/repos/:owner/:repo"},
	{"PUT", "/teams/:id/repos/:owner/:repo"},
	{"DELETE", "/teams/:id/repos/:owner/:repo"},
	{"GET", "/user/teams"},

	// Pull Requests
	{"GET", "/repos/:owner/:repo/pulls"},
	{"GET", "/repos/:owner/:repo/pulls/:number"},
	{"POST", "/repos/:owner/:repo/pulls"},
	{"PATCH", "/repos/:owner/:repo/pulls/:number"},
	{"GET", "/repos/:owner/:repo/pulls/:number/commits"},
	{"GET", "/repos/:owner/:repo/pulls/:number/files"},
	{"GET", "/repos/:owner/:repo/pulls/:number/merge"},
	{"PUT", "/repos/:owner/:repo/pulls/:number/merge"},
	{"GET", "/repos/:owner/:repo/pulls/:number/comments"},
	{"GET", "/repos/:owner/:repo/pulls/comments"},
	{"GET", "/repos/:owner/:repo/pulls/comments/:number"},
	{"PUT", "/repos/:owner/:repo/pulls/:number/comments"},
	{"PATCH", "/repos/:owner/:repo/pulls/comments/:number"},
	{"DELETE", "/repos/:owner/:repo/pulls/comments/:number"},

	// Repositories
	{"GET", "/user/repos"},
	{"GET", "/users/:user/repos"},
	{"GET", "/orgs/:org/repos"},
	{"GET", "/repositories"},
	{"POST", "/user/repos"},
	{"POST", "/orgs/:org/repos"},
	{"GET", "/repos/:owner/:repo"},
	{"PATCH", "/repos/:owner/:repo"},
	{"GET", "/repos/:owner/:repo/contributors"},
	{"GET", "/repos/:owner/:repo/languages"},
	{"GET", "/repos/:owner/:repo/teams"},
	{"GET", "/repos/:owner/:repo/tags"},
	{"GET", "/repos/:owner/:repo/branches"},
	{"GET", "/repos/:owner/:repo/branches/:branch"},
	{"DELETE", "/repos/:owner/:repo"},
	{"GET", "/repos/:owner/:repo/collaborators"},
	{"GET", "/repos/:owner/:repo/collaborators/:user"},
	{"PUT", "/repos/:owner/:repo/collaborators/:user"},
	{"DELETE", "/repos/:owner/:repo/collaborators/:user"},
	{"GET", "/repos/:owner/:repo/comments"},
	{"GET", "/repos/:owner/:repo/commits/:sha/comments"},
	{"POST", "/repos/:owner/:repo/commits/:sha/comments"},
	{"GET", "/repos/:owner/:repo/comments/:id"},
	{"PATCH", "/repos/:owner/:repo/comments/:id"},
	{"DELETE", "/repos/:owner/:repo/comments/:id"},
	{"GET", "/repos/:owner/:repo/commits"},
	{"GET", "/repos/:owner/:repo/commits/:sha"},
	{"GET", "/repos/:owner/:repo/readme"},
	{"GET", "/repos/:owner/:repo/contents/*"},
	{"PUT", "/repos/:owner/:repo/contents/*"},
	{"DELETE", "/repos/:owner/:repo/contents/*"},
	{"GET", "/repos/:owner/:repo/:archive_format/:ref"},
	{"GET", "/repos/:owner/:repo/keys"},
	{"GET", "/repos/:owner/:repo/keys/:id"},
	{"POST", "/repos/:owner/:repo/keys"},
	{"PATCH", "/repos/:owner/:repo/keys/:id"},
	{"DELETE", "/repos/:owner/:repo/keys/:id"},
	{"GET", "/repos/:owner/:repo/downloads"},
	{"GET", "/repos/:owner/:repo/downloads/:id"},
	{"DELETE", "/repos/:owner/:repo/downloads/:id"},
	{"GET", "/repos/:owner/:repo/forks"},
	{"POST", "/repos/:owner/:repo/forks"},
	{"GET", "/repos/:owner/:repo/hooks"},
	{"GET", "/repos/:owner/:repo/hooks/:id"},
	{"POST", "/repos/:owner/:repo/hooks"},
	{"PATCH", "/repos/:owner/:repo/hooks/:id"},
	{"POST", "/repos/:owner/:repo/hooks/:id/tests"},
	{"DELETE", "/repos/:owner/:repo/hooks/:id"},
	{"POST", "/repos/:owner/:repo/merges"},
	{"GET", "/repos/:owner/:repo/releases"},
	{"GET", "/repos/:owner/:repo/releases/:id"},
	{"POST", "/repos/:owner/:repo/releases"},
	{"PATCH", "/repos/:owner/:repo/releases/:id"},
	{"DELETE", "/repos/:owner/:repo/releases/:id"},
	{"GET", "/repos/:owner/:repo/releases/:id/assets"},
	{"GET", "/repos/:owner/:repo/stats/contributors"},
	{"GET", "/repos/:owner/:repo/stats/commit_activity"},
	{"GET", "/repos/:owner/:repo/stats/code_frequency"},
	{"GET", "/repos/:owner/:repo/stats/participation"},
	{"GET", "/repos/:owner/:repo/stats/punch_card"},
	{"GET", "/repos/:owner/:repo/statuses/:ref"},
	{"POST", "/repos/:owner/:repo/statuses/:ref"},

	// Search
	{"GET", "/search/repositories"},
	{"GET", "/search/code"},
	{"GET", "/search/issues"},
	{"GET", "/search/users"},
	{"GET", "/legacy/issues/search/:owner/:repository/:state/:keyword"},
	{"GET", "/legacy/repos/search/:keyword"},
	{"GET", "/legacy/user/search/:keyword"},
	{"GET", "/legacy/user/email/:email"},

	// Users
	{"GET", "/users/:user"},
	{"GET", "/user"},
	{"PATCH", "/user"},
	{"GET", "/users"},
	{"GET", "/user/emails"},
	{"POST", "/user/emails"},
	{"DELETE", "/user/emails"},
	{"GET", "/users/:user/followers"},
	{"GET", "/user/followers"},
	{"GET", "/users/:user/following"},
	{"GET", "/user/following"},
	{"GET", "/user/following/:user"},
	{"GET", "/users/:user/following/:target_user"},
	{"PUT", "/user/following/:user"},
	{"DELETE", "/user/following/:user"},
	{"GET", "/users/:user/keys"},
	{"GET", "/user/keys"},
	{"GET", "/user/keys/:id"},
	{"POST", "/user/keys"},
	{"PATCH", "/user/keys/:id"},
	{"DELETE", "/user/keys/:id"},
}
