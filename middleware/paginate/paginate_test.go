package paginate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

// --- Config tests ---

// Test_ConfigDefault covers the config New runs with: the documented defaults,
// filled in for what a config leaves out or sets out of range.
func Test_ConfigDefault(t *testing.T) {
	t.Parallel()
	// defaults spells ConfigDefault out, so a change to it fails here.
	defaults := Config{
		PageKey: "page", DefaultPage: 1, LimitKey: "limit", DefaultLimit: 10, MaxLimit: DefaultMaxLimit,
		DefaultSort: "id", OffsetKey: "offset", CursorKey: "cursor",
	}
	override, cursor, maxLimit := defaults, defaults, defaults
	override.PageKey, override.LimitKey, override.DefaultPage, override.DefaultLimit = "p", "l", 5, 50
	cursor.CursorKey, cursor.CursorParam = "after", "starting_after"
	maxLimit.MaxLimit = 50

	for _, tc := range []struct {
		name string
		cfg  []Config
		want Config
	}{
		{"no config", nil, defaults},
		{"override", []Config{{PageKey: "p", LimitKey: "l", DefaultPage: 5, DefaultLimit: 50}}, override},
		{"override cursor key", []Config{{CursorKey: "after", CursorParam: "starting_after"}}, cursor},
		{"negative defaults", []Config{{DefaultPage: -1, DefaultLimit: -1}}, defaults},
		{"zero MaxLimit is normalized", []Config{{MaxLimit: 0}}, defaults},
		{"MaxLimit is kept", []Config{{MaxLimit: 50}}, maxLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, configDefault(tc.cfg...))
		})
	}
}

// --- PageInfo tests ---

func Test_SortOrderFromString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected SortOrder
	}{
		{"asc", ASC},
		{"desc", DESC},
		{"DESC", DESC},
		{"Desc", DESC},
		{"invalid", ASC},
		{"", ASC},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, SortOrderFromString(tt.input))
		})
	}
}

func Test_PageInfoStart(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pageInfo PageInfo
		expected int
	}{
		{"Page 1, limit 10", PageInfo{Page: 1, Limit: 10}, 0},
		{"Page 1, limit 1", PageInfo{Page: 1, Limit: 1}, 0},
		{"Page 2, limit 10", PageInfo{Page: 2, Limit: 10}, 10},
		{"Page 2, limit 1", PageInfo{Page: 2, Limit: 1}, 1},
		{"Page 3, limit 20", PageInfo{Page: 3, Limit: 20}, 40},
		{"With offset", PageInfo{Page: 2, Limit: 10, Offset: 25}, 25},
		{"Zero page", PageInfo{Page: 0, Limit: 10}, 0},
		// In cursor mode, Page is 0 (not set). Start() should return 0, not negative.
		{"Cursor mode", PageInfo{Page: 0, Limit: 20}, 0},
		{"Overflow clamps to MaxInt", PageInfo{Page: int(^uint(0) >> 1), Limit: 100}, int(^uint(0) >> 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.pageInfo.Start())
		})
	}
}

func Test_PageInfoSortBy(t *testing.T) {
	t.Parallel()
	p := NewPageInfo(1, 10, 0, nil)
	p.SortBy("name", ASC).SortBy("date", DESC)

	require.Len(t, p.Sort, 2)
	require.Equal(t, "name", p.Sort[0].Field)
	require.Equal(t, ASC, p.Sort[0].Order)
	require.Equal(t, "date", p.Sort[1].Field)
	require.Equal(t, DESC, p.Sort[1].Order)
}

