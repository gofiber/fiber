package paginate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type paginateResponse struct {
	NextPageURL     string      `json:"next_page_url"`
	PreviousPageURL string      `json:"prev_page_url"`
	Sort            []SortField `json:"sort"`
	Page            int         `json:"page"`
	Limit           int         `json:"limit"`
	Offset          int         `json:"offset"`
	Start           int         `json:"start"`
}

type cursorResponse struct {
	Cursor     string      `json:"cursor"`
	NextCursor string      `json:"next_cursor"`
	Sort       []SortField `json:"sort"`
	Limit      int         `json:"limit"`
	HasMore    bool        `json:"has_more"`
}

// --- Config tests ---

func Test_ConfigDefault(t *testing.T) {
	t.Parallel()

	cfg := configDefault()
	require.Equal(t, "page", cfg.PageKey)
	require.Equal(t, 1, cfg.DefaultPage)
	require.Equal(t, "limit", cfg.LimitKey)
	require.Equal(t, 10, cfg.DefaultLimit)
}

func Test_ConfigOverride(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		PageKey:      "p",
		LimitKey:     "l",
		DefaultPage:  5,
		DefaultLimit: 50,
	})
	require.Equal(t, "p", cfg.PageKey)
	require.Equal(t, "l", cfg.LimitKey)
	require.Equal(t, 5, cfg.DefaultPage)
	require.Equal(t, 50, cfg.DefaultLimit)
}

func Test_ConfigDefaultCursorKey(t *testing.T) {
	t.Parallel()

	cfg := configDefault()
	require.Equal(t, "cursor", cfg.CursorKey)
}

func Test_ConfigOverrideCursorKey(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		CursorKey:   "after",
		CursorParam: "starting_after",
	})
	require.Equal(t, "after", cfg.CursorKey)
	require.Equal(t, "starting_after", cfg.CursorParam)
}

func Test_ConfigNegativeDefaults(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		DefaultPage:  -1,
		DefaultLimit: -1,
	})
	require.Equal(t, 1, cfg.DefaultPage)
	require.Equal(t, 10, cfg.DefaultLimit)
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
		{"Page 2, limit 10", PageInfo{Page: 2, Limit: 10}, 10},
		{"Page 3, limit 20", PageInfo{Page: 3, Limit: 20}, 40},
		{"With offset", PageInfo{Page: 2, Limit: 10, Offset: 25}, 25},
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

func Test_PageInfoNextPageURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		expected string
		pageInfo PageInfo
	}{
		{
			name:     "Middle page",
			baseURL:  "https://example.com/users",
			expected: "https://example.com/users?limit=10&page=3",
			pageInfo: PageInfo{Page: 2, Limit: 10},
		},
		{
			name:     "First page",
			baseURL:  "https://example.com/users",
			expected: "https://example.com/users?limit=20&page=2",
			pageInfo: PageInfo{Page: 1, Limit: 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.pageInfo.NextPageURL(tt.baseURL))
		})
	}
}

func Test_PageInfoPreviousPageURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		expected string
		pageInfo PageInfo
	}{
		{
			name:     "Middle page",
			baseURL:  "https://example.com/users",
			expected: "https://example.com/users?limit=10&page=1",
			pageInfo: PageInfo{Page: 2, Limit: 10},
		},
		{
			name:     "First page returns empty",
			baseURL:  "https://example.com/users",
			expected: "",
			pageInfo: PageInfo{Page: 1, Limit: 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.pageInfo.PreviousPageURL(tt.baseURL))
		})
	}
}

func Test_PageInfoStartCursorMode(t *testing.T) {
	t.Parallel()

	// In cursor mode, Page is 0 (not set). Start() should return 0, not negative.
	p := &PageInfo{Page: 0, Limit: 20}
	require.Equal(t, 0, p.Start())
}

func Test_PageInfoNextPageURLWithExistingQueryParams(t *testing.T) {
	t.Parallel()

	p := PageInfo{Page: 2, Limit: 10}
	result := p.NextPageURL("https://example.com/users?filter=active")
	require.Contains(t, result, "filter=active")
	require.Contains(t, result, "page=3")
	require.Contains(t, result, "limit=10")
}

func Test_PageInfoPreviousPageURLWithExistingQueryParams(t *testing.T) {
	t.Parallel()

	p := PageInfo{Page: 3, Limit: 10}
	result := p.PreviousPageURL("https://example.com/users?filter=active")
	require.Contains(t, result, "filter=active")
	require.Contains(t, result, "page=2")
	require.Contains(t, result, "limit=10")
}

func Test_PageInfoCursorFields(t *testing.T) {
	t.Parallel()

	p := &PageInfo{
		Cursor:     "abc123",
		HasMore:    true,
		NextCursor: "def456",
	}

	require.Equal(t, "abc123", p.Cursor)
	require.True(t, p.HasMore)
	require.Equal(t, "def456", p.NextCursor)
}

