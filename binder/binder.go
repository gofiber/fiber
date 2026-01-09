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
