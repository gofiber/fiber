package logger

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcquireReqHeaderMapResets(t *testing.T) {
	t.Parallel()

	ptr := acquireReqHeaderMap()
	headers := *ptr

	headers["X-Test"] = []string{"value"}

	*ptr = headers
	releaseReqHeaderMap(ptr)

	ptr = acquireReqHeaderMap()
	headers = *ptr
	require.Empty(t, headers)

	*ptr = headers
	releaseReqHeaderMap(ptr)
}

func TestReleaseReqHeaderMapDropsOversized(t *testing.T) {
	t.Parallel()

	ptr := acquireReqHeaderMap()
	headers := *ptr
	firstAddr := fmt.Sprintf("%p", headers)

	for i := 0; i <= reqHeaderMapMaxEntries; i++ {
		key := "Header-" + strconv.Itoa(i)
		headers[key] = []string{"value"}
	}

	*ptr = headers
	releaseReqHeaderMap(ptr)

	ptr = acquireReqHeaderMap()
	headers = *ptr
	secondAddr := fmt.Sprintf("%p", headers)
	require.NotEqual(t, firstAddr, secondAddr)

	*ptr = headers
	releaseReqHeaderMap(ptr)
}
