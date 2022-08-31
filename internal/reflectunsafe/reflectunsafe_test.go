package reflectunsafe_test

import (
	"testing"

	"github.com/gofiber/fiber/v3/internal/reflectunsafe"
	"github.com/stretchr/testify/require"
)

func TestTypeID(t *testing.T) {
	_, intType := reflectunsafe.ValueAndTypeID(int(1))
	_, uintType := reflectunsafe.ValueAndTypeID(uint(1))
	_, shouldBeIntType := reflectunsafe.ValueAndTypeID(int(1))
	require.NotEqual(t, intType, uintType)
	require.Equal(t, intType, shouldBeIntType)
}