func Test_CursorValuesRoundTrip(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		"id":         float64(42),
		"created_at": "2026-01-01T00:00:00Z",
	}

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

func Test_CursorValuesEmptyCursor(t *testing.T) {
	t.Parallel()

	p := &PageInfo{Cursor: ""}
	require.Nil(t, p.CursorValues())
}

func Test_CursorValuesInvalidBase64(t *testing.T) {
	t.Parallel()

	p := &PageInfo{Cursor: "not-valid-base64!!!"}
	require.Nil(t, p.CursorValues())
}

func Test_CursorValuesInvalidJSON(t *testing.T) {
	t.Parallel()

	p := &PageInfo{Cursor: "bm90LWpzb24"}
	require.Nil(t, p.CursorValues())
}

func Test_NextCursorURL(t *testing.T) {
	t.Parallel()

	t.Run("with HasMore", func(t *testing.T) {
		t.Parallel()
		p := &PageInfo{Limit: 20}
		require.NoError(t, p.SetNextCursor(map[string]any{"id": float64(42)}))

		url := p.NextCursorURL("https://example.com/users")
		expected := fmt.Sprintf("https://example.com/users?cursor=%s&limit=20", p.NextCursor)
		require.Equal(t, expected, url)
	})

	t.Run("without HasMore", func(t *testing.T) {
		t.Parallel()
		p := &PageInfo{Limit: 20}
		require.Empty(t, p.NextCursorURL("https://example.com/users"))
	})
}

func Test_SetNextCursorSetsFields(t *testing.T) {
	t.Parallel()

	p := &PageInfo{Limit: 10}
	require.NoError(t, p.SetNextCursor(map[string]any{"id": float64(1)}))
	require.True(t, p.HasMore)
	require.NotEmpty(t, p.NextCursor)
}

// --- Middleware handler tests ---

func Test_PaginateWithQueries(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		DefaultSort: "id",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?page=2&limit=20", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 2, body.Page)
	require.Equal(t, 20, body.Limit)
	require.Equal(t, 0, body.Offset)
	require.Equal(t, 20, body.Start)
	require.Equal(t, "http://example.com?limit=20&page=3", body.NextPageURL)
	require.Equal(t, "http://example.com?limit=20&page=1", body.PreviousPageURL)
	require.Equal(t, []SortField{{Field: "id", Order: ASC}}, body.Sort)
}

func Test_PaginateWithOffset(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:   pageInfo.Page,
			Limit:  pageInfo.Limit,
			Offset: pageInfo.Offset,
			Start:  pageInfo.Start(),
			Sort:   pageInfo.Sort,
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?offset=20&limit=20", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 1, body.Page)
	require.Equal(t, 20, body.Limit)
	require.Equal(t, 20, body.Offset)
	require.Equal(t, 20, body.Start)
}

func Test_PaginateCheckDefaultsWhenNoQueries(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:   pageInfo.Page,
			Limit:  pageInfo.Limit,
			Offset: pageInfo.Offset,
			Start:  pageInfo.Start(),
			Sort:   pageInfo.Sort,
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 1, body.Page)
	require.Equal(t, 10, body.Limit)
	require.Equal(t, 0, body.Offset)
	require.Equal(t, 0, body.Start)
	require.Equal(t, []SortField{{Field: "id", Order: ASC}}, body.Sort)
}

func Test_PaginateCheckDefaultsWhenNoPage(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:  pageInfo.Page,
			Limit: pageInfo.Limit,
			Start: pageInfo.Start(),
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?limit=20", http.NoBody))
	require.NoError(t, err)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 1, body.Page)
	require.Equal(t, 20, body.Limit)
	require.Equal(t, 0, body.Start)
}

func Test_PaginateCheckDefaultsWhenNoLimit(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:  pageInfo.Page,
			Limit: pageInfo.Limit,
			Start: pageInfo.Start(),
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?page=2", http.NoBody))
	require.NoError(t, err)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 2, body.Page)
	require.Equal(t, 10, body.Limit)
	require.Equal(t, 10, body.Start)
}

func Test_PaginateConfigDefaultPageDefaultLimit(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		DefaultPage:  100,
		DefaultLimit: MaxLimit,
		DefaultSort:  "name",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:  pageInfo.Page,
			Limit: pageInfo.Limit,
			Start: pageInfo.Start(),
			Sort:  pageInfo.Sort,
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 100, body.Page)
	require.Equal(t, MaxLimit, body.Limit)
	require.Equal(t, 9900, body.Start)
	require.Equal(t, []SortField{{Field: "name", Order: ASC}}, body.Sort)
}

