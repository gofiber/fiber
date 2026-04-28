package fiber

import (
	"context"
	"encoding/json"
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

func newSharedState(
	storage Storage,
	prefix string,
	jsonEncoder utils.JSONMarshal,
	jsonDecoder utils.JSONUnmarshal,
	msgPackEncoder utils.MsgPackMarshal,
	msgPackDecoder utils.MsgPackUnmarshal,
	cborEncoder utils.CBORMarshal,
	cborDecoder utils.CBORUnmarshal,
	xmlEncoder utils.XMLMarshal,
	xmlDecoder utils.XMLUnmarshal,
) *SharedState {
	if prefix == "" {
		prefix = defaultSharedStatePrefix
	}

	if jsonEncoder == nil {
		jsonEncoder = json.Marshal
	}
	if jsonDecoder == nil {
		jsonDecoder = json.Unmarshal
	}

	return &SharedState{
		storage:        storage,
		jsonEncoder:    jsonEncoder,
		jsonDecoder:    jsonDecoder,
		msgPackEncoder: msgPackEncoder,
		msgPackDecoder: msgPackDecoder,
		cborEncoder:    cborEncoder,
		cborDecoder:    cborDecoder,
		xmlEncoder:     xmlEncoder,
		xmlDecoder:     xmlDecoder,
		prefix:         prefix,
	}
}

func (s *SharedState) Set(key string, val []byte, ttl time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, ttl)
}

func (s *SharedState) SetWithContext(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if s == nil || s.storage == nil {
		return ErrSharedStorageNotConfigured
	}

	return s.storage.SetWithContext(ctx, s.key(key), val, ttl)
}

func (s *SharedState) Get(key string) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetWithContext(context.Background(), key)
}

func (s *SharedState) GetWithContext(ctx context.Context, key string) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if s == nil || s.storage == nil {
		return nil, false, ErrSharedStorageNotConfigured
	}

	data, err := s.storage.GetWithContext(ctx, s.key(key))
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
	if s == nil || s.storage == nil {
		return ErrSharedStorageNotConfigured
	}

	encoded, err := s.jsonEncoder(v)
	if err != nil {
		return fmt.Errorf("fiber: failed to encode shared state value: %w", err)
	}

	return s.storage.SetWithContext(ctx, s.key(key), encoded, ttl)
}

func (s *SharedState) GetJSON(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetJSONWithContext(context.Background(), key, out)
}

func (s *SharedState) GetJSONWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if s == nil || s.storage == nil {
		return nil, false, ErrSharedStorageNotConfigured
	}

	data, err := s.storage.GetWithContext(ctx, s.key(key))
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return nil, false, nil
	}

	if err := s.jsonDecoder(data, out); err != nil {
		return nil, false, fmt.Errorf("fiber: failed to decode shared state value: %w", err)
	}

	return append([]byte(nil), data...), true, nil
}

func (s *SharedState) SetMsgPack(key string, v any, ttl time.Duration) error {
	return s.SetMsgPackWithContext(context.Background(), key, v, ttl)
}

func (s *SharedState) SetMsgPackWithContext(ctx context.Context, key string, v any, ttl time.Duration) error {
	if s == nil || s.storage == nil {
		return ErrSharedStorageNotConfigured
	}

	encoded, err := s.msgPackEncoder(v)
	if err != nil {
		return fmt.Errorf("fiber: failed to encode shared state msgpack value: %w", err)
	}

	return s.storage.SetWithContext(ctx, s.key(key), encoded, ttl)
}

func (s *SharedState) GetMsgPack(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetMsgPackWithContext(context.Background(), key, out)
}

func (s *SharedState) GetMsgPackWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if s == nil || s.storage == nil {
		return nil, false, ErrSharedStorageNotConfigured
	}

	data, err := s.storage.GetWithContext(ctx, s.key(key))
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return nil, false, nil
	}

	if err := s.msgPackDecoder(data, out); err != nil {
		return nil, false, fmt.Errorf("fiber: failed to decode shared state msgpack value: %w", err)
	}

	return append([]byte(nil), data...), true, nil
}

func (s *SharedState) SetCBOR(key string, v any, ttl time.Duration) error {
	return s.SetCBORWithContext(context.Background(), key, v, ttl)
}

func (s *SharedState) SetCBORWithContext(ctx context.Context, key string, v any, ttl time.Duration) error {
	if s == nil || s.storage == nil {
		return ErrSharedStorageNotConfigured
	}

	encoded, err := s.cborEncoder(v)
	if err != nil {
		return fmt.Errorf("fiber: failed to encode shared state cbor value: %w", err)
	}

	return s.storage.SetWithContext(ctx, s.key(key), encoded, ttl)
}

func (s *SharedState) GetCBOR(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetCBORWithContext(context.Background(), key, out)
}

func (s *SharedState) GetCBORWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if s == nil || s.storage == nil {
		return nil, false, ErrSharedStorageNotConfigured
	}

	data, err := s.storage.GetWithContext(ctx, s.key(key))
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return nil, false, nil
	}

	if err := s.cborDecoder(data, out); err != nil {
		return nil, false, fmt.Errorf("fiber: failed to decode shared state cbor value: %w", err)
	}

	return append([]byte(nil), data...), true, nil
}

func (s *SharedState) SetXML(key string, v any, ttl time.Duration) error {
	return s.SetXMLWithContext(context.Background(), key, v, ttl)
}

func (s *SharedState) SetXMLWithContext(ctx context.Context, key string, v any, ttl time.Duration) error {
	if s == nil || s.storage == nil {
		return ErrSharedStorageNotConfigured
	}

	encoded, err := s.xmlEncoder(v)
	if err != nil {
		return fmt.Errorf("fiber: failed to encode shared state xml value: %w", err)
	}

	return s.storage.SetWithContext(ctx, s.key(key), encoded, ttl)
}

func (s *SharedState) GetXML(key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	return s.GetXMLWithContext(context.Background(), key, out)
}

func (s *SharedState) GetXMLWithContext(ctx context.Context, key string, out any) ([]byte, bool, error) { //nolint:gocritic // Keep unnamed returns for clarity.
	if s == nil || s.storage == nil {
		return nil, false, ErrSharedStorageNotConfigured
	}

	data, err := s.storage.GetWithContext(ctx, s.key(key))
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return nil, false, nil
	}

	if err := s.xmlDecoder(data, out); err != nil {
		return nil, false, fmt.Errorf("fiber: failed to decode shared state xml value: %w", err)
	}

	return append([]byte(nil), data...), true, nil
}

func (s *SharedState) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *SharedState) DeleteWithContext(ctx context.Context, key string) error {
	if s == nil || s.storage == nil {
		return ErrSharedStorageNotConfigured
	}

	return s.storage.DeleteWithContext(ctx, s.key(key))
}

func (s *SharedState) Has(key string) (bool, error) {
	return s.HasWithContext(context.Background(), key)
}

func (s *SharedState) HasWithContext(ctx context.Context, key string) (bool, error) {
	if s == nil || s.storage == nil {
		return false, ErrSharedStorageNotConfigured
	}

	data, err := s.storage.GetWithContext(ctx, s.key(key))
	if err != nil {
		return false, err
	}

	return data != nil, nil
}

func (s *SharedState) key(key string) string {
	return s.prefix + key
}
