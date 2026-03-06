package paginate

import (
	"encoding/base64"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

const (
	pageInfoKey contextKey = iota
)

// DefaultMaxLimit is the default maximum limit allowed.
const DefaultMaxLimit = 100

// New creates a new pagination middleware handler.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		appCfg := c.App().Config()
		maxLimit := cfg.MaxLimit
		if maxLimit < 1 {
			maxLimit = DefaultMaxLimit
		}

		limit := fiber.Query(c, cfg.LimitKey, cfg.DefaultLimit)
		if limit < 1 {
			limit = cfg.DefaultLimit
		}
		if limit > maxLimit {
			limit = maxLimit
		}

		sorts := parseSortQuery(c.Query(cfg.SortKey), cfg.AllowedSorts, cfg.DefaultSort)

		cursorRaw := c.Query(cfg.CursorKey)
		if cursorRaw == "" && cfg.CursorParam != "" {
			cursorRaw = c.Query(cfg.CursorParam)
		}

		if cursorRaw != "" {
			data, err := base64.RawURLEncoding.DecodeString(cursorRaw)
			if err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "invalid cursor")
			}
			var obj map[string]any
			if err := appCfg.JSONDecoder(data, &obj); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "invalid cursor")
			}

			pageInfo := &PageInfo{
				Limit:         limit,
				Sort:          sorts,
				Cursor:        cursorRaw,
				cursorData:    obj,
				jsonMarshal:   appCfg.JSONEncoder,
				jsonUnmarshal: appCfg.JSONDecoder,
			}
			fiber.StoreInContext(c, pageInfoKey, pageInfo)
			return c.Next()
		}

		page := max(fiber.Query(c, cfg.PageKey, cfg.DefaultPage), 1)
		offset := max(fiber.Query(c, cfg.OffsetKey, 0), 0)

		pageInfo := NewPageInfo(page, limit, offset, sorts)
		pageInfo.jsonMarshal = appCfg.JSONEncoder
		pageInfo.jsonUnmarshal = appCfg.JSONDecoder
		fiber.StoreInContext(c, pageInfoKey, pageInfo)
		return c.Next()
	}
}

// FromContext returns the PageInfo from the request context.
// It accepts fiber.CustomCtx, fiber.Ctx, *fasthttp.RequestCtx, and context.Context.
// Returns nil and false if no PageInfo is stored.
func FromContext(ctx any) (*PageInfo, bool) {
	return fiber.ValueFromContext[*PageInfo](ctx, pageInfoKey)
}

func parseSortQuery(query string, allowedSorts []string, defaultSort string) []SortField {
	if query == "" {
		return []SortField{{Field: defaultSort, Order: ASC}}
	}

	fields := strings.Split(query, ",")
	sortFields := make([]SortField, 0, len(fields))

	for _, field := range fields {
		field = utils.TrimSpace(field)
		if field == "" {
			continue
		}
		order := ASC
		if strings.HasPrefix(field, "-") {
			order = DESC
			field = utils.TrimSpace(field[1:])
		}
		if field == "" {
			continue
		}
		if len(allowedSorts) == 0 || slices.Contains(allowedSorts, field) {
			sortFields = append(sortFields, SortField{Field: field, Order: order})
		}
	}

	if len(sortFields) == 0 {
		return []SortField{{Field: defaultSort, Order: ASC}}
	}

	return sortFields
}
