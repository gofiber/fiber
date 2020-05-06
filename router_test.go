// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was based on urlpath created by @ucarion (MIT License).
// 💖 Modified for the Fiber router by @renanbastos93 & @renewerner87

package fiber

import (
	"fmt"
	"reflect"
	"testing"
)

var app *App

func init() {
	app = New()
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

type testcase struct {
	uri    string
	params []string
	ok     bool
}

func Test_With_Param_And_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param/*"),
		[]testcase{
			{uri: "/api/v1/entity", params: []string{"entity", ""}, ok: true},
			{uri: "/api/v1/entity/", params: []string{"entity", ""}, ok: true},
			{uri: "/api/v1/entity/1", params: []string{"entity", "1"}, ok: true},
			{uri: "/api/v", params: nil, ok: false},
			{uri: "/api/v2", params: nil, ok: false},
			{uri: "/api/v1/", params: nil, ok: false},
		},
	)
}

func Test_With_A_Param_Optional(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param?"),
		[]testcase{
			{uri: "/api/v1", params: []string{""}, ok: true},
			{uri: "/api/v1/", params: []string{""}, ok: true},
			{uri: "/api/v1/optional", params: []string{"optional"}, ok: true},
			{uri: "/api/v", params: nil, ok: false},
			{uri: "/api/v2", params: nil, ok: false},
			{uri: "/api/xyz", params: nil, ok: false},
		},
	)
}

func Test_With_With_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/*"),
		[]testcase{
			{uri: "/api/v1", params: []string{""}, ok: true},
			{uri: "/api/v1/", params: []string{""}, ok: true},
			{uri: "/api/v1/entity", params: []string{"entity"}, ok: true},
			{uri: "/api/v1/entity/1/2", params: []string{"entity/1/2"}, ok: true},
			{uri: "/api/v", params: nil, ok: false},
			{uri: "/api/v2", params: nil, ok: false},
			{uri: "/api/abc", params: nil, ok: false},
		},
	)
}
func Test_With_With_Param(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param"),
		[]testcase{
			{uri: "/api/v1/entity", params: []string{"entity"}, ok: true},
			{uri: "/api/v1", params: nil, ok: false},
			{uri: "/api/v1/", params: nil, ok: false},
		},
	)
}

func Test_With_Without_A_Param_Or_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/const"),
		[]testcase{
			{uri: "/api/v1/const", params: []string{}, ok: true},
			{uri: "/api/v1", params: nil, ok: false},
			{uri: "/api/v1/", params: nil, ok: false},
			{uri: "/api/v1/something", params: nil, ok: false},
		},
	)
}
func Test_With_With_A_Param_And_Wildcard_Differents_Positions(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param/abc/*"),
		[]testcase{
			{uri: "/api/v1/well/abc/wildcard", params: []string{"well", "wildcard"}, ok: true},
			{uri: "/api/v1/well/abc/", params: []string{"well", ""}, ok: true},
			{uri: "/api/v1/well/abc", params: []string{"well", ""}, ok: true},
			{uri: "/api/v1/well/ttt", params: nil, ok: false},
		},
	)
}
func Test_With_With_Params_And_Optional(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/:day/:month?/:year?"),
		[]testcase{
			{uri: "/api/1", params: []string{"1", "", ""}, ok: true},
			{uri: "/api/1/", params: []string{"1", "", ""}, ok: true},
			{uri: "/api/1/2", params: []string{"1", "2", ""}, ok: true},
			{uri: "/api/1/2/3", params: []string{"1", "2", "3"}, ok: true},
			{uri: "/api/", params: nil, ok: false},
		},
	)
}
func Test_With_With_Simple_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*"),
		[]testcase{
			{uri: "/api/", params: []string{""}, ok: true},
			{uri: "/api/joker", params: []string{"joker"}, ok: true},
			{uri: "/api", params: []string{""}, ok: true},
			{uri: "/api/v1/entity", params: []string{"v1/entity"}, ok: true},
			{uri: "/api2/v1/entity", params: nil, ok: false},
			{uri: "/api_ignore/v1/entity", params: nil, ok: false},
		},
	)
}
func Test_With_With_Wildcard_And_Optional(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*/:param?"),
		[]testcase{
			{uri: "/api/", params: []string{"", ""}, ok: true},
			{uri: "/api/joker", params: []string{"joker", ""}, ok: true},
			{uri: "/api/joker/batman", params: []string{"joker", "batman"}, ok: true},
			{uri: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, ok: true},
			{uri: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, ok: true},
			{uri: "/api", params: []string{"", ""}, ok: true},
		},
	)
}
func Test_With_With_Wildcard_And_Param(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*/:param"),
		[]testcase{
			{uri: "/api/test/abc", params: []string{"test", "abc"}, ok: true},
			{uri: "/api/joker/batman", params: []string{"joker", "batman"}, ok: true},
			{uri: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, ok: true},
			{uri: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, ok: true},
			{uri: "/api", params: nil, ok: false},
		},
	)
}
func Test_With_With_Wildcard_And_2Params(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*/:param/:param2"),
		[]testcase{
			{uri: "/api/test/abc", params: nil, ok: false},
			{uri: "/api/joker/batman", params: nil, ok: false},
			{uri: "/api/joker/batman/robin", params: []string{"joker", "batman", "robin"}, ok: true},
			{uri: "/api/joker/batman/robin/1", params: []string{"joker/batman", "robin", "1"}, ok: true},
			{uri: "/api/joker/batman/robin/1/2", params: []string{"joker/batman/robin", "1", "2"}, ok: true},
			{uri: "/api", params: nil, ok: false},
		},
	)
}
func Test_With_With_Simple_Path(t *testing.T) {
	checkCases(
		t,
		parseParams("/"),
		[]testcase{
			{uri: "/api", params: nil, ok: false},
			{uri: "", params: []string{}, ok: true},
			{uri: "/", params: []string{}, ok: true},
		},
	)
}
func Test_With_With_Empty_Path(t *testing.T) {
	checkCases(
		t,
		parseParams(""),
		[]testcase{
			{uri: "/api", params: nil, ok: false},
			{uri: "", params: []string{}, ok: true},
			{uri: "/", params: []string{}, ok: true},
		},
	)
}