// Test_PageInfoPageURLs covers the next and previous page links, with the
// default query keys and with custom ones.
func Test_PageInfoPageURLs(t *testing.T) {
	t.Parallel()
	const users, items = "https://example.com/users", "https://example.com/items"
	const filtered = users + "?filter=active"
	capped := "/items?limit=10&page=" + strconv.Itoa(math.MaxInt)
	next, prev := (*PageInfo).NextPageURL, (*PageInfo).PreviousPageURL
	nextWithKeys := func(p *PageInfo, base string) string { return p.NextPageURLWithKeys(base, "p", "per_page") }
	prevWithKeys := func(p *PageInfo, base string) string { return p.PreviousPageURLWithKeys(base, "p", "size") }

	for _, tc := range []struct {
		pageURL    func(*PageInfo, string) string
		name, base string
		expected   string
		pageInfo   PageInfo
	}{
		{next, "Next, middle page", users, users + "?limit=10&page=3", PageInfo{Page: 2, Limit: 10}},
		{next, "Next, first page", users, users + "?limit=20&page=2", PageInfo{Page: 1, Limit: 20}},
		{prev, "Previous, middle page", users, users + "?limit=10&page=1", PageInfo{Page: 2, Limit: 10}},
		{prev, "Previous, first page returns empty", users, "", PageInfo{Page: 1, Limit: 20}},
		{next, "Next keeps the query", filtered, filtered + "&limit=10&page=3", PageInfo{Page: 2, Limit: 10}},
		{prev, "Previous keeps the query", filtered, filtered + "&limit=10&page=2", PageInfo{Page: 3, Limit: 10}},
		{next, "Next does not overflow", "/items", capped, PageInfo{Page: math.MaxInt, Limit: 10}},
		{nextWithKeys, "Next with keys", users, users + "?p=3&per_page=10", PageInfo{Page: 2, Limit: 10}},
		{prevWithKeys, "Previous with keys", items, items + "?p=2&size=15", PageInfo{Page: 3, Limit: 15}},
		{prevWithKeys, "Previous with keys, first page returns empty", items, "", PageInfo{Page: 1, Limit: 15}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.pageURL(&tc.pageInfo, tc.base))
		})
	}
}

func Test_encodeQueryValues(t *testing.T) {
	t.Parallel()
	cases := []url.Values{
		{},
		{"a": {"1"}},
		{"filter": {"active"}, "page": {"2"}, "limit": {"10"}},
		{"q": {"a b+c&d=e", "second value"}, "sp ace": {"100%"}},
		{"unicode": {"héllo wörld"}, "reserved": {";/?:@&=+$,#[]"}},
		{"empty": {""}, "multi": {"", "x", ""}},
	}
	for _, q := range cases {
		require.Equal(t, q.Encode(), encodeQueryValues(q))
	}
}

func Test_PageInfoCursorFields(t *testing.T) {
	t.Parallel()
	p := &PageInfo{Cursor: "abc123", HasMore: true, NextCursor: "def456"}

	require.Equal(t, "abc123", p.Cursor)
	require.True(t, p.HasMore)
	require.Equal(t, "def456", p.NextCursor)
}

func Test_CursorValuesRoundTrip(t *testing.T) {
	t.Parallel()
	original := map[string]any{"id": float64(42), "created_at": "2026-01-01T00:00:00Z"}

	p := &PageInfo{}
	require.NoError(t, p.SetNextCursor(original))

	require.True(t, p.HasMore)
	require.NotEmpty(t, p.NextCursor)

	p2 := &PageInfo{Cursor: p.NextCursor}
	decoded := p2.CursorValues()

	require.NotNil(t, decoded)
	require.InEpsilon(t, float64(42), decoded["id"], 0)
	require.Equal(t, "2026-01-01T00:00:00Z", decoded["created_at"])
}

// Test_CursorValuesUndecodable covers the cursors CursorValues cannot decode.
func Test_CursorValuesUndecodable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cursor string
	}{
		{name: "empty cursor", cursor: ""},
		{name: "invalid base64", cursor: "not-valid-base64!!!"},
		{name: "valid base64 but invalid JSON", cursor: "bm90LWpzb24"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &PageInfo{Cursor: tc.cursor}
			require.Nil(t, p.CursorValues())
		})
	}
}

// Test_NextCursorURL covers the next cursor link, with the default query keys
// and with custom ones, for a page with and without a next cursor.
func Test_NextCursorURL(t *testing.T) {
	t.Parallel()
	const users = "https://example.com/users"
	withKeys := func(p *PageInfo, base string) string { return p.NextCursorURLWithKeys(base, "after", "per_page") }

	for _, tc := range []struct {
		cursorURL func(*PageInfo, string) string
		name      string
		expected  string // %s stands for the next cursor
		hasMore   bool
	}{
		{(*PageInfo).NextCursorURL, "with HasMore", users + "?cursor=%s&limit=20", true},
		{(*PageInfo).NextCursorURL, "without HasMore", "", false},
		{withKeys, "with keys, has more", users + "?after=%s&per_page=20", true},
		{withKeys, "with keys, no more", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &PageInfo{Limit: 20}
			expected := tc.expected
			if tc.hasMore {
				require.NoError(t, p.SetNextCursor(map[string]any{"id": float64(42)}))
				expected = fmt.Sprintf(expected, p.NextCursor)
			}
			require.Equal(t, expected, tc.cursorURL(p, users))
		})
	}
}

