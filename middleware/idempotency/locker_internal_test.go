package idempotency

import (
	"strconv"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

func (l *MemoryLock) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.keys)
}

// go test -run Test_MemoryLock_NoKeyLeak
func Test_MemoryLock_NoKeyLeak(t *testing.T) {
	t.Parallel()

	const keys = 1000

	l := NewMemoryLock()

	for i := 0; i < keys; i++ {
		key := strconv.Itoa(i)
		utils.AssertEqual(t, nil, l.Lock(key))
		utils.AssertEqual(t, nil, l.Unlock(key))
	}
	utils.AssertEqual(t, 0, l.size())

	// Contended keys must drain too, once the last holder released them.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < keys; j++ {
				key := strconv.Itoa(j)
				if err := l.Lock(key); err != nil {
					t.Error(err)
					return
				}
				if err := l.Unlock(key); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	utils.AssertEqual(t, 0, l.size())
}
