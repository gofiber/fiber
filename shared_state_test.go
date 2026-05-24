package fiber

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	storagememory "github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

func newSharedStateMemoryStorage(t *testing.T) *storagememory.Storage {
	t.Helper()

	store := storagememory.New()
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	return store
}

type contextCheckingStorage struct {
	base   Storage
	ctxKey any
}

type errorStorage struct {
	err      error
	closeErr error
}

func (s *errorStorage) GetWithContext(context.Context, string) ([]byte, error) {
	return nil, s.err
}

func (s *errorStorage) Get(string) ([]byte, error) {
	return nil, s.err
}

func (s *errorStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	return s.err
}

func (s *errorStorage) Set(string, []byte, time.Duration) error {
	return s.err
}

func (s *errorStorage) DeleteWithContext(context.Context, string) error {
	return s.err
}

func (s *errorStorage) Delete(string) error {
	return s.err
}

func (s *errorStorage) ResetWithContext(context.Context) error {
	return s.err
}

func (s *errorStorage) Reset() error {
	return s.err
}

func (s *errorStorage) Close() error {
	return s.closeErr
}

func (s *contextCheckingStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if ctx.Value(s.ctxKey) == nil {
		return errors.New("context value not found")
	}
	return s.base.SetWithContext(ctx, key, val, exp)
}

func (s *contextCheckingStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if ctx.Value(s.ctxKey) == nil {
		return nil, errors.New("context value not found")
	}
	return s.base.GetWithContext(ctx, key)
}

func (s *contextCheckingStorage) DeleteWithContext(ctx context.Context, key string) error {
	if ctx.Value(s.ctxKey) == nil {
		return errors.New("context value not found")
	}
	return s.base.DeleteWithContext(ctx, key)
}

