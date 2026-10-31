// Package storagetest provides the fiber.Storage fakes that the tests of the
// root package and of the storage-backed middleware keep needing: a map-backed
// storage with a per-operation error table and a log of the calls it served,
// and a storage that always fails. The fakes do not expire entries; Call.TTL
// records the expiration each Set was given.
//
// Like internal/apptest, this package does not import fiber, so the in-package
// tests of the fiber package itself can use it without an import cycle. Every
// type has the full fiber.Storage method set, including the *WithContext
// variants, Reset and Close.
package storagetest

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// The operations whose errors Memory.SetErr configures and whose calls
// Memory.Calls reports.
const (
	OpGet    = "get"
	OpSet    = "set"
	OpDelete = "del"
	OpReset  = "reset"
)

// markerKey is the context key under which Marked keeps its label.
type markerKey struct{}

// Marked returns a background context that carries label. Memory reports it as
// Call.Marker for every call that receives the context, so a test can tell
// which context reached the storage.
func Marked(label string) context.Context {
	return context.WithValue(context.Background(), markerKey{}, label)
}

// MarkedCanceled is Marked for a context that is already canceled.
func MarkedCanceled(label string) context.Context {
	ctx, cancel := context.WithCancel(Marked(label))
	cancel()
	return ctx
}

// Call is one operation a Memory served.
type Call struct {
	// Op is OpGet, OpSet, OpDelete or OpReset.
	Op string
	// Key is the key the call named, empty for OpReset.
	Key string
	// Marker is the label of the Marked context the call received, or empty.
	Marker string
	// Value is a copy of the value an OpSet stored, nil for the other operations.
	Value []byte
	// TTL is the expiration an OpSet was given, which Memory otherwise ignores.
	TTL time.Duration
	// Canceled reports whether the call's context was already canceled. The
	// methods without a context use a background context, which never is.
	Canceled bool
}

// Memory is a map-backed storage for tests. Its zero value is ready to use.
//
// Values are copied on Set and on Get, and expirations are ignored. An
// operation fails with the error SetErr configured for its key, or else for
// the key "*" that stands for any key. Every call is recorded, failed or not.
//
// The methods lock Mu. Tests may read and write Data directly while no request
// is in flight, or while holding Mu.
type Memory struct {
	// Data holds the stored values by key.
	Data map[string][]byte
	// errs maps "<op>|<key>" to the error that operation returns, guarded by Mu.
	errs map[string]error
	// calls is the call log, guarded by Mu.
	calls []Call
	// Mu guards Data, the error table and the call log.
	Mu sync.RWMutex
	// HonorContext makes every *WithContext method fail with the error of an
	// already canceled or expired context before it does anything else.
	HonorContext bool
}

// New returns an empty Memory whose Data map is made, for tests that fill Data
// directly.
func New() *Memory {
	return &Memory{Data: make(map[string][]byte)}
}

// SetErr makes op on key fail with err, key "*" making it fail on every key,
// or succeed again when err is nil. It returns m to chain, and panics when op
// is none of the Op constants, which would otherwise inject nothing.
func (m *Memory) SetErr(op, key string, err error) *Memory {
	if !slices.Contains([]string{OpGet, OpSet, OpDelete, OpReset}, op) {
		panic("storagetest: unknown operation " + strconv.Quote(op))
	}
	m.Mu.Lock()
	defer m.Mu.Unlock()
	if err == nil {
		delete(m.errs, op+"|"+key)
		return m
	}
	if m.errs == nil {
		m.errs = make(map[string]error)
	}
	m.errs[op+"|"+key] = err
	return m
}

// ClearErrs removes every error SetErr configured.
func (m *Memory) ClearErrs() {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	clear(m.errs)
}

// Holds reports whether m stores a value under key.
func (m *Memory) Holds(key string) bool {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	_, ok := m.Data[key]
	return ok
}

// Keys returns the stored keys in sorted order.
func (m *Memory) Keys() []string {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	return slices.Sorted(maps.Keys(m.Data))
}

// Calls returns the recorded calls of op in order.
func (m *Memory) Calls(op string) []Call {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	var out []Call
	for _, c := range m.calls {
		if c.Op == op {
			out = append(out, c)
		}
	}
	return out
}

