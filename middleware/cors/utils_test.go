package cors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isOriginSerializedOrNull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		origin             string
		expectedSerialized bool
		expectedNull       bool
	}{
		{
			name:               "literal null origin",
			origin:             "null",
			expectedSerialized: false,
			expectedNull:       true,
		},
		{
			name:               "valid http origin",
			origin:             "https://example.com",
			expectedSerialized: true,
			expectedNull:       false,
		},
		{
			name:               "valid origin with port",
			origin:             "http://example.com:8080",
			expectedSerialized: true,
			expectedNull:       false,
		},
		{
			name:               "invalid origin",
			origin:             "not-an-origin",
			expectedSerialized: false,
			expectedNull:       false,
		},
		{
			name:               "empty string",
			origin:             "",
			expectedSerialized: false,
			expectedNull:       false,
		},
		{
			name:               "origin with path is invalid",
			origin:             "https://example.com/path",
			expectedSerialized: false,
			expectedNull:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isSerialized, isNull := isOriginSerializedOrNull(tt.origin)
			assert.Equal(t, tt.expectedSerialized, isSerialized, "isSerialized mismatch for origin %q", tt.origin)
			assert.Equal(t, tt.expectedNull, isNull, "isNull mismatch for origin %q", tt.origin)
		})
	}
}
