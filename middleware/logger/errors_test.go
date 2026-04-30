package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_TemplateParameterMissingErrorUnwrap(t *testing.T) {
	t.Parallel()

	err := errTemplateParameterMissing("method")
	require.ErrorIs(t, err, ErrTemplateParameterMissing)

	var typed templateParameterMissingError
	require.ErrorAs(t, err, &typed)
	require.EqualError(t, err, "logger: template parameter missing: method")
}
