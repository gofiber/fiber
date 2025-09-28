package binder

import (
	"errors"
	"mime/multipart"
	"sync"
)

// Binder errors
var (
	ErrSuitableContentNotFound = errors.New("binder: suitable content not found to parse body")
	ErrMapNotConvertible       = errors.New("binder: map is not convertible to map[string]string or map[string][]string")
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

var MsgPackBinderPool = sync.Pool{
	New: func() any {
		return &MsgPackBinding{}
	},
}

const (
	stringSliceMapDefaultCap = 8
	stringSliceMapMaxEntries = 128
)

var stringSliceMapPool = sync.Pool{
	New: func() any {
		return make(map[string][]string, stringSliceMapDefaultCap)
	},
}

const (
	fileHeaderSliceMapDefaultCap = 4
	fileHeaderSliceMapMaxEntries = 64
)

var fileHeaderSliceMapPool = sync.Pool{
	New: func() any {
		return make(map[string][]*multipart.FileHeader, fileHeaderSliceMapDefaultCap)
	},
}

// GetFromThePool retrieves a binder from the provided sync.Pool and panics if
// the stored value cannot be cast to the requested type.
func GetFromThePool[T any](pool *sync.Pool) T {
	binder, ok := pool.Get().(T)
	if !ok {
		panic(errors.New("failed to type-assert to T"))
	}

	return binder
}

// PutToThePool returns the binder to the provided sync.Pool.
func PutToThePool[T any](pool *sync.Pool, binder T) {
	pool.Put(binder)
}

func acquireStringSliceMap() map[string][]string {
	m, ok := stringSliceMapPool.Get().(map[string][]string)
	if !ok {
		panic(errors.New("failed to type-assert to map[string][]string"))
	}
	if m == nil {
		return make(map[string][]string, stringSliceMapDefaultCap)
	}
	if len(m) > 0 {
		clear(m)
	}
	return m
}

func releaseStringSliceMap(m map[string][]string) {
	if m == nil {
		return
	}
	used := len(m)
	if used > 0 {
		clear(m)
	}
	if used > stringSliceMapMaxEntries {
		return
	}
	stringSliceMapPool.Put(m)
}

func acquireFileHeaderSliceMap() map[string][]*multipart.FileHeader {
	m, ok := fileHeaderSliceMapPool.Get().(map[string][]*multipart.FileHeader)
	if !ok {
		panic(errors.New("failed to type-assert to map[string][]*multipart.FileHeader"))
	}
	if m == nil {
		return make(map[string][]*multipart.FileHeader, fileHeaderSliceMapDefaultCap)
	}
	if len(m) > 0 {
		clear(m)
	}
	return m
}

func releaseFileHeaderSliceMap(m map[string][]*multipart.FileHeader) {
	if m == nil {
		return
	}
	used := len(m)
	if used > 0 {
		clear(m)
	}
	if used > fileHeaderSliceMapMaxEntries {
		return
	}
	fileHeaderSliceMapPool.Put(m)
}
