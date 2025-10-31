package encryptcookie

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_configDefault_KeyValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid base64", func(t *testing.T) {
		t.Parallel()
		_, decErr := base64.StdEncoding.DecodeString("invalid")
		expectedErr := fmt.Errorf("failed to base64-decode key: %w", decErr).Error()
		require.PanicsWithError(t, expectedErr, func() {
			configDefault(Config{Key: "invalid"})
		})
	})

	t.Run("invalid length", func(t *testing.T) {
		t.Parallel()
		key := make([]byte, 20)
		_, err := rand.Read(key)
		require.NoError(t, err)
		invalidKey := base64.StdEncoding.EncodeToString(key)
		require.PanicsWithValue(t, ErrInvalidKeyLength, func() {
			configDefault(Config{Key: invalidKey})
		})
	})
}