func (s *contextCheckingStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *contextCheckingStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (s *contextCheckingStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *contextCheckingStorage) ResetWithContext(ctx context.Context) error {
	return s.base.ResetWithContext(ctx)
}

func (s *contextCheckingStorage) Reset() error {
	return s.base.Reset()
}

func (s *contextCheckingStorage) Close() error {
	return s.base.Close()
}

func TestSharedState_NotConfigured(t *testing.T) {
	t.Parallel()

	app := New()

	err := app.SharedState().Set("raw-key", []byte("raw"), time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	raw, found, err := app.SharedState().Get("raw-key")
	require.Nil(t, raw)
	require.False(t, found)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = app.SharedState().SetJSON("key", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	err = app.SharedState().SetMsgPack("key", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	err = app.SharedState().SetCBOR("key", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	err = app.SharedState().SetXML("key", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	_, found, err = app.SharedState().GetJSON("key", &Map{})
	require.False(t, found)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	_, found, err = app.SharedState().GetMsgPack("key", &Map{})
	require.False(t, found)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	_, found, err = app.SharedState().GetCBOR("key", &Map{})
	require.False(t, found)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	_, found, err = app.SharedState().GetXML("key", &Map{})
	require.False(t, found)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = app.SharedState().Delete("key")
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	has, err := app.SharedState().Has("key")
	require.False(t, has)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = app.SharedState().Reset()
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = app.SharedState().Close()
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
}

func TestSharedState_PreforkSafeWithSharedStorage(t *testing.T) {
	t.Parallel()

	store := newSharedStateMemoryStorage(t)
	workerA := New(Config{AppName: "prefork-app", SharedStorage: store})
	workerB := New(Config{AppName: "prefork-app", SharedStorage: store})

	workerA.State().Set("process-only", "from-worker-a")
	_, ok := workerB.State().Get("process-only")
	require.False(t, ok)

	payload := Map{"worker": "a", "version": 3}
	err := workerA.SharedState().SetJSON("cluster-key", payload, time.Minute)
	require.NoError(t, err)

	err = workerA.SharedState().Set("raw-cluster-key", []byte("raw-value"), time.Minute)
	require.NoError(t, err)

	var out map[string]any
	rawJSON, found, err := workerB.SharedState().GetJSON("cluster-key", &out)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, rawJSON)
	require.Equal(t, "a", out["worker"])
	require.EqualValues(t, 3, out["version"])

	raw, found, err := workerB.SharedState().Get("raw-cluster-key")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("raw-value"), raw)

	has, err := workerB.SharedState().Has("cluster-key")
	require.NoError(t, err)
	require.True(t, has)

	err = workerB.SharedState().Delete("cluster-key")
	require.NoError(t, err)

	has, err = workerA.SharedState().Has("cluster-key")
	require.NoError(t, err)
	require.False(t, has)

	require.NoError(t, workerA.SharedState().Delete("raw-cluster-key"))
	raw, found, err = workerB.SharedState().Get("raw-cluster-key")
	require.NoError(t, err)
	require.Nil(t, raw)
	require.False(t, found)
}

func TestSharedState_ExplicitSerializationError(t *testing.T) {
	t.Parallel()

	app := New(Config{SharedStorage: newSharedStateMemoryStorage(t)})
	err := app.SharedState().SetJSON("invalid", make(chan int), time.Second)
	require.Error(t, err)
}

func TestSharedState_ContextAwareVariants(t *testing.T) {
	t.Parallel()

	type testContextKey string

	ctxKey := testContextKey("tenant")
	store := &contextCheckingStorage{ctxKey: ctxKey, base: newSharedStateMemoryStorage(t)}
	app := New(Config{SharedStorage: store})

	t.Run("missing context", func(t *testing.T) {
		t.Parallel()

		err := app.SharedState().SetJSONWithContext(context.Background(), "key", Map{"ok": true}, time.Second)
		require.Error(t, err)
	})

	t.Run("context propagation", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(context.Background(), ctxKey, "value")
		err := app.SharedState().SetJSONWithContext(ctx, "key", Map{"ok": true}, time.Second)
		require.NoError(t, err)

		var out map[string]bool
		_, found, err := app.SharedState().GetJSONWithContext(ctx, "key", &out)
		require.NoError(t, err)
		require.True(t, found)
		require.True(t, out["ok"])

		has, err := app.SharedState().HasWithContext(ctx, "key")
		require.NoError(t, err)
		require.True(t, has)

		err = app.SharedState().DeleteWithContext(ctx, "key")
		require.NoError(t, err)
	})
}

func TestSharedState_KeyNamespacing(t *testing.T) {
	t.Parallel()

	store := newSharedStateMemoryStorage(t)
	appOne := New(Config{AppName: "app-one", SharedStorage: store})
	appTwo := New(Config{AppName: "app-two", SharedStorage: store})

	err := appOne.SharedState().SetJSON("same-key", Map{"app": 1}, time.Minute)
	require.NoError(t, err)
	err = appTwo.SharedState().SetJSON("same-key", Map{"app": 2}, time.Minute)
	require.NoError(t, err)

	var outOne map[string]int
	_, found, err := appOne.SharedState().GetJSON("same-key", &outOne)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 1, outOne["app"])

	var outTwo map[string]int
	_, found, err = appTwo.SharedState().GetJSON("same-key", &outTwo)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 2, outTwo["app"])
}

func TestSharedState_KeyNamespacingWithOverlappingPrefixes(t *testing.T) {
	t.Parallel()

	store := newSharedStateMemoryStorage(t)
	shortPrefixApp := New(Config{AppName: "app", SharedStorage: store})
	longPrefixApp := New(Config{AppName: "app-one", SharedStorage: store})

	err := longPrefixApp.SharedState().Set("session:123", []byte("victim-secret"), time.Minute)
	require.NoError(t, err)

	err = shortPrefixApp.SharedState().Set("one-session:123", []byte("attacker-value"), time.Minute)
	require.NoError(t, err)

	data, found, err := longPrefixApp.SharedState().Get("session:123")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("victim-secret"), data)
}

