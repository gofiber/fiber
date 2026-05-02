package logger

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
	"github.com/stretchr/testify/require"
)

func Test_UnknownTagErrorIsAndAs(t *testing.T) {
	t.Parallel()

	err := &UnknownTagError{Tag: "method"}
	require.ErrorIs(t, err, ErrUnknownTag)

	var typed *UnknownTagError
	require.ErrorAs(t, err, &typed)
	require.Equal(t, "method", typed.Tag)
	require.EqualError(t, err, `logger: unknown template tag: "method"`)
}

func Test_TranslateBuildError(t *testing.T) {
	t.Parallel()

	got := translateBuildError(&logtemplate.UnknownTagError{Tag: "missing:value", Param: "value"})
	var typed *UnknownTagError
	require.ErrorAs(t, got, &typed)
	require.Equal(t, "missing:value", typed.Tag)
	require.Equal(t, "value", typed.Param)
	require.ErrorIs(t, got, ErrUnknownTag)

	require.NoError(t, translateBuildError(errors.New("unrelated")))
}
