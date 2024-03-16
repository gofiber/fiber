package bind_test

import (
	"testing"

	"github.com/gofiber/fiber/v3/internal/bind"
	"github.com/stretchr/testify/require"
)

func TestTypeID(t *testing.T) {
	_, intType := bind.ValueAndTypeID(int(1))
	_, uintType := bind.ValueAndTypeID(uint(1))
	_, shouldBeIntType := bind.ValueAndTypeID(int(1))
	require.NotEqual(t, intType, uintType)
	require.Equal(t, intType, shouldBeIntType)
}
