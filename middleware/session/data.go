package session

import (
	"sync"
)

type data struct {
	sync.RWMutex
	Data map[string]any
}

var dataPool = sync.Pool{
	New: func() any {
		d := new(data)
		d.Data = make(map[string]any)
		return d
	},
}

func acquireData() *data {
	return dataPool.Get().(*data)
}

func (d *data) Reset() {
	d.Lock()
	d.Data = make(map[string]any)
	d.Unlock()
}

func (d *data) Get(key string) any {
	d.RLock()
	v := d.Data[key]
	d.RUnlock()
	return v
}

func (d *data) Set(key string, value any) {
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