func Test_With_With_FileName(t *testing.T) {
	checkCases(
		t,
		parseParams("/config/abc.json"),
		[]testcase{
			{uri: "/config/abc.json", params: []string{}, ok: true},
			{uri: "config/abc.json", params: nil, ok: false},
			{uri: "/config/efg.json", params: nil, ok: false},
			{uri: "/config", params: nil, ok: false},
		},
	)
}

func Test_With_With_FileName_And_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/config/*.json"),
		[]testcase{
			{uri: "/config/abc.json", params: []string{"abc.json"}, ok: true},
			{uri: "/config/efg.json", params: []string{"efg.json"}, ok: true},
			//{uri: "/config/efg.csv", params: nil, ok: false},// doesn`t work, current: params: "efg.csv", true
			{uri: "config/abc.json", params: nil, ok: false},
			{uri: "/config", params: nil, ok: false},
		},
	)
}

func Test_With_With_Simple_Path_And_NoMatch(t *testing.T) {
	checkCases(
		t,
		parseParams("/xyz"),
		[]testcase{
			{uri: "xyz", params: nil, ok: false},
			{uri: "xyz/", params: nil, ok: false},
		},
	)
}

func checkCases(tParent *testing.T, parser parsedParams, tcases []testcase) {
	for _, tcase := range tcases {
		tParent.Run(fmt.Sprintf("%+v", tcase), func(t *testing.T) {
			params, ok := parser.matchParams(tcase.uri)
			if !reflect.DeepEqual(params, tcase.params) {
				t.Errorf("Path.Match() got = %v, want %v", params, tcase.params)
			}
			if ok != tcase.ok {
				t.Errorf("Path.Match() got1 = %v, want %v", ok, tcase.ok)
			}
		})
	}
}

func Benchmark_Router(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, r := range testRoutes {
			_, _ = matchRoute(r.method, r.path)
		}
	}
}

func matchRoute(method, path string) (match bool, values []string) {
	for i := range app.routes {
		match, values = app.routes[i].matchRoute(method, path)
		if match {
			return
		}
	}
	return
}

type route struct {
	method string
	path   string
}

var testRoutes = []route{
	// We need to add more routes to test
	{"GET", "/authorizations/1337"},
	{"DELETE", "/user/starred/fenny/fiber"},
	{"PATCH", "/repos/fenny/fiber/git/refs/teams/maintainers"},
}

var githubAPI = []route{
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