func Test_SetNextCursorSetsFields(t *testing.T) {
	t.Parallel()
	p := &PageInfo{Limit: 10}
	require.NoError(t, p.SetNextCursor(map[string]any{"id": float64(1)}))
	require.True(t, p.HasMore)
	require.NotEmpty(t, p.NextCursor)
}

// --- Middleware handler tests ---

// paginateTestApp returns an app that runs New(cfg...) in front of GET /, which
// answers with the PageInfo the middleware stored on the request context as
// JSON, or with 400 when it stored none.
func paginateTestApp(cfg ...Config) *fiber.App {
	app := fiber.New()
	app.Use(New(cfg...))
	app.Get("/", pageInfoHandler)
	return app
}

// pageInfoHandler answers with the PageInfo on the request context as JSON, or
// with 400 when there is none.
func pageInfoHandler(c fiber.Ctx) error {
	pageInfo, ok := FromContext(c)
	if !ok {
		return fiber.ErrBadRequest
	}
	return c.JSON(pageInfo)
}

// paginateResponse is what Test_PaginateWithQueries' handler derives from the
// PageInfo, including the page links it builds from the request's base URL.
type paginateResponse struct {
	NextPageURL     string      `json:"next_page_url"`
	PreviousPageURL string      `json:"prev_page_url"`
	Sort            []SortField `json:"sort"`
	Page            int         `json:"page"`
	Limit           int         `json:"limit"`
	Offset          int         `json:"offset"`
	Start           int         `json:"start"`
}

func Test_PaginateWithQueries(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{DefaultSort: "id"}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := FromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}

		return c.JSON(paginateResponse{
			Page:            pageInfo.Page,
			Limit:           pageInfo.Limit,
			Offset:          pageInfo.Offset,
			Start:           pageInfo.Start(),
			Sort:            pageInfo.Sort,
			NextPageURL:     pageInfo.NextPageURL(c.BaseURL()),
			PreviousPageURL: pageInfo.PreviousPageURL(c.BaseURL()),
		})
	})

	resp := apptest.Expect(t, app, "/?page=2&limit=20", fiber.StatusOK)
	var body paginateResponse
	require.NoError(t, json.Unmarshal(apptest.BodyBytes(t, resp), &body))

	require.Equal(t, 2, body.Page)
	require.Equal(t, 20, body.Limit)
	require.Equal(t, 0, body.Offset)
	require.Equal(t, 20, body.Start)
	require.Equal(t, "http://example.com?limit=20&page=3", body.NextPageURL)
	require.Equal(t, "http://example.com?limit=20&page=1", body.PreviousPageURL)
	require.Equal(t, []SortField{{Field: "id", Order: ASC}}, body.Sort)
}