func TestSharedState_StorageErrorsArePropagated(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("storage failed")
	closeErr := errors.New("close failed")
	app := New(Config{
		SharedStorage: &errorStorage{err: expectedErr, closeErr: closeErr},
		MsgPackEncoder: func(any) ([]byte, error) {
			return []byte("msgpack"), nil
		},
		MsgPackDecoder: func([]byte, any) error {
			return nil
		},
		CBOREncoder: func(any) ([]byte, error) {
			return []byte("cbor"), nil
		},
		CBORDecoder: func([]byte, any) error {
			return nil
		},
		XMLEncoder: func(any) ([]byte, error) {
			return []byte("<xml/>"), nil
		},
		XMLDecoder: func([]byte, any) error {
			return nil
		},
	})

	err := app.SharedState().SetJSON("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, expectedErr)
	err = app.SharedState().SetMsgPack("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, expectedErr)
	err = app.SharedState().SetCBOR("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, expectedErr)
	err = app.SharedState().SetXML("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, expectedErr)

	err = app.SharedState().Set("k", []byte("v"), time.Second)
	require.ErrorIs(t, err, expectedErr)

	_, _, err = app.SharedState().Get("k")
	require.ErrorIs(t, err, expectedErr)

	_, _, err = app.SharedState().GetJSON("k", &Map{})
	require.ErrorIs(t, err, expectedErr)
	_, _, err = app.SharedState().GetMsgPack("k", &Map{})
	require.ErrorIs(t, err, expectedErr)
	_, _, err = app.SharedState().GetCBOR("k", &Map{})
	require.ErrorIs(t, err, expectedErr)
	_, _, err = app.SharedState().GetXML("k", &Map{})
	require.ErrorIs(t, err, expectedErr)

	_, err = app.SharedState().Has("k")
	require.ErrorIs(t, err, expectedErr)

	err = app.SharedState().Delete("k")
	require.ErrorIs(t, err, expectedErr)

	err = app.SharedState().Reset()
	require.ErrorIs(t, err, expectedErr)

	err = app.SharedState().Close()
	require.ErrorIs(t, err, closeErr)
}

func TestSharedState_NilReceiver(t *testing.T) {
	t.Parallel()

	var state *SharedState

	err := state.SetJSON("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	err = state.SetMsgPack("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	err = state.SetCBOR("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	err = state.SetXML("k", Map{"v": 1}, time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = state.Set("k", []byte("v"), time.Second)
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	_, _, err = state.Get("k")
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	_, _, err = state.GetJSON("k", &Map{})
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	_, _, err = state.GetMsgPack("k", &Map{})
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	_, _, err = state.GetCBOR("k", &Map{})
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
	_, _, err = state.GetXML("k", &Map{})
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = state.Delete("k")
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	_, err = state.Has("k")
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = state.Reset()
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)

	err = state.Close()
	require.ErrorIs(t, err, ErrSharedStorageNotConfigured)
}

func TestSharedState_DefaultPrefixFallback(t *testing.T) {
	t.Parallel()

	state := newSharedState(&Config{SharedStorage: newSharedStateMemoryStorage(t)})
	require.Equal(t, defaultSharedStatePrefix, state.prefix)
}

func TestSharedState_NewAppDefaultPrefixIncludesAppName(t *testing.T) {
	t.Parallel()

	app := New(Config{AppName: "my-app", SharedStorage: newSharedStateMemoryStorage(t)})
	require.Equal(t, defaultSharedStatePrefix+"my-app-", app.SharedState().prefix)
}

func TestSharedState_GetJSON_UnmarshalError(t *testing.T) {
	t.Parallel()

	store := newSharedStateMemoryStorage(t)
	app := New(Config{SharedStorage: store})

	storageKey, ok := app.SharedState().storageKey("broken")
	require.True(t, ok)
	require.NoError(t, store.Set(storageKey, []byte("{"), 0))

	var out map[string]any
	_, found, err := app.SharedState().GetJSON("broken", &out)
	require.False(t, found)
	require.Error(t, err)
}

func TestSharedState_Get_ReturnsCopy(t *testing.T) {
	t.Parallel()

	app := New(Config{SharedStorage: newSharedStateMemoryStorage(t)})
	require.NoError(t, app.SharedState().Set("raw", []byte("value"), time.Minute))

	got, found, err := app.SharedState().Get("raw")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("value"), got)

	got[0] = 'X'

	gotAgain, found, err := app.SharedState().Get("raw")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("value"), gotAgain)
}

func TestSharedState_RawWithContextVariants(t *testing.T) {
	t.Parallel()

	type testContextKey string

	ctxKey := testContextKey("tenant")
	store := &contextCheckingStorage{ctxKey: ctxKey, base: newSharedStateMemoryStorage(t)}
	app := New(Config{SharedStorage: store})

	err := app.SharedState().SetWithContext(context.Background(), "raw", []byte("x"), time.Second)
	require.Error(t, err)

	ctx := context.WithValue(context.Background(), ctxKey, "value")
	require.NoError(t, app.SharedState().SetWithContext(ctx, "raw", []byte("x"), time.Second))

	raw, found, err := app.SharedState().GetWithContext(ctx, "raw")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("x"), raw)
}

func TestSharedState_SetGet_RawDataKinds(t *testing.T) {
	t.Parallel()

	app := New(Config{SharedStorage: newSharedStateMemoryStorage(t)})

	testCases := []struct {
		key   string
		value []byte
	}{
		{key: "plain", value: []byte("text")},
		{key: "json", value: []byte(`{"id":42}`)},
		{key: "binary", value: []byte{0x00, 0xFF, 0x10, 0x7F}},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, app.SharedState().Set(tc.key, tc.value, time.Minute))

			got, found, err := app.SharedState().Get(tc.key)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, tc.value, got)
		})
	}
}

