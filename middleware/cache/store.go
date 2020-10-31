package cache

// go:generate msgp
// msgp -file="store.go" -o="store_msgp.go" -tests=false -unexported
// don't forget to replace the msgp import path to:
// "github.com/gofiber/fiber/v2/internal/msgp"
type entry struct {
	body   []byte `msg:"body"`
	cType  []byte `msg:"cType"`
	status int    `msg:"status"`
	exp    uint64 `msg:"exp"`
}