// Test_PaginateRequests covers what the middleware reads from the query (page,
// limit, offset, sort and cursor), the configuration that renames, defaults or
// bounds those parameters and the PageInfo a handler then finds on the request
// context.
func Test_PaginateRequests(t *testing.T) {
	t.Parallel()
	idAsc := []SortField{{Field: "id", Order: ASC}}
	nameAsc := []SortField{{Field: "name", Order: ASC}}
	nameThenDateDesc := []SortField{{Field: "name", Order: ASC}, {Field: "date", Order: DESC}}
	nameThenIDDesc := []SortField{{Field: "name", Order: ASC}, {Field: "id", Order: DESC}}
	cursor42 := base64.RawURLEncoding.EncodeToString([]byte(`{"id":42}`))
	cursor1 := base64.RawURLEncoding.EncodeToString([]byte(`{"id":1}`))
	byID := []Config{{DefaultSort: "id"}}
	edge := []Config{{DefaultSort: "id", DefaultLimit: 10}}
	maxed := []Config{{DefaultSort: "id", DefaultLimit: 10, MaxLimit: 50}}
	sorted := []Config{{SortKey: "sort", DefaultSort: "id", AllowedSorts: []string{"id", "name", "date"}}}

	for _, tc := range []struct {
		name   string
		target string
		cfg    []Config // none: the middleware is created without a config
		want   PageInfo
		start  int // want.Start()
	}{
		{"offset", "/?offset=20&limit=20", nil, PageInfo{Page: 1, Limit: 20, Offset: 20, Sort: idAsc}, 20},
		{"defaults when no queries", "/", nil, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{"defaults when no page", "/?limit=20", nil, PageInfo{Page: 1, Limit: 20, Sort: idAsc}, 0},
		{"defaults when no limit", "/?page=2", nil, PageInfo{Page: 2, Limit: 10, Sort: idAsc}, 10},
		{
			"config default page and default limit", "/",
			[]Config{{DefaultPage: 100, DefaultLimit: DefaultMaxLimit, DefaultSort: "name"}},
			PageInfo{Page: 100, Limit: DefaultMaxLimit, Sort: nameAsc},
			9900,
		},
		{
			"config page key and limit key", "/?site=2&size=5",
			[]Config{{PageKey: "site", LimitKey: "size", DefaultSort: "id"}},
			PageInfo{Page: 2, Limit: 5, Sort: idAsc},
			5,
		},
		{
			"negative default page and default limit values", "/",
			[]Config{{DefaultPage: -1, DefaultLimit: -1, DefaultSort: "id"}},
			PageInfo{Page: 1, Limit: 10, Sort: idAsc},
			0,
		},
		{"Negative page", "/?page=-1", edge, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{"Page zero", "/?page=0", edge, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{"Negative limit", "/?limit=-10", edge, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{"Limit zero", "/?limit=0", edge, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{"Limit exceeds max", "/?limit=200", edge, PageInfo{Page: 1, Limit: DefaultMaxLimit, Sort: idAsc}, 0},
		{"Limit within custom max", "/?limit=40", maxed, PageInfo{Page: 1, Limit: 40, Sort: idAsc}, 0},
		{"Limit exceeds custom max", "/?limit=200", maxed, PageInfo{Page: 1, Limit: 50, Sort: idAsc}, 0},
		{"Default Sort", "/", sorted, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{"Single Sort", "/?sort=name", sorted, PageInfo{Page: 1, Limit: 10, Sort: nameAsc}, 0},
		{"Multiple Sort", "/?sort=name,-date", sorted, PageInfo{Page: 1, Limit: 10, Sort: nameThenDateDesc}, 0},
		{"Invalid Sort", "/?sort=invalid", sorted, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{"no cursor falls back to page mode", "/?page=3&limit=15", byID, PageInfo{Page: 3, Limit: 15, Sort: idAsc}, 30},
		{
			"cursor with limit", "/?cursor=" + cursor42 + "&limit=20", byID,
			PageInfo{Cursor: cursor42, Limit: 20, Sort: idAsc},
			0,
		},
		{
			"cursor takes priority over page", "/?cursor=" + cursor42 + "&page=5&limit=10", nil,
			PageInfo{Cursor: cursor42, Limit: 10, Sort: idAsc}, // no Page
			0,
		},
		{"empty cursor is the first page", "/?cursor=&limit=10", nil, PageInfo{Page: 1, Limit: 10, Sort: idAsc}, 0},
		{
			"cursor with sort", "/?cursor=" + cursor42 + "&sort=name,-id",
			[]Config{{SortKey: "sort", DefaultSort: "id", AllowedSorts: []string{"id", "name"}}},
			PageInfo{Cursor: cursor42, Limit: 10, Sort: nameThenIDDesc},
			0,
		},
		{
			"cursor with custom key", "/?after=" + cursor1,
			[]Config{{CursorKey: "after"}},
			PageInfo{Cursor: cursor1, Limit: 10, Sort: idAsc},
			0,
		},
		{
			"cursor with param alias", "/?starting_after=" + cursor1,
			[]Config{{CursorParam: "starting_after"}},
			PageInfo{Cursor: cursor1, Limit: 10, Sort: idAsc},
			0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, paginateTestApp(tc.cfg...), tc.target, fiber.StatusOK)
			var got PageInfo
			require.NoError(t, json.Unmarshal(apptest.BodyBytes(t, resp), &got))
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.start, got.Start())
		})
	}
}

// Test_PaginateBadRequest covers the requests a handler finds no PageInfo for,
// because the middleware did not run, and the cursors the middleware refuses.
func Test_PaginateBadRequest(t *testing.T) {
	t.Parallel()
	notJSON := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	withoutNew := fiber.New()
	withoutNew.Get("/", pageInfoHandler)

	for _, tc := range []struct {
		app    *fiber.App
		name   string
		target string
	}{
		{withoutNew, "FromContext without New", "/"},
		{paginateTestApp(Config{Next: func(_ fiber.Ctx) bool { return true }}), "Next skips the middleware", "/"},
		{paginateTestApp(), "Invalid base64 cursor", "/?cursor=not-valid!!!"},
		{paginateTestApp(), "Valid base64 cursor but invalid JSON", "/?cursor=" + notJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			apptest.Expect(t, tc.app, tc.target, fiber.StatusBadRequest)
		})
	}
}

func Test_ParseSortQuery(t *testing.T) {
	t.Parallel()
	all := []string{"id", "name", "date"}
	idAsc := []SortField{{Field: "id", Order: ASC}}
	nameAsc := []SortField{{Field: "name", Order: ASC}}
	tests := []struct {
		name         string
		query        string
		allowedSorts []string
		defaultSort  string
		expected     []SortField
	}{
		{"Empty query", "", all, "id", idAsc},
		{"Single allowed field", "name", all, "id", nameAsc},
		{
			"Multiple fields with mixed order", "name,-date,id", all, "id",
			[]SortField{{Field: "name", Order: ASC}, {Field: "date", Order: DESC}, {Field: "id", Order: ASC}},
		},
		{"Disallowed field", "email,name", all, "id", nameAsc},
		{"All disallowed fields", "email,phone", all, "id", idAsc},
		{"Nil AllowedSorts falls back to default sort", "email,-phone", nil, "id", idAsc},
		{"Empty AllowedSorts falls back to default sort", "email,-phone", []string{}, "id", idAsc},
		{"Bare dash is skipped", "-", nil, "id", idAsc},
		{"Dash in comma list is skipped", "name,-,email", nil, "id", idAsc},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseSortQuery(tt.query, tt.allowedSorts, tt.defaultSort)
			require.Equal(t, tt.expected, result)
		})
	}
}

func Test_Paginate_EmptySortKey_IgnoresQuery(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{AllowedSorts: []string{"name", "id"}, DefaultSort: "id"}))
	app.Get("/", func(c fiber.Ctx) error {
		info, ok := FromContext(c)
		if !ok {
			return c.SendString("missing")
		}
		return c.SendString(strconv.Itoa(len(info.Sort)) + " " + info.Sort[0].Field + " " + string(info.Sort[0].Order))
	})

	for _, target := range []string{"/", "/?=-name"} {
		resp := apptest.Do(t, app, target)
		body := apptest.Body(t, resp)
		require.Equal(t, "1 id asc", body, "GET %s must use the default sort", target)
	}
}