func TestSharedState_SetGet_JSONDataKinds(t *testing.T) {
	t.Parallel()

	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	app := New(Config{SharedStorage: newSharedStateMemoryStorage(t)})

	t.Run("map", func(t *testing.T) {
		t.Parallel()

		expected := map[string]any{
			"name": "fiber",
			"ok":   true,
		}
		require.NoError(t, app.SharedState().SetJSON("map", expected, time.Minute))

		var out map[string]any
		raw, found, err := app.SharedState().GetJSON("map", &out)
		require.NoError(t, err)
		require.True(t, found)
		require.NotEmpty(t, raw)
		require.Equal(t, expected["name"], out["name"])
		require.Equal(t, expected["ok"], out["ok"])
	})

	t.Run("slice", func(t *testing.T) {
		t.Parallel()

		expected := []int{1, 2, 3, 4}
		require.NoError(t, app.SharedState().SetJSON("slice", expected, time.Minute))

		var out []int
		_, found, err := app.SharedState().GetJSON("slice", &out)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expected, out)
	})

	t.Run("struct", func(t *testing.T) {
		t.Parallel()

		expected := sample{Name: "shared", Count: 7}
		require.NoError(t, app.SharedState().SetJSON("struct", expected, time.Minute))

		var out sample
		_, found, err := app.SharedState().GetJSON("struct", &out)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expected, out)
	})
}

func TestSharedState_UsesAppJSONCodec(t *testing.T) {
	t.Parallel()

	encoderCalled := false
	decoderCalled := false

	app := New(Config{
		SharedStorage: newSharedStateMemoryStorage(t),
		JSONEncoder: func(_ any) ([]byte, error) {
			encoderCalled = true
			return json.Marshal(Map{"via": "custom-encoder"})
		},
		JSONDecoder: func(data []byte, out any) error {
			decoderCalled = true
			return json.Unmarshal(data, out)
		},
	})

	require.NoError(t, app.SharedState().SetJSON("codec", Map{"ignored": true}, time.Minute))

	var out map[string]string
	_, found, err := app.SharedState().GetJSON("codec", &out)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "custom-encoder", out["via"])
	require.True(t, encoderCalled)
	require.True(t, decoderCalled)
}

func TestSharedState_UsesAppMsgPackCodec(t *testing.T) {
	t.Parallel()

	encoderCalled := false
	decoderCalled := false

	app := New(Config{
		SharedStorage: newSharedStateMemoryStorage(t),
		MsgPackEncoder: func(_ any) ([]byte, error) {
			encoderCalled = true
			return []byte("msgpack-payload"), nil
		},
		MsgPackDecoder: func(data []byte, out any) error {
			decoderCalled = true
			ptr, ok := out.(*string)
			if ok {
				*ptr = string(data)
			}
			return nil
		},
	})

	require.NoError(t, app.SharedState().SetMsgPack("codec", Map{"ignored": true}, time.Minute))

	var out string
	raw, found, err := app.SharedState().GetMsgPack("codec", &out)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("msgpack-payload"), raw)
	require.Equal(t, "msgpack-payload", out)
	require.True(t, encoderCalled)
	require.True(t, decoderCalled)
}

