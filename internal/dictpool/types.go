package dictpool

//go:generate msgp

// KV struct so it storages key/value data.
type KV struct {
	Key   string
	Value interface{}
}

// Dict dictionary as slice with better performance.
type Dict struct {
	// D slice of KV for storage the data
	D []KV

	// Use binary search to the get an item.
	// It's only useful on big heaps.
	//
	// WARNING: Increase searching performance on big heaps,
	// but whe set new items could be slowier due to the sorting.
	BinarySearch bool
}

// DictMap dictionary as map.
type DictMap map[string]interface{}
