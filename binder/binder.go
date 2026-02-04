package binder

import (
	"errors"
	"sync"
)

// Binder errors
var (
	ErrSuitableContentNotFound = errors.New("binder: suitable content not found to parse body")
	ErrMapNotConvertible       = errors.New("binder: map is not convertible to map[string]string or map[string][]string")
	ErrMapNilDestination       = errors.New("binder: map destination is nil and cannot be initialized")
	ErrInvalidDestinationValue = errors.New("binder: invalid destination value")
	ErrUnmatchedBrackets       = errors.New("unmatched brackets")
)

var errPoolTypeAssertion = errors.New("failed to type-assert to T")

// bindDataPool is a shared pool for map[string][]string used by binders
// to avoid allocations on each bind call.
var bindDataPool = sync.Pool{
	New: func() any {
		return make(map[string][]string, 8)
	},
}

// AcquireBindData retrieves a map from the pool for binding data.
// The returned map is empty and ready for use.
func AcquireBindData() map[string][]string {
	m, ok := bindDataPool.Get().(map[string][]string)
	if !ok {
		return make(map[string][]string, 8)
	}
	return m
}

// ReleaseBindData clears the map and returns it to the pool.
func ReleaseBindData(m map[string][]string) {
	clear(m)
	bindDataPool.Put(m)
}

var HeaderBinderPool = sync.Pool{
	New: func() any {
		return &HeaderBinding{}
	},
}

var RespHeaderBinderPool = sync.Pool{
	New: func() any {
		return &RespHeaderBinding{}
	},
}

var CookieBinderPool = sync.Pool{
	New: func() any {
		return &CookieBinding{}
	},
}

var QueryBinderPool = sync.Pool{
	New: func() any {
		return &QueryBinding{}
	},
}

var FormBinderPool = sync.Pool{
	New: func() any {
		return &FormBinding{}
	},
}

var URIBinderPool = sync.Pool{
	New: func() any {
		return &URIBinding{}
	},
}

var XMLBinderPool = sync.Pool{
	New: func() any {
		return &XMLBinding{}
	},
}

var JSONBinderPool = sync.Pool{
	New: func() any {
		return &JSONBinding{}
	},
}

var CBORBinderPool = sync.Pool{
	New: func() any {
		return &CBORBinding{}
	},
}

var MsgPackBinderPool = sync.Pool{
	New: func() any {
		return &MsgPackBinding{}
	},
}

// GetFromThePool retrieves a binder from the provided sync.Pool and panics if
// the stored value cannot be cast to the requested type.
func GetFromThePool[T any](pool *sync.Pool) T {
	binder, ok := pool.Get().(T)
	if !ok {
		panic(errPoolTypeAssertion)
	}

	return binder
}

// PutToThePool returns the binder to the provided sync.Pool.
func PutToThePool[T any](pool *sync.Pool, binder T) {
	pool.Put(binder)
}
