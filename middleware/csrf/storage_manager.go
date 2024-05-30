package csrf

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/memory"
	"github.com/gofiber/utils/v2"
)

// go:generate msgp
// msgp -file="storage_manager.go" -o="storage_manager_msgp.go" -tests=false -unexported
type item struct{}

//msgp:ignore manager
type storageManager struct {
	pool    sync.Pool
	memory  *memory.Storage
	storage fiber.Storage
}

func newStorageManager(storage fiber.Storage) *storageManager {
	// Create new storage handler
	storageManager := &storageManager{
		pool: sync.Pool{
			New: func() any {
				return new(item)
			},
		},
	}
	if storage != nil {
		// Use provided storage if provided
		storageManager.storage = storage
	} else {
		// Fallback too memory storage
		storageManager.memory = memory.New()
	}
	return storageManager
}

// get raw data from storage or memory
func (m *storageManager) getRaw(key string) (raw []byte, err error) {
	if m.storage != nil {
		raw, err = m.storage.Get(key)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrNotGetStorage, err.Error())
		}
	} else {
		var ok bool
		raw, ok = m.memory.Get(key).([]byte)
		if !ok {
			return nil, ErrNotGetStorage
		}
	}

	return raw, nil
}

// set data to storage or memory
func (m *storageManager) setRaw(key string, raw []byte, exp time.Duration) (err error) {
	if m.storage != nil {
		err = m.storage.Set(key, raw, exp)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrNotSetStorage, err.Error())
		}
	} else {
		// the key is crucial in crsf and sometimes a reference to another value which can be reused later(pool/unsafe values concept), so a copy is made here
		m.memory.Set(utils.CopyString(key), raw, exp)
	}

	return nil
}

// delete data from storage or memory
func (m *storageManager) delRaw(key string) (err error) {
	if m.storage != nil {
		err = m.storage.Delete(key)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrNotSetStorage, err.Error())
		}
	} else {
		m.memory.Delete(key)
	}

	return nil
}
