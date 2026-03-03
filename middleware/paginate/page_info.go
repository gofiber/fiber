package paginate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

// SortOrderFromString returns a SortOrder from a string.
func SortOrderFromString(s string) SortOrder {
	if s == "desc" {
		return DESC
	}
	return ASC
}

// PageInfo contains pagination information.
type PageInfo struct {
	cursorData map[string]any
	Cursor     string      `json:"cursor,omitempty"`
	NextCursor string      `json:"next_cursor,omitempty"`
	Sort       []SortField `json:"sort"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	HasMore    bool        `json:"has_more,omitempty"`
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
	return (p.Page - 1) * p.Limit
}

// SortBy adds a sort field. Chainable.
func (p *PageInfo) SortBy(field string, order SortOrder) *PageInfo {
	p.Sort = append(p.Sort, SortField{Field: field, Order: order})
	return p
}

// NextPageURLWithKeys returns the URL for the next page using custom query keys.
func (p *PageInfo) NextPageURLWithKeys(baseURL, pageKey, limitKey string) string {
	return fmt.Sprintf("%s?%s=%d&%s=%d", baseURL, pageKey, p.Page+1, limitKey, p.Limit)
}

// NextPageURL returns the URL for the next page.
func (p *PageInfo) NextPageURL(baseURL string) string {
	return p.NextPageURLWithKeys(baseURL, "page", "limit")
}

// PreviousPageURLWithKeys returns the URL for the previous page using custom query keys.
// Returns empty string if on page 1.
func (p *PageInfo) PreviousPageURLWithKeys(baseURL, pageKey, limitKey string) string {
	if p.Page > 1 {
		return fmt.Sprintf("%s?%s=%d&%s=%d", baseURL, pageKey, p.Page-1, limitKey, p.Limit)
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
	return fmt.Sprintf("%s?%s=%s&%s=%d", baseURL, cursorKey, p.NextCursor, limitKey, p.Limit)
}

// NextCursorURL returns the URL for the next cursor page.
// Returns empty string if HasMore is false.
func (p *PageInfo) NextCursorURL(baseURL string) string {
	return p.NextCursorURLWithKeys(baseURL, "cursor", "limit")
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

	data, err := base64.RawURLEncoding.DecodeString(p.Cursor)
	if err != nil {
		return nil
	}

	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil
	}

	p.cursorData = values

	return values
}

// SetNextCursor encodes a key-value map into an opaque cursor token
// and sets both NextCursor and HasMore on the PageInfo.
func (p *PageInfo) SetNextCursor(values map[string]any) error {
	data, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCursorEncode, err)
	}

	p.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	p.HasMore = true

	return nil
}