// CallKeys returns the keys of the recorded calls of op in order.
func (m *Memory) CallKeys(op string) []string {
	var keys []string
	for _, c := range m.Calls(op) {
		keys = append(keys, c.Key)
	}
	return keys
}

// ClearCalls forgets the recorded calls and keeps the data.
func (m *Memory) ClearCalls() {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.calls = nil
}

// begin records c and returns the error it has to fail with. The caller holds Mu.
func (m *Memory) begin(ctx context.Context, c *Call) error {
	ctxErr := ctx.Err()
	c.Canceled = errors.Is(ctxErr, context.Canceled)
	if label, ok := ctx.Value(markerKey{}).(string); ok {
		c.Marker = label
	}
	m.calls = append(m.calls, *c)

	if m.HonorContext && ctxErr != nil {
		return ctxErr //nolint:wrapcheck // the fake returns the context error verbatim
	}
	if len(m.errs) == 0 {
		return nil
	}
	if err := m.errs[c.Op+"|"+c.Key]; err != nil {
		return err
	}
	return m.errs[c.Op+"|*"]
}

// GetWithContext returns a copy of the value under key, or nil when there is none.
func (m *Memory) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	if err := m.begin(ctx, &Call{Op: OpGet, Key: key}); err != nil {
		return nil, err
	}
	return append([]byte(nil), m.Data[key]...), nil
}

// Get is GetWithContext with a background context.
func (m *Memory) Get(key string) ([]byte, error) { return m.GetWithContext(context.Background(), key) }

// SetWithContext stores a copy of val under key and ignores exp.
func (m *Memory) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	if err := m.begin(ctx, &Call{Op: OpSet, Key: key, Value: append([]byte(nil), val...), TTL: exp}); err != nil {
		return err
	}
	if m.Data == nil {
		m.Data = make(map[string][]byte)
	}
	m.Data[key] = append([]byte(nil), val...)
	return nil
}

// Set is SetWithContext with a background context.
func (m *Memory) Set(key string, val []byte, exp time.Duration) error {
	return m.SetWithContext(context.Background(), key, val, exp)
}

// DeleteWithContext removes key.
func (m *Memory) DeleteWithContext(ctx context.Context, key string) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	if err := m.begin(ctx, &Call{Op: OpDelete, Key: key}); err != nil {
		return err
	}
	delete(m.Data, key)
	return nil
}

// Delete is DeleteWithContext with a background context.
func (m *Memory) Delete(key string) error { return m.DeleteWithContext(context.Background(), key) }

// ResetWithContext drops all data. Like a real storage, it keeps the errors
// SetErr configured; ClearErrs removes them.
func (m *Memory) ResetWithContext(ctx context.Context) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	if err := m.begin(ctx, &Call{Op: OpReset}); err != nil {
		return err
	}
	clear(m.Data)
	return nil
}

// Reset is ResetWithContext with a background context.
func (m *Memory) Reset() error { return m.ResetWithContext(context.Background()) }

// Close does nothing and keeps the data.
func (*Memory) Close() error { return nil }

// Failing is a storage whose every operation fails with Err, which may be nil.
type Failing struct {
	// Err is what Get, Set, Delete and Reset return, with or without a context.
	Err error
	// CloseErr is what Close returns.
	CloseErr error
	closed   atomic.Bool
}

// Closed reports whether Close was called.
func (f *Failing) Closed() bool { return f.closed.Load() }

// GetWithContext fails with Err.
func (f *Failing) GetWithContext(context.Context, string) ([]byte, error) { return nil, f.Err }

// Get fails with Err.
func (f *Failing) Get(string) ([]byte, error) { return nil, f.Err }

// SetWithContext fails with Err.
func (f *Failing) SetWithContext(context.Context, string, []byte, time.Duration) error { return f.Err }

// Set fails with Err.
func (f *Failing) Set(string, []byte, time.Duration) error { return f.Err }

// DeleteWithContext fails with Err.
func (f *Failing) DeleteWithContext(context.Context, string) error { return f.Err }

// Delete fails with Err.
func (f *Failing) Delete(string) error { return f.Err }

// ResetWithContext fails with Err.
func (f *Failing) ResetWithContext(context.Context) error { return f.Err }

// Reset fails with Err.
func (f *Failing) Reset() error { return f.Err }

// Close records that it ran and returns CloseErr.
func (f *Failing) Close() error {
	f.closed.Store(true)
	return f.CloseErr
}