// --- Benchmarks ---

// benchApp returns an app that runs New(cfg...) in front of GET /, which
// answers with the PageInfo as JSON.
func benchApp(cfg ...Config) *fiber.App {
	app := fiber.New()
	app.Use(New(cfg...))
	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, _ := FromContext(c)
		return c.JSON(pageInfo)
	})
	return app
}

func Benchmark_PaginateMiddleware(b *testing.B) {
	app := benchApp()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/?page=2&limit=20&sort=name,-date", http.NoBody)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // close error not relevant in tests
	}
}

func Benchmark_PaginateMiddlewareWithCustomConfig(b *testing.B) {
	app := benchApp(Config{
		PageKey:      "p",
		LimitKey:     "l",
		SortKey:      "s",
		DefaultPage:  1,
		DefaultLimit: 30,
		DefaultSort:  "id",
		AllowedSorts: []string{"id", "name", "date"},
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/?p=3&l=25&s=name,-id", http.NoBody)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // close error not relevant in tests
	}
}

func Benchmark_PaginateCursorMiddleware(b *testing.B) {
	app := benchApp(Config{SortKey: "sort", DefaultSort: "id", AllowedSorts: []string{"id", "name", "date"}})

	cursorJSON := `{"id":42,"created_at":"2026-01-01T00:00:00Z"}`
	cursor := base64.RawURLEncoding.EncodeToString([]byte(cursorJSON))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/?cursor="+cursor+"&limit=20&sort=name,-id", http.NoBody)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // close error not relevant in tests
	}
}