func Test_PaginateConfigPageKeyLimitKey(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		PageKey:     "site",
		LimitKey:    "size",
		DefaultSort: "id",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:  pageInfo.Page,
			Limit: pageInfo.Limit,
			Start: pageInfo.Start(),
			Sort:  pageInfo.Sort,
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?site=2&size=5", http.NoBody))
	require.NoError(t, err)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 2, body.Page)
	require.Equal(t, 5, body.Limit)
	require.Equal(t, 5, body.Start)
}

func Test_PaginateNegativeDefaultPageDefaultLimitValues(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		DefaultPage:  -1,
		DefaultLimit: -1,
		DefaultSort:  "id",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:  pageInfo.Page,
			Limit: pageInfo.Limit,
			Start: pageInfo.Start(),
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	var body paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 1, body.Page)
	require.Equal(t, 10, body.Limit)
	require.Equal(t, 0, body.Start)
}

func Test_PaginateFromContextWithoutNew(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/", func(c fiber.Ctx) error {
		_, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(nil)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_PaginateNextSkip(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		_, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(nil)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_PaginateEdgeCases(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		DefaultSort:  "id",
		DefaultLimit: 10,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(pageInfo)
	})

	testCases := []struct {
		name          string
		url           string
		expectedPage  int
		expectedLimit int
	}{
		{"Negative page", "/?page=-1", 1, 10},
		{"Page zero", "/?page=0", 1, 10},
		{"Negative limit", "/?limit=-10", 1, 10},
		{"Limit zero", "/?limit=0", 1, 10},
		{"Limit exceeds max", "/?limit=200", 1, MaxLimit},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, tc.url, http.NoBody))
			require.NoError(t, err)
			require.Equal(t, 200, resp.StatusCode)

			var result PageInfo
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
			require.Equal(t, tc.expectedPage, result.Page)
			require.Equal(t, tc.expectedLimit, result.Limit)
		})
	}
}

func Test_PaginateWithMultipleSorting(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		SortKey:      "sort",
		DefaultSort:  "id",
		AllowedSorts: []string{"id", "name", "date"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Sort: pageInfo.Sort,
		})
	})

	testCases := []struct {
		name         string
		url          string
		expectedSort []SortField
	}{
		{"Default Sort", "/", []SortField{{Field: "id", Order: ASC}}},
		{"Single Sort", "/?sort=name", []SortField{{Field: "name", Order: ASC}}},
		{"Multiple Sort", "/?sort=name,-date", []SortField{{Field: "name", Order: ASC}, {Field: "date", Order: DESC}}},
		{"Invalid Sort", "/?sort=invalid", []SortField{{Field: "id", Order: ASC}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tc.url, http.NoBody))
			require.NoError(t, err)

			var result paginateResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
			require.Equal(t, tc.expectedSort, result.Sort)
		})
	}
}

func Test_ParseSortQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		query        string
		allowedSorts []string
		defaultSort  string
		expected     []SortField
	}{
		{
			"Empty query",
			"",
			[]string{"id", "name", "date"},
			"id",
			[]SortField{{Field: "id", Order: ASC}},
		},
		{
			"Single allowed field",
			"name",
			[]string{"id", "name", "date"},
			"id",
			[]SortField{{Field: "name", Order: ASC}},
		},
		{
			"Multiple fields with mixed order",
			"name,-date,id",
			[]string{"id", "name", "date"},
			"id",
			[]SortField{
				{Field: "name", Order: ASC},
				{Field: "date", Order: DESC},
				{Field: "id", Order: ASC},
			},
		},
		{
			"Disallowed field",
			"email,name",
			[]string{"id", "name", "date"},
			"id",
			[]SortField{{Field: "name", Order: ASC}},
		},
		{
			"All disallowed fields",
			"email,phone",
			[]string{"id", "name", "date"},
			"id",
			[]SortField{{Field: "id", Order: ASC}},
		},
		{
			"Nil AllowedSorts allows all fields",
			"email,-phone",
			nil,
			"id",
			[]SortField{
				{Field: "email", Order: ASC},
				{Field: "phone", Order: DESC},
			},
		},
		{
			"Bare dash is skipped",
			"-",
			nil,
			"id",
			[]SortField{{Field: "id", Order: ASC}},
		},
		{
			"Dash in comma list is skipped",
			"name,-,email",
			nil,
			"id",
			[]SortField{
				{Field: "name", Order: ASC},
				{Field: "email", Order: ASC},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseSortQuery(tt.query, tt.allowedSorts, tt.defaultSort)
			require.Equal(t, tt.expected, result)
		})
	}
}

// --- Cursor tests ---

