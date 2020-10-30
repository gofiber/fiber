package csrf_test

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/utils"
)

func Test_MemoryStorage(t *testing.T) {
	t.Parallel()
	key := "Hello World"

	s := csrf.NewMemoryStorage()

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
