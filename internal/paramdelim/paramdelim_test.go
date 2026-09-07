package paramdelim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_PathEndChars pins the shared set to the router's grammar: the six bytes
// that end a ":name", and not '#', which only the client treats as a terminator.
func Test_PathEndChars(t *testing.T) {
	t.Parallel()

	set := PathEndChars()
	for _, c := range []byte{'/', '-', '.', ':', '\\', '?'} {
		require.True(t, set[c], "%q must end a path parameter", c)
	}
	require.False(t, set['#'], "'#' is a client-only terminator")

	// The client adds '#' to what it gets back, so this must hand out a copy.
	set['#'] = true
	require.False(t, PathEndChars()['#'])
}