func TestSharedState_UnconfiguredCodecsReturnErrorInsteadOfPanic(t *testing.T) {
	t.Parallel()

	app := New(Config{SharedStorage: newSharedStateMemoryStorage(t)})

	err := app.SharedState().SetMsgPack("codec", Map{"ignored": true}, time.Minute)
	require.ErrorContains(t, err, "shared state msgpack")

	require.NoError(t, app.SharedState().Set("msgpack-payload", []byte("payload"), time.Minute))

	var out Map
	_, found, err := app.SharedState().GetMsgPack("msgpack-payload", &out)
	require.False(t, found)
	require.ErrorContains(t, err, "shared state msgpack")

	err = app.SharedState().SetCBOR("codec", Map{"ignored": true}, time.Minute)
	require.ErrorContains(t, err, "shared state cbor")

	require.NoError(t, app.SharedState().Set("cbor-payload", []byte("payload"), time.Minute))

	_, found, err = app.SharedState().GetCBOR("cbor-payload", &out)
	require.False(t, found)
	require.ErrorContains(t, err, "shared state cbor")
}

func TestSharedState_UsesAppCBORCodec(t *testing.T) {
	t.Parallel()

	encoderCalled := false
	decoderCalled := false

	app := New(Config{
		SharedStorage: newSharedStateMemoryStorage(t),
		CBOREncoder: func(_ any) ([]byte, error) {
			encoderCalled = true
			return []byte("cbor-payload"), nil
		},
		CBORDecoder: func(data []byte, out any) error {
			decoderCalled = true
			ptr, ok := out.(*string)
			if ok {
				*ptr = string(data)
			}
			return nil
		},
	})

	require.NoError(t, app.SharedState().SetCBOR("codec", Map{"ignored": true}, time.Minute))

	var out string
	raw, found, err := app.SharedState().GetCBOR("codec", &out)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("cbor-payload"), raw)
	require.Equal(t, "cbor-payload", out)
	require.True(t, encoderCalled)
	require.True(t, decoderCalled)
}

func TestSharedState_UsesAppXMLCodec(t *testing.T) {
	t.Parallel()

	encoderCalled := false
	decoderCalled := false

	app := New(Config{
		SharedStorage: newSharedStateMemoryStorage(t),
		XMLEncoder: func(_ any) ([]byte, error) {
			encoderCalled = true
			return []byte("<value>xml-payload</value>"), nil
		},
		XMLDecoder: func(data []byte, out any) error {
			decoderCalled = true
			ptr, ok := out.(*string)
			if ok {
				*ptr = string(data)
			}
			return nil
		},
	})

	require.NoError(t, app.SharedState().SetXML("codec", Map{"ignored": true}, time.Minute))

	var out string
	raw, found, err := app.SharedState().GetXML("codec", &out)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("<value>xml-payload</value>"), raw)
	require.Equal(t, "<value>xml-payload</value>", out)
	require.True(t, encoderCalled)
	require.True(t, decoderCalled)
}

func TestSharedState_EmptyKeyBehavior(t *testing.T) {
	t.Parallel()

	app := New(Config{SharedStorage: newSharedStateMemoryStorage(t)})

	require.NoError(t, app.SharedState().Set("", []byte("raw"), time.Minute))
	require.NoError(t, app.SharedState().SetJSON("", Map{"v": 1}, time.Minute))
	require.NoError(t, app.SharedState().SetMsgPack("", Map{"v": 1}, time.Minute))
	require.NoError(t, app.SharedState().SetCBOR("", Map{"v": 1}, time.Minute))
	require.NoError(t, app.SharedState().SetXML("", Map{"v": 1}, time.Minute))

	raw, found, err := app.SharedState().Get("")
	require.NoError(t, err)
	require.Nil(t, raw)
	require.False(t, found)

	_, found, err = app.SharedState().GetJSON("", &Map{})
	require.NoError(t, err)
	require.False(t, found)

	_, found, err = app.SharedState().GetMsgPack("", &Map{})
	require.NoError(t, err)
	require.False(t, found)

	_, found, err = app.SharedState().GetCBOR("", &Map{})
	require.NoError(t, err)
	require.False(t, found)

	_, found, err = app.SharedState().GetXML("", &Map{})
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, app.SharedState().Delete(""))

	has, err := app.SharedState().Has("")
	require.NoError(t, err)
	require.False(t, has)
}
