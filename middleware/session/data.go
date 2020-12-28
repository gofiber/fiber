package session

import (
	"sync"
)

// go:generate msgp
// msgp -file="data.go" -o="data_msgp.go" -tests=false -unexported
// don't forget to replace the msgp import path to:
// "github.com/gofiber/fiber/v2/internal/msgp"
type data struct {
	sync.RWMutex `gotiny:"-"`
	d            map[string]interface{} `gotiny:"d"`
}

var dataPool = sync.Pool{
	New: func() interface{} {
		d := new(data)
		d.d = make(map[string]interface{})
		return d
	},
}

func acquireData() *data {
	return dataPool.Get().(*data)
}

func releaseData(d *data) {
	d.Reset()
	dataPool.Put(d)
}

func (d *data) Reset() {
	d.Lock()
	for key := range d.d {
		delete(d.d, key)
	}
	d.Unlock()
}

func (d *data) Get(key string) interface{} {
	d.RLock()
	v := d.d[key]
	d.RUnlock()
	return v
}

func (d *data) Set(key string, value interface{}) {
	d.Lock()
	d.d[key] = value
	d.Unlock()
}

func (d *data) Delete(key string) {
	d.Lock()
	delete(d.d, key)
	d.Unlock()
}

func (d *data) Len() int {
	return len(d.d)
}
