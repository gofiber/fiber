package binder

import (
	"errors"
	"sync"
)

// Binder errors
var (
	ErrSuitableContentNotFound = errors.New("binder: suitable content not found to parse body")
	ErrMapNotConvertable       = errors.New("binder: map is not convertable to map[string]string or map[string][]string")
)

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

func GetFromThePool[T any](pool *sync.Pool) T {
	binder, ok := pool.Get().(T)
	if !ok {
		panic(errors.New("failed to type-assert to T"))
	}

	return binder
}

func PutToThePool[T any](pool *sync.Pool, binder T) {
	pool.Put(binder)
}
