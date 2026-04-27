package logger

import (
	"github.com/gofiber/fiber/v3/internal/logtemplate"
)

// ErrTemplateParameterMissing indicates that a template parameter was referenced but not provided.
var ErrTemplateParameterMissing = logtemplate.ErrParameterMissing
