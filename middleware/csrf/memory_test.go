package csrf

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
)

func Test_MemoryStorage(t *testing.T) {
	t.Parallel()
	key := "Hello World"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newMemoryStorage(ctx)

	utils.AssertEqual(t, nil, s.Set(key, nil, time.Second))

	data, err := s.Get(key)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, []byte(key), data)

	// Check delete
	utils.AssertEqual(t, nil, s.Delete(key))
	data, err = s.Get(key)
	utils.AssertEqual(t, true, err != nil)
	utils.AssertEqual(t, "csrf: invalid key", err.Error())

	// Check expiration
	utils.AssertEqual(t, nil, s.Set(key, nil, time.Millisecond))
	time.Sleep(2 * time.Millisecond)
	data, err = s.Get(key)
	utils.AssertEqual(t, true, err != nil)
	utils.AssertEqual(t, "csrf: invalid key", err.Error())
}

func Test_MemoryStorageGC(t *testing.T) {
	t.Parallel()
	units := 50
	ctx, cancel := context.WithCancel(context.Background())
	s := &memoryStorage{
		gc:     100 * time.Millisecond,
		tokens: make(map[string]int64),
	}
	var wg sync.WaitGroup
	wg.Add(units)
	go s.collect(ctx)
	for i := 0; i < units; i++ {
		go func(wg *sync.WaitGroup, i int) {
			defer wg.Done()
			s.Set(fmt.Sprintf("Test Data %d", i), nil, 1*time.Second)
		}(&wg, i)
	}
	wg.Wait()
	time.Sleep(time.Second)
	cancel()
	utils.AssertEqual(t, 0, len(s.tokens))
}
