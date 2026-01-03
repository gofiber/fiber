package fiber

import (
	"fmt"
)

func adaptHandlers(path string, raw ...any) []Handler {
	if len(raw) == 0 {
		return nil
	}

	handlers := make([]Handler, 0, len(raw))
	for _, h := range raw {
		if h == nil {
			panic(fmt.Sprintf("nil handler in route: %s\n", path))
		}

		switch v := h.(type) {
		case Handler:
			handlers = append(handlers, v)
		case []Handler:
			for _, inner := range v {
				if inner == nil {
					panic(fmt.Sprintf("nil handler in route: %s\n", path))
				}
				handlers = append(handlers, inner)
			}
		case []any:
			handlers = append(handlers, adaptHandlers(path, v...)...)
		default:
			panic(fmt.Sprintf("invalid handler type: %T", h))
		}
	}

	return handlers
}
