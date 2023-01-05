package dictpool

import "sync"

var defaultPool = sync.Pool{
	New: func() interface{} {
		return new(Dict)
	},
}

// AcquireDict acquire new dict.
func AcquireDict() *Dict {
	return defaultPool.Get().(*Dict)
}

// ReleaseDict release dict.
func ReleaseDict(d *Dict) {
	d.Reset()
	defaultPool.Put(d)
}
