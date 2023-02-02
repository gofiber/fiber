package session

import (
	"sync"
)

// go:generate msgp
// msgp -file="data.go" -o="data_msgp.go" -tests=false -unexported
type data struct {
	sync.RWMutex
	Data map[string]interface{}
}

var dataPool = sync.Pool{
	New: func() interface{} {
		d := new(data)
		d.Data = make(map[string]interface{})
		return d
	},
}

func acquireData() *data {
	return dataPool.Get().(*data) //nolint:forcetypeassert // We store nothing else in the pool
}

func (d *data) Reset() {
	d.Lock()
	d.Data = make(map[string]interface{})
	d.Unlock()
}

func (d *data) Get(key string) interface{} {
	d.RLock()
	v := d.Data[key]
	d.RUnlock()
	return v
}

func (d *data) Set(key string, value interface{}) {
	d.Lock()
	d.Data[key] = value
	d.Unlock()
}

func (d *data) Delete(key string) {
	d.Lock()
	delete(d.Data, key)
	d.Unlock()
}

func (d *data) Keys() []string {
	d.Lock()
	keys := make([]string, 0, len(d.Data))
	for k := range d.Data {
		keys = append(keys, k)
	}
	d.Unlock()
	return keys
}

func (d *data) Len() int {
	return len(d.Data)
}
