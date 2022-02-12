package dictpool

import (
	"sort"

	"github.com/gofiber/fiber/v2/utils"
)

func (d *Dict) allocKV() *KV {
	n := len(d.D)

	if cap(d.D) > n {
		d.D = d.D[:n+1]
	} else {
		d.D = append(d.D, KV{})
	}

	return &d.D[n]
}

func (d *Dict) append(key string, value interface{}) {
	kv := d.allocKV()
	kv.Key = key
	kv.Value = value
}

func (d *Dict) indexOf(key string) int {
	n := len(d.D)

	if d.BinarySearch {
		idx := sort.Search(n, func(i int) bool {
			return key <= d.D[i].Key
		})

		if idx < n && d.D[idx].Key == key {
			return idx
		}
	} else {
		for i := 0; i < n; i++ {
			if d.D[i].Key == key {
				return i
			}
		}
	}

	return -1
}

// Len is the number of elements in the Dict.
func (d *Dict) Len() int {
	return len(d.D)
}

// Swap swaps the elements with indexes i and j.
func (d *Dict) Swap(i, j int) {
	iKey, iValue := d.D[i].Key, d.D[i].Value
	jKey, jValue := d.D[j].Key, d.D[j].Value

	d.D[i].Key, d.D[i].Value = jKey, jValue
	d.D[j].Key, d.D[j].Value = iKey, iValue
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (d *Dict) Less(i, j int) bool {
	return d.D[i].Key < d.D[j].Key
}

// Get get data from key.
func (d *Dict) Get(key string) interface{} {
	idx := d.indexOf(key)
	if idx > -1 {
		return d.D[idx].Value
	}

	return nil
}

// GetBytes get data from key.
func (d *Dict) GetBytes(key []byte) interface{} {
	return d.Get(utils.UnsafeString(key))
}

// Set set new key.
func (d *Dict) Set(key string, value interface{}) {
	idx := d.indexOf(key)
	if idx > -1 {
		kv := &d.D[idx]
		kv.Value = value
	} else {
		d.append(key, value)

		if d.BinarySearch {
			sort.Sort(d)
		}
	}
}

// SetBytes set new key.
func (d *Dict) SetBytes(key []byte, value interface{}) {
	d.Set(utils.UnsafeString(key), value)
}

// Del delete key.
func (d *Dict) Del(key string) {
	idx := d.indexOf(key)
	if idx > -1 {
		n := len(d.D) - 1
		d.Swap(idx, n)
		d.D = d.D[:n] // Remove last position
	}
}

// DelBytes delete key.
func (d *Dict) DelBytes(key []byte) {
	d.Del(utils.UnsafeString(key))
}

// Has check if key exists.
func (d *Dict) Has(key string) bool {
	return d.indexOf(key) > -1
}

// HasBytes check if key exists.
func (d *Dict) HasBytes(key []byte) bool {
	return d.Has(utils.UnsafeString(key))
}

// Reset reset dict.
func (d *Dict) Reset() {
	d.D = d.D[:0]
}

// Map convert to map.
func (d *Dict) Map(dst DictMap) {
	for i := range d.D {
		kv := &d.D[i]

		sd, ok := kv.Value.(*Dict)
		if ok {
			subDst := make(DictMap)
			sd.Map(subDst)
			dst[kv.Key] = subDst
		} else {
			dst[kv.Key] = kv.Value
		}
	}
}

// Parse convert map to Dict.
func (d *Dict) Parse(src DictMap) {
	d.Reset()

	for k, v := range src {
		sv, ok := v.(map[string]interface{})
		if ok {
			subDict := new(Dict)
			subDict.Parse(sv)
			d.append(k, subDict)
		} else {
			d.append(k, v)
		}
	}
}
