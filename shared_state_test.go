package fiber

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	storagememory "github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/stretchr/testify/require"
)

func newSharedStateMemoryStorage(t *testing.T) *storagememory.Storage {
	t.Helper()

	store := storagememory.New()
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	return store
}

// stateTestRequireMarked requires that every call of the ops in store carries the Marked context label.
func stateTestRequireMarked(t *testing.T, store *storagetest.Memory, label string, ops ...string) {
	t.Helper()

	for _, op := range ops {
		calls := store.Calls(op)
		require.NotEmpty(t, calls, op)
		for _, call := range calls {
			require.Equal(t, label, call.Marker, op)
		}
	}
}

// stateTestExpectSharedOpsFail calls every SharedState operation on state and expects each one to fail
// with wantErr and return no data, except Close, which must fail with wantCloseErr.
func stateTestExpectSharedOpsFail(t *testing.T, state *SharedState, wantErr, wantCloseErr error) {
	t.Helper()

	require.ErrorIs(t, state.Set("k", []byte("v"), time.Second), wantErr)

	raw, found, err := state.Get("k")
	require.Nil(t, raw)
	require.False(t, found)
	require.ErrorIs(t, err, wantErr)

	setters := []func(string, any, time.Duration) error{state.SetJSON, state.SetMsgPack, state.SetCBOR, state.SetXML}
	for i, set := range setters {
		require.ErrorIs(t, set("k", Map{"v": 1}, time.Second), wantErr, "setter %d", i)
	}
	getters := []func(string, any) ([]byte, bool, error){state.GetJSON, state.GetMsgPack, state.GetCBOR, state.GetXML}
	for i, get := range getters {
		_, found, err = get("k", &Map{})
		require.False(t, found, "getter %d", i)
		require.ErrorIs(t, err, wantErr, "getter %d", i)
	}

	has, err := state.Has("k")
	require.False(t, has)
	require.ErrorIs(t, err, wantErr)

	require.ErrorIs(t, state.Delete("k"), wantErr)
	require.ErrorIs(t, state.Reset(), wantErr)
	require.ErrorIs(t, state.Close(), wantCloseErr)
}

func TestSharedState_NotConfigured(t *testing.T) {
	t.Parallel()
	app := New()

	stateTestExpectSharedOpsFail(t, app.SharedState(), ErrSharedStorageNotConfigured, ErrSharedStorageNotConfigured)
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
	require.NoError(t, workerA.SharedState().SetJSON("cluster-key", payload, time.Minute))
	require.NoError(t, workerA.SharedState().Set("raw-cluster-key", []byte("raw-value"), time.Minute))

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

	require.NoError(t, workerB.SharedState().Delete("cluster-key"))

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
	require.Error(t, app.SharedState().SetJSON("invalid", make(chan int), time.Second))
}

func TestSharedState_ContextAwareVariants(t *testing.T) {
	t.Parallel()
	store := storagetest.New()
	state := New(Config{SharedStorage: store}).SharedState()
	ctx := storagetest.Marked("tenant")

	require.NoError(t, state.SetJSONWithContext(ctx, "key", Map{"ok": true}, time.Second))

	var out map[string]bool
	_, found, err := state.GetJSONWithContext(ctx, "key", &out)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, out["ok"])

	has, err := state.HasWithContext(ctx, "key")
	require.NoError(t, err)
	require.True(t, has)

	require.NoError(t, state.DeleteWithContext(ctx, "key"))
	stateTestRequireMarked(t, store, "tenant", storagetest.OpSet, storagetest.OpGet, storagetest.OpDelete)
}

func TestSharedState_KeyNamespacing(t *testing.T) {
	t.Parallel()
	store := newSharedStateMemoryStorage(t)
	appOne := New(Config{AppName: "app-one", SharedStorage: store})
	appTwo := New(Config{AppName: "app-two", SharedStorage: store})

	require.NoError(t, appOne.SharedState().SetJSON("same-key", Map{"app": 1}, time.Minute))
	require.NoError(t, appTwo.SharedState().SetJSON("same-key", Map{"app": 2}, time.Minute))

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

	require.NoError(t, longPrefixApp.SharedState().Set("session:123", []byte("victim-secret"), time.Minute))
	require.NoError(t, shortPrefixApp.SharedState().Set("one-session:123", []byte("attacker-value"), time.Minute))

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
		SharedStorage:  &storagetest.Failing{Err: expectedErr, CloseErr: closeErr},
		MsgPackEncoder: func(any) ([]byte, error) { return []byte("msgpack"), nil },
		MsgPackDecoder: func([]byte, any) error { return nil },
		CBOREncoder:    func(any) ([]byte, error) { return []byte("cbor"), nil },
		CBORDecoder:    func([]byte, any) error { return nil },
		XMLEncoder:     func(any) ([]byte, error) { return []byte("<xml/>"), nil },
		XMLDecoder:     func([]byte, any) error { return nil },
	})

	stateTestExpectSharedOpsFail(t, app.SharedState(), expectedErr, closeErr)
}

