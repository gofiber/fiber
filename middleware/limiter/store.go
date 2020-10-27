package limiter

//go:generate msgp -o=store_msgp.go -tests=false -file=store.go
type Entry struct {
	Hits int
	Exp  uint64
}
