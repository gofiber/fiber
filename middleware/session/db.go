package session

import (
	"sync"
)

// go:generate msgp
// msgp -file="db.go" -o="db_msgp.go" -tests=false -unexported
// don't forget to replace the msgp import path to:
// "github.com/gofiber/fiber/v2/internal/msgp"
type db struct {
	d []kv
}

// go:generate msgp
type kv struct {
	k string
	v interface{}
}

var dbPool = sync.Pool{
	New: func() interface{} {
		return new(db)
	},
}

func acquireDB() *db {
	return dbPool.Get().(*db)
}

func releaseDB(d *db) {
	d.Reset()
	dbPool.Put(d)
}

func (d *db) Reset() {
	d.d = d.d[:0]
}

func (d *db) Get(key string) interface{} {
	idx := d.indexOf(key)
	if idx > -1 {
		return d.d[idx].v
	}
	return nil
}

func (d *db) Set(key string, value interface{}) {
	idx := d.indexOf(key)
	if idx > -1 {
		kv := &d.d[idx]
		kv.v = value
	} else {
		d.append(key, value)
	}
}

func (d *db) Delete(key string) {
	idx := d.indexOf(key)
	if idx > -1 {
		n := len(d.d) - 1
		d.swap(idx, n)
		d.d = d.d[:n]
	}
}

func (d *db) Len() int {
	return len(d.d)
}

func (d *db) swap(i, j int) {
	iKey, iValue := d.d[i].k, d.d[i].v
	jKey, jValue := d.d[j].k, d.d[j].v

	d.d[i].k, d.d[i].v = jKey, jValue
	d.d[j].k, d.d[j].v = iKey, iValue
}

func (d *db) allocPage() *kv {
	n := len(d.d)
	if cap(d.d) > n {
		d.d = d.d[:n+1]
	} else {
		d.d = append(d.d, kv{})
	}
	return &d.d[n]
}

func (d *db) append(key string, value interface{}) {
	kv := d.allocPage()
	kv.k = key
	kv.v = value
}

func (d *db) indexOf(key string) int {
	n := len(d.d)
	for i := 0; i < n; i++ {
		if d.d[i].k == key {
			return i
		}
	}
	return -1
}
