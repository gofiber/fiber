package paginate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/gofiber/utils/v2"
)

// ErrCursorEncode is returned when cursor values cannot be encoded.
var ErrCursorEncode = errors.New("paginate: failed to encode cursor values")

// SortOrder represents sort order.
type SortOrder string

const (
	ASC  SortOrder = "asc"
	DESC SortOrder = "desc"
)

// SortField represents a sort field with direction.
type SortField struct {
	Field string    `json:"field"`
	Order SortOrder `json:"order"`
}

// SortOrderFromString returns a SortOrder from a string (case-insensitive).
func SortOrderFromString(s string) SortOrder {
	if utils.EqualFold(s, "desc") {
		return DESC
	}
	return ASC
}

// PageInfo contains pagination information.
type PageInfo struct {
	cursorData    map[string]any
	jsonMarshal   utils.JSONMarshal
	jsonUnmarshal utils.JSONUnmarshal
	Cursor        string      `json:"cursor,omitempty"`
	NextCursor    string      `json:"next_cursor,omitempty"`
	Sort          []SortField `json:"sort"`
	Page          int         `json:"page"`
	Limit         int         `json:"limit"`
	Offset        int         `json:"offset"`
	HasMore       bool        `json:"has_more,omitempty"`
}

// NewPageInfo creates a new PageInfo.
func NewPageInfo(page, limit, offset int, sort []SortField) *PageInfo {
	return &PageInfo{
		Page:   page,
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
	}
}

// Start returns the start index based on page/limit or offset.
func (p *PageInfo) Start() int {
	if p.Offset > 0 {
		return p.Offset
	}
	if p.Page < 1 || p.Limit < 1 {
		return 0
	}

	const maxInt = int(^uint(0) >> 1)
	if p.Page-1 > maxInt/p.Limit {
		return maxInt
	}

	return (p.Page - 1) * p.Limit
}

// SortBy adds a sort field. Chainable.
func (p *PageInfo) SortBy(field string, order SortOrder) *PageInfo {
	p.Sort = append(p.Sort, SortField{Field: field, Order: order})
	return p
}

// NextPageURLWithKeys returns the URL for the next page using custom query keys.
func (p *PageInfo) NextPageURLWithKeys(baseURL, pageKey, limitKey string) string {
	return buildPaginationURL(baseURL, pageKey, utils.FormatInt(int64(p.Page+1)), limitKey, utils.FormatInt(int64(p.Limit)))
}

// NextPageURL returns the URL for the next page.
func (p *PageInfo) NextPageURL(baseURL string) string {
	return p.NextPageURLWithKeys(baseURL, "page", "limit")
}

// PreviousPageURLWithKeys returns the URL for the previous page using custom query keys.
// Returns empty string if on page 1.
func (p *PageInfo) PreviousPageURLWithKeys(baseURL, pageKey, limitKey string) string {
	if p.Page > 1 {
		return buildPaginationURL(baseURL, pageKey, utils.FormatInt(int64(p.Page-1)), limitKey, utils.FormatInt(int64(p.Limit)))
	}
	return ""
}

// PreviousPageURL returns the URL for the previous page.
// Returns empty string if on page 1.
func (p *PageInfo) PreviousPageURL(baseURL string) string {
	return p.PreviousPageURLWithKeys(baseURL, "page", "limit")
}

// NextCursorURLWithKeys returns the URL for the next cursor page using custom query keys.
// Returns empty string if HasMore is false.
func (p *PageInfo) NextCursorURLWithKeys(baseURL, cursorKey, limitKey string) string {
	if !p.HasMore {
		return ""
	}
	return buildPaginationURL(baseURL, cursorKey, p.NextCursor, limitKey, utils.FormatInt(int64(p.Limit)))
}

// NextCursorURL returns the URL for the next cursor page.
// Returns empty string if HasMore is false.
func (p *PageInfo) NextCursorURL(baseURL string) string {
	return p.NextCursorURLWithKeys(baseURL, "cursor", "limit")
}

// buildPaginationURL parses baseURL and sets/replaces two query parameters,
// preserving any existing query string values.
func buildPaginationURL(baseURL, pageParam, pageValue, limitParam, limitValue string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	q := u.Query()
	q.Set(pageParam, pageValue)
	q.Set(limitParam, limitValue)
	u.RawQuery = q.Encode()
	return u.String()
}

// CursorValues returns the decoded cursor key-value map.
// If the cursor was parsed by the middleware, the pre-parsed data is returned.
// Otherwise it decodes the opaque cursor string.
// Returns nil if cursor is empty or invalid.
func (p *PageInfo) CursorValues() map[string]any {
	if p.cursorData != nil {
		return p.cursorData
	}

	if p.Cursor == "" {
		return nil
	}

	if len(p.Cursor) > maxCursorLen {
		return nil
	}

	data, err := base64.RawURLEncoding.DecodeString(p.Cursor)
	if err != nil {
		return nil
	}

	var values map[string]any
	unmarshal := p.jsonUnmarshal
	if unmarshal == nil {
		unmarshal = json.Unmarshal
	}
	if err := unmarshal(data, &values); err != nil {
		return nil
	}

	p.cursorData = values

	return values
}

// SetNextCursor encodes a key-value map into an opaque cursor token
// and sets both NextCursor and HasMore on the PageInfo.
func (p *PageInfo) SetNextCursor(values map[string]any) error {
	marshal := p.jsonMarshal
	if marshal == nil {
		marshal = json.Marshal
	}
	data, err := marshal(values)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCursorEncode, err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(data)
	if len(encoded) > maxCursorLen {
		return fmt.Errorf("%w: cursor token exceeds maximum length (%d)", ErrCursorEncode, maxCursorLen)
	}

	p.NextCursor = encoded
	p.HasMore = true

	return nil
}
