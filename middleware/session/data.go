package session

import "sync"

// go:generate msgp
// msgp -file="data.go" -o="data_msgp.go" -tests=false -unexported
// don't forget to replace the msgp import path to:
// "github.com/gofiber/fiber/v2/internal/msgp"
type data struct {
	d []kv
}

// go:generate msgp
type kv struct {
	k string
	v interface{}
}

var dataPool = sync.Pool{
	New: func() interface{} {
		return new(data)
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
	d.d = d.d[:0]
}

func (d *data) Get(key string) interface{} {
	idx := d.indexOf(key)
	if idx > -1 {
		return d.d[idx].v
	}
	return nil
}

func (d *data) Set(key string, value interface{}) {
	idx := d.indexOf(key)
	if idx > -1 {
		kv := &d.d[idx]
		kv.v = value
	} else {
		d.append(key, value)
	}
}

func (d *data) Delete(key string) {
	idx := d.indexOf(key)
	if idx > -1 {
		n := len(d.d) - 1
		d.swap(idx, n)
		d.d = d.d[:n]
	}
}

func (d *data) Len() int {
	return len(d.d)
}

func (d *data) swap(i, j int) {
	iKey, iValue := d.d[i].k, d.d[i].v
	jKey, jValue := d.d[j].k, d.d[j].v

	d.d[i].k, d.d[i].v = jKey, jValue
	d.d[j].k, d.d[j].v = iKey, iValue
}

func (d *data) allocPage() *kv {
	n := len(d.d)
	if cap(d.d) > n {
		d.d = d.d[:n+1]
	} else {
		d.d = append(d.d, kv{})
	}
	return &d.d[n]
}

func (d *data) append(key string, value interface{}) {
	kv := d.allocPage()
	kv.k = key
	kv.v = value
}

func (d *data) indexOf(key string) int {
	n := len(d.d)
	for i := 0; i < n; i++ {
		if d.d[i].k == key {
			return i
		}
	}
	return -1
}