func Test_PaginateWithCursor(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		DefaultSort: "id",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(cursorResponse{
			Cursor: pageInfo.Cursor,
			Limit:  pageInfo.Limit,
			Sort:   pageInfo.Sort,
		})
	})

	cursorJSON := `{"id":42}`
	cursor := base64.RawURLEncoding.EncodeToString([]byte(cursorJSON))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?cursor="+cursor+"&limit=20", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var result cursorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, cursor, result.Cursor)
	require.Equal(t, 20, result.Limit)
}

func Test_PaginateCursorPriorityOverPage(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(pageInfo)
	})

	cursorJSON := `{"id":42}`
	cursor := base64.RawURLEncoding.EncodeToString([]byte(cursorJSON))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?cursor="+cursor+"&page=5&limit=10", http.NoBody))
	require.NoError(t, err)

	var result PageInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, cursor, result.Cursor)
	require.Equal(t, 0, result.Page)
}

func Test_PaginateEmptyCursorIsFirstPage(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(pageInfo)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?cursor=&limit=10", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var result PageInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Empty(t, result.Cursor)
}

func Test_PaginateInvalidCursorReturns400(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, _ := PageInfoFromContext(c)
		return c.JSON(pageInfo)
	})

	testCases := []struct {
		name   string
		cursor string
	}{
		{"Invalid base64", "not-valid!!!"},
		{"Valid base64 but invalid JSON", base64.RawURLEncoding.EncodeToString([]byte("not-json"))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?cursor="+tc.cursor, http.NoBody))
			require.NoError(t, err)
			require.Equal(t, 400, resp.StatusCode)
		})
	}
}

func Test_PaginateCursorWithSort(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		SortKey:      "sort",
		DefaultSort:  "id",
		AllowedSorts: []string{"id", "name"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(cursorResponse{
			Cursor: pageInfo.Cursor,
			Sort:   pageInfo.Sort,
		})
	})

	cursorJSON := `{"id":42}`
	cursor := base64.RawURLEncoding.EncodeToString([]byte(cursorJSON))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?cursor="+cursor+"&sort=name,-id", http.NoBody))
	require.NoError(t, err)

	var result cursorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, []SortField{{Field: "name", Order: ASC}, {Field: "id", Order: DESC}}, result.Sort)
}

func Test_PaginateCursorWithCustomKey(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		CursorKey: "after",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(cursorResponse{
			Cursor: pageInfo.Cursor,
			Limit:  pageInfo.Limit,
		})
	})

	cursorJSON := `{"id":1}`
	cursor := base64.RawURLEncoding.EncodeToString([]byte(cursorJSON))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?after="+cursor, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var result cursorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, cursor, result.Cursor)
}

func Test_PaginateCursorWithParamAlias(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		CursorParam: "starting_after",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(cursorResponse{
			Cursor: pageInfo.Cursor,
		})
	})

	cursorJSON := `{"id":1}`
	cursor := base64.RawURLEncoding.EncodeToString([]byte(cursorJSON))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?starting_after="+cursor, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var result cursorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, cursor, result.Cursor)
}

func Test_PaginateNoCursorFallsBackToPageMode(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		DefaultSort: "id",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, ok := PageInfoFromContext(c)
		if !ok {
			return fiber.ErrBadRequest
		}
		return c.JSON(paginateResponse{
			Page:  pageInfo.Page,
			Limit: pageInfo.Limit,
			Start: pageInfo.Start(),
		})
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/?page=3&limit=15", http.NoBody))
	require.NoError(t, err)

	var result paginateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, 3, result.Page)
	require.Equal(t, 15, result.Limit)
	require.Equal(t, 30, result.Start)
}

// --- Benchmarks ---

func Benchmark_PaginateMiddleware(b *testing.B) {
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, _ := PageInfoFromContext(c)
		return c.JSON(pageInfo)
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/?page=2&limit=20&sort=name,-date", http.NoBody)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}

func Benchmark_PaginateMiddlewareWithCustomConfig(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		PageKey:      "p",
		LimitKey:     "l",
		SortKey:      "s",
		DefaultPage:  1,
		DefaultLimit: 30,
		DefaultSort:  "id",
		AllowedSorts: []string{"id", "name", "date"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, _ := PageInfoFromContext(c)
		return c.JSON(pageInfo)
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/?p=3&l=25&s=name,-id", http.NoBody)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}

func Benchmark_PaginateCursorMiddleware(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		SortKey:      "sort",
		DefaultSort:  "id",
		AllowedSorts: []string{"id", "name", "date"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		pageInfo, _ := PageInfoFromContext(c)
		return c.JSON(pageInfo)
	})

	cursorJSON := `{"id":42,"created_at":"2026-01-01T00:00:00Z"}`
	cursor := base64.RawURLEncoding.EncodeToString([]byte(cursorJSON))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/?cursor="+cursor+"&limit=20&sort=name,-id", http.NoBody)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}