func TestSharedState_NilReceiver(t *testing.T) {
	t.Parallel()
	var state *SharedState

	stateTestExpectSharedOpsFail(t, state, ErrSharedStorageNotConfigured, ErrSharedStorageNotConfigured)
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
	store := storagetest.New()
	state := New(Config{SharedStorage: store}).SharedState()
	ctx := storagetest.Marked("tenant")

	require.NoError(t, state.SetWithContext(ctx, "raw", []byte("x"), time.Second))

	raw, found, err := state.GetWithContext(ctx, "raw")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("x"), raw)
	stateTestRequireMarked(t, store, "tenant", storagetest.OpSet, storagetest.OpGet)
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
		expected := map[string]any{"name": "fiber", "ok": true}
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

// TestSharedState_UsesAppCodec checks that SharedState encodes and decodes with the MsgPack, CBOR and
// XML codecs configured on the app. Each subtest configures only the codec under test, so a method
// wired to another codec would never see the app's encoder and decoder. JSON has its own test above
// because it round-trips a real document.
func TestSharedState_UsesAppCodec(t *testing.T) {
	t.Parallel()
	type (
		encoder = func(any) ([]byte, error)
		decoder = func([]byte, any) error
	)

	for _, tc := range []struct {
		configure func(cfg *Config, enc encoder, dec decoder)
		set       func(*SharedState, string, any, time.Duration) error
		get       func(*SharedState, string, any) ([]byte, bool, error)
		name      string
		payload   string
	}{
		{
			name:      "MsgPack",
			payload:   "msgpack-payload",
			configure: func(cfg *Config, enc encoder, dec decoder) { cfg.MsgPackEncoder, cfg.MsgPackDecoder = enc, dec },
			set:       (*SharedState).SetMsgPack,
			get:       (*SharedState).GetMsgPack,
		},
		{
			name:      "CBOR",
			payload:   "cbor-payload",
			configure: func(cfg *Config, enc encoder, dec decoder) { cfg.CBOREncoder, cfg.CBORDecoder = enc, dec },
			set:       (*SharedState).SetCBOR,
			get:       (*SharedState).GetCBOR,
		},
		{
			name:      "XML",
			payload:   "<value>xml-payload</value>",
			configure: func(cfg *Config, enc encoder, dec decoder) { cfg.XMLEncoder, cfg.XMLDecoder = enc, dec },
			set:       (*SharedState).SetXML,
			get:       (*SharedState).GetXML,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			encoderCalled := false
			decoderCalled := false

			cfg := Config{SharedStorage: newSharedStateMemoryStorage(t)}
			tc.configure(&cfg,
				func(_ any) ([]byte, error) {
					encoderCalled = true
					return []byte(tc.payload), nil
				},
				func(data []byte, out any) error {
					decoderCalled = true
					ptr, ok := out.(*string)
					if ok {
						*ptr = string(data)
					}
					return nil
				},
			)
			app := New(cfg)

			require.NoError(t, tc.set(app.SharedState(), "codec", Map{"ignored": true}, time.Minute))

			var out string
			raw, found, err := tc.get(app.SharedState(), "codec", &out)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, []byte(tc.payload), raw)
			require.Equal(t, tc.payload, out)
			require.True(t, encoderCalled)
			require.True(t, decoderCalled)
		})
	}
}

func TestSharedState_UnconfiguredCodecsReturnErrorInsteadOfPanic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		set     func(*SharedState, string, any, time.Duration) error
		get     func(*SharedState, string, any) ([]byte, bool, error)
		name    string
		wantErr string
	}{
		{
			name: "MsgPack", set: (*SharedState).SetMsgPack, get: (*SharedState).GetMsgPack,
			wantErr: "shared state msgpack",
		},
		{name: "CBOR", set: (*SharedState).SetCBOR, get: (*SharedState).GetCBOR, wantErr: "shared state cbor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := New(Config{SharedStorage: newSharedStateMemoryStorage(t)}).SharedState()

			require.ErrorContains(t, tc.set(state, "codec", Map{"ignored": true}, time.Minute), tc.wantErr)

			require.NoError(t, state.Set("payload", []byte("payload"), time.Minute))

			var out Map
			_, found, err := tc.get(state, "payload", &out)
			require.False(t, found)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestSharedState_EmptyKeyBehavior(t *testing.T) {
	t.Parallel()
	app := New(Config{SharedStorage: newSharedStateMemoryStorage(t)})

	state := app.SharedState()
	require.NoError(t, state.Set("", []byte("raw"), time.Minute))
	setters := []func(string, any, time.Duration) error{state.SetJSON, state.SetMsgPack, state.SetCBOR, state.SetXML}
	for i, set := range setters {
		require.NoError(t, set("", Map{"v": 1}, time.Minute), "setter %d", i)
	}

	raw, found, err := state.Get("")
	require.NoError(t, err)
	require.Nil(t, raw)
	require.False(t, found)

	getters := []func(string, any) ([]byte, bool, error){state.GetJSON, state.GetMsgPack, state.GetCBOR, state.GetXML}
	for i, get := range getters {
		_, found, err = get("", &Map{})
		require.NoError(t, err, "getter %d", i)
		require.False(t, found, "getter %d", i)
	}

	require.NoError(t, state.Delete(""))

	has, err := state.Has("")
	require.NoError(t, err)
	require.False(t, has)
}
