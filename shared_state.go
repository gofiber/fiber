package fiber

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/utils/v2"
)

const defaultSharedStatePrefix = "gofiber-shared-state-"

var ErrSharedStorageNotConfigured = errors.New("fiber: shared storage is not configured")

type SharedState struct {
	storage        Storage
	jsonEncoder    utils.JSONMarshal
	jsonDecoder    utils.JSONUnmarshal
	msgPackEncoder utils.MsgPackMarshal
	msgPackDecoder utils.MsgPackUnmarshal
	cborEncoder    utils.CBORMarshal
	cborDecoder    utils.CBORUnmarshal
	xmlEncoder     utils.XMLMarshal
	xmlDecoder     utils.XMLUnmarshal
	prefix         string
}

func newSharedState(cfg *Config) *SharedState {
	if cfg == nil {
		cfg = &Config{}
	}

	prefix := cfg.SharedStatePrefix
	if prefix == "" {
		prefix = defaultSharedStatePrefix
		if cfg.AppName != "" {
			prefix += cfg.AppName + "-"
		}
	}

	jsonEncoder := cfg.JSONEncoder
	if jsonEncoder == nil {
		jsonEncoder = json.Marshal
	}
	jsonDecoder := cfg.JSONDecoder
	if jsonDecoder == nil {
		jsonDecoder = json.Unmarshal
	}
	xmlEncoder := cfg.XMLEncoder
	if xmlEncoder == nil {
		xmlEncoder = xml.Marshal
	}
	xmlDecoder := cfg.XMLDecoder
	if xmlDecoder == nil {
		xmlDecoder = xml.Unmarshal
	}

	return &SharedState{
		storage:        cfg.SharedStorage,
		jsonEncoder:    jsonEncoder,
		jsonDecoder:    jsonDecoder,
		msgPackEncoder: cfg.MsgPackEncoder,
		msgPackDecoder: cfg.MsgPackDecoder,
		cborEncoder:    cfg.CBOREncoder,
		cborDecoder:    cfg.CBORDecoder,
		xmlEncoder:     xmlEncoder,
		xmlDecoder:     xmlDecoder,
		prefix:         prefix,
	}
}

func (s *SharedState) Set(key string, val []byte, ttl time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, ttl)
}

func (s *SharedState) SetWithContext(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	storageKey, ok := s.storageKey(key)
	if !ok {
		return nil
	}

	return s.storage.SetWithContext(ctx, storageKey, val, ttl)
}

func (s *SharedState) Get(key string) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetWithContext(context.Background(), key)
}

func (s *SharedState) GetWithContext(ctx context.Context, key string) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if err := s.ensureStorage(); err != nil {
		return nil, false, err
	}

	storageKey, ok := s.storageKey(key)
	if !ok {
		return nil, false, nil
	}

	data, err := s.storage.GetWithContext(ctx, storageKey)
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return nil, false, nil
	}

	return append([]byte(nil), data...), true, nil
}

func (s *SharedState) SetJSON(key string, v any, ttl time.Duration) error {
	return s.SetJSONWithContext(context.Background(), key, v, ttl)
}

func (s *SharedState) SetJSONWithContext(ctx context.Context, key string, v any, ttl time.Duration) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	return s.setEncodedWithContext(ctx, key, v, ttl, s.jsonEncoder, "json")
}

func (s *SharedState) GetJSON(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetJSONWithContext(context.Background(), key, out)
}

func (s *SharedState) GetJSONWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if err := s.ensureStorage(); err != nil {
		return nil, false, err
	}

	return s.getEncodedWithContext(ctx, key, out, s.jsonDecoder, "json")
}

func (s *SharedState) SetMsgPack(key string, v any, ttl time.Duration) error {
	return s.SetMsgPackWithContext(context.Background(), key, v, ttl)
}

func (s *SharedState) SetMsgPackWithContext(ctx context.Context, key string, v any, ttl time.Duration) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	return s.setEncodedWithContext(ctx, key, v, ttl, s.msgPackEncoder, "msgpack")
}

func (s *SharedState) GetMsgPack(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetMsgPackWithContext(context.Background(), key, out)
}

func (s *SharedState) GetMsgPackWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if err := s.ensureStorage(); err != nil {
		return nil, false, err
	}

	return s.getEncodedWithContext(ctx, key, out, s.msgPackDecoder, "msgpack")
}

func (s *SharedState) SetCBOR(key string, v any, ttl time.Duration) error {
	return s.SetCBORWithContext(context.Background(), key, v, ttl)
}

func (s *SharedState) SetCBORWithContext(ctx context.Context, key string, v any, ttl time.Duration) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	return s.setEncodedWithContext(ctx, key, v, ttl, s.cborEncoder, "cbor")
}

func (s *SharedState) GetCBOR(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetCBORWithContext(context.Background(), key, out)
}

func (s *SharedState) GetCBORWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if err := s.ensureStorage(); err != nil {
		return nil, false, err
	}

	return s.getEncodedWithContext(ctx, key, out, s.cborDecoder, "cbor")
}

func (s *SharedState) SetXML(key string, v any, ttl time.Duration) error {
	return s.SetXMLWithContext(context.Background(), key, v, ttl)
}

