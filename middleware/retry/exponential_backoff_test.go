package retry

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestExponentialBackoff_Retry(t *testing.T) {
	tests := []struct {
		name       string
		expBackoff *ExponentialBackoff
		f          func() error
		expErr     error
	}{
		{
			name: "With default values - successful",
			f: func() error {
				return nil
			},
		},
		{
			name: "With default values - unsuccessful",
			f: func() error {
				return fmt.Errorf("failed function")
			},
			expErr: fmt.Errorf("failed function"),
		},
		{
			name: "Successful function",
			expBackoff: &ExponentialBackoff{
				InitialInterval: 1 * time.Millisecond,
				MaxBackoffTime:  100 * time.Millisecond,
				Multiplier:      2.0,
				MaxRetryCount:   5,
			},
			f: func() error {
				return nil
			},
		},
		{
			name: "Unsuccessful function",
			expBackoff: &ExponentialBackoff{
				InitialInterval: 2 * time.Millisecond,
				MaxBackoffTime:  100 * time.Millisecond,
				Multiplier:      2.0,
				MaxRetryCount:   5,
			},
			f: func() error {
				return fmt.Errorf("failed function")
			},
			expErr: fmt.Errorf("failed function"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expBackoff == nil {
				tt.expBackoff = NewExponentialBackoff()
			}
			err := tt.expBackoff.Retry(tt.f)
			assert.Equal(t, tt.expErr, err)
		})
	}
}
