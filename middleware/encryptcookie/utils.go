package encryptcookie

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
)

var ErrInvalidKeyLength = errors.New("encryption key must be 16, 24, or 32 bytes")

const (
	encryptCookieBufferDefaultCap = 32
	encryptCookieBufferMaxCap     = 4096
)

var encryptCookieBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, encryptCookieBufferDefaultCap)
		return &buf
	},
}

func acquireEncryptCookieBuffer(requiredCap, nonceSize int) *[]byte {
	bufAny := encryptCookieBufferPool.Get()
	bufPtr, ok := bufAny.(*[]byte)
	if !ok {
		panic(errors.New("failed to type-assert to *[]byte"))
	}

	buf := *bufPtr
	if cap(buf) < requiredCap {
		buf = make([]byte, 0, requiredCap)
	}

	buf = buf[:nonceSize]
	*bufPtr = buf

	return bufPtr
}

func releaseEncryptCookieBuffer(bufPtr *[]byte) {
	if bufPtr == nil {
		return
	}

	buf := *bufPtr
	if cap(buf) > encryptCookieBufferMaxCap {
		*bufPtr = make([]byte, 0, encryptCookieBufferDefaultCap)
		encryptCookieBufferPool.Put(bufPtr)
		return
	}

	for i := range buf {
		buf[i] = 0
	}

	*bufPtr = buf[:0]
	encryptCookieBufferPool.Put(bufPtr)
}

// decodeKey decodes the provided base64-encoded key and validates its length.
// It returns the decoded key bytes or an error when invalid.
func decodeKey(key string) ([]byte, error) {
	keyDecoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to base64-decode key: %w", err)
	}

	keyLen := len(keyDecoded)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidKeyLength
	}

	return keyDecoded, nil
}

// validateKey checks if the provided base64-encoded key is of valid length.
func validateKey(key string) error {
	_, err := decodeKey(key)
	return err
}

// EncryptCookie Encrypts a cookie value with specific encryption key
func EncryptCookie(value, key string) (string, error) {
	keyDecoded, err := decodeKey(key)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(keyDecoded)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM mode: %w", err)
	}

	nonceSize := gcm.NonceSize()
	requiredCap := nonceSize + len(value) + gcm.Overhead()
	noncePtr := acquireEncryptCookieBuffer(requiredCap, nonceSize)
	nonce := *noncePtr

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		releaseEncryptCookieBuffer(noncePtr)
		return "", fmt.Errorf("failed to read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(value), nil)
	*noncePtr = ciphertext

	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	releaseEncryptCookieBuffer(noncePtr)

	return encoded, nil
}

// DecryptCookie Decrypts a cookie value with specific encryption key
func DecryptCookie(value, key string) (string, error) {
	keyDecoded, err := decodeKey(key)
	if err != nil {
		return "", err
	}

	enc, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("failed to base64-decode value: %w", err)
	}

	block, err := aes.NewCipher(keyDecoded)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM mode: %w", err)
	}

	nonceSize := gcm.NonceSize()

	if len(enc) < nonceSize {
		return "", errors.New("encrypted value is not valid")
	}

	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}

// GenerateKey returns a random string of 16, 24, or 32 bytes.
// The length of the key determines the AES encryption algorithm used:
// 16 bytes for AES-128, 24 bytes for AES-192, and 32 bytes for AES-256-GCM.
func GenerateKey(length int) string {
	if length != 16 && length != 24 && length != 32 {
		panic(ErrInvalidKeyLength)
	}

	key := make([]byte, length)

	if _, err := rand.Read(key); err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(key)
}

// Check given cookie key is disabled for encryption or not
func isDisabled(key string, except []string) bool {
	return slices.Contains(except, key)
}