func (s *SharedState) SetXMLWithContext(ctx context.Context, key string, v any, ttl time.Duration) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	return s.setEncodedWithContext(ctx, key, v, ttl, s.xmlEncoder, "xml")
}

func (s *SharedState) GetXML(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetXMLWithContext(context.Background(), key, out)
}

func (s *SharedState) GetXMLWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if err := s.ensureStorage(); err != nil {
		return nil, false, err
	}

	return s.getEncodedWithContext(ctx, key, out, s.xmlDecoder, "xml")
}

func (s *SharedState) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *SharedState) DeleteWithContext(ctx context.Context, key string) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	storageKey, ok := s.storageKey(key)
	if !ok {
		return nil
	}

	return s.storage.DeleteWithContext(ctx, storageKey)
}

func (s *SharedState) Has(key string) (bool, error) {
	return s.HasWithContext(context.Background(), key)
}

func (s *SharedState) HasWithContext(ctx context.Context, key string) (bool, error) {
	if err := s.ensureStorage(); err != nil {
		return false, err
	}

	storageKey, ok := s.storageKey(key)
	if !ok {
		return false, nil
	}

	data, err := s.storage.GetWithContext(ctx, storageKey)
	if err != nil {
		return false, err
	}

	return data != nil, nil
}

func (s *SharedState) Reset() error {
	return s.ResetWithContext(context.Background())
}

func (s *SharedState) ResetWithContext(ctx context.Context) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	return s.storage.ResetWithContext(ctx)
}

func (s *SharedState) Close() error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	return s.storage.Close()
}

func (s *SharedState) ensureStorage() error {
	if s == nil || s.storage == nil {
		return ErrSharedStorageNotConfigured
	}

	return nil
}

func (s *SharedState) setEncodedWithContext(
	ctx context.Context,
	key string,
	v any,
	ttl time.Duration,
	encoder func(any) ([]byte, error),
	format string,
) error {
	if err := s.ensureStorage(); err != nil {
		return err
	}

	storageKey, ok := s.storageKey(key)
	if !ok {
		return nil
	}

	encoded, err := encodeSharedStateValue(v, encoder, format)
	if err != nil {
		return err
	}

	return s.storage.SetWithContext(ctx, storageKey, encoded, ttl)
}

//nolint:gocritic // Keep unnamed returns for clarity.
func (s *SharedState) getEncodedWithContext(
	ctx context.Context,
	key string,
	out any,
	decoder func([]byte, any) error,
	format string,
) ([]byte, bool, error) {
	if err := s.ensureStorage(); err != nil {
		return nil, false, err
	}

	storageKey, ok := s.storageKey(key)
	if !ok {
		return nil, false, nil
	}

	data, err := s.storage.GetWithContext(ctx, storageKey)
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return nil, false, nil
	}

	if err := decodeSharedStateValue(data, out, decoder, format); err != nil {
		return nil, false, err
	}

	return append([]byte(nil), data...), true, nil
}

func encodeSharedStateValue(v any, encoder func(any) ([]byte, error), format string) ([]byte, error) {
	if encoder == nil {
		return nil, sharedStateCodecNotConfiguredError(format, "encoder")
	}

	var (
		encoded   []byte
		err       error
		recovered any
	)
	func() {
		// App-configured codecs may be nil or may still use Fiber's
		// binder.Unimplemented* placeholders, which panic instead of returning an
		// error, so recover here and surface a regular error.
		defer func() {
			recovered = recover()
		}()

		encoded, err = encoder(v)
	}()

	if recovered != nil {
		return nil, sharedStateCodecPanicError("encode", format, recovered)
	}
	if err != nil {
		return nil, fmt.Errorf("fiber: failed to encode shared state %s value: %w", format, err)
	}

	return encoded, nil
}

func decodeSharedStateValue(data []byte, out any, decoder func([]byte, any) error, format string) error {
	if decoder == nil {
		return sharedStateCodecNotConfiguredError(format, "decoder")
	}

	var (
		err       error
		recovered any
	)
	func() {
		// App-configured codecs may be nil or may still use Fiber's
		// binder.Unimplemented* placeholders, which panic instead of returning an
		// error, so recover here and surface a regular error.
		defer func() {
			recovered = recover()
		}()

		err = decoder(data, out)
	}()

	if recovered != nil {
		return sharedStateCodecPanicError("decode", format, recovered)
	}
	if err != nil {
		return fmt.Errorf("fiber: failed to decode shared state %s value: %w", format, err)
	}

	return nil
}

func sharedStateCodecNotConfiguredError(format, direction string) error {
	return fmt.Errorf("fiber: shared state %s %s is not configured", format, direction)
}

func sharedStateCodecPanicError(operation, format string, recovered any) error {
	if err, ok := recovered.(error); ok {
		return fmt.Errorf("fiber: failed to %s shared state %s value: %w", operation, format, err)
	}

	return fmt.Errorf("fiber: failed to %s shared state %s value: %v", operation, format, recovered)
}

func (s *SharedState) storageKey(key string) (string, bool) {
	if key == "" {
		return "", false
	}

return s.prefix + hex.EncodeToString(utils.UnsafeBytes(key)), true
}
