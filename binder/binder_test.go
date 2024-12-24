package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetAndPutToThePool(t *testing.T) {
	t.Parallel()

	// Panics in case we get from another pool
	require.Panics(t, func() {
		_ = GetFromThePool[*HeaderBinding](&CookieBinderPool)
	})

	// We get from the pool
	binder := GetFromThePool[*HeaderBinding](&HeaderBinderPool)
	PutToThePool(&HeaderBinderPool, binder)

	_ = GetFromThePool[*RespHeaderBinding](&RespHeaderBinderPool)
	_ = GetFromThePool[*QueryBinding](&QueryBinderPool)
	_ = GetFromThePool[*FormBinding](&FormBinderPool)
	_ = GetFromThePool[*URIBinding](&URIBinderPool)
	_ = GetFromThePool[*XMLBinding](&XMLBinderPool)
	_ = GetFromThePool[*JSONBinding](&JSONBinderPool)
	_ = GetFromThePool[*CBORBinding](&CBORBinderPool)
}
