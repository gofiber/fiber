package encryptcookie

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// EncryptCookie Encrypts a cookie value with specific encryption key
func EncryptCookie(value, key string) (string, error) {
	keyDecoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("failed to base64-decode key: %w", err)
	}

	block, err := aes.NewCipher(keyDecoded)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM mode: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to read: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(value), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCookie Decrypts a cookie value with specific encryption key
func DecryptCookie(value, key string) (string, error) {
	keyDecoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("failed to base64-decode key: %w", err)
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

// GenerateKey Generates an encryption key
func GenerateKey() string {
	const keyLen = 32
	ret := make([]byte, keyLen)

	if _, err := rand.Read(ret); err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(ret)
}

// Check given cookie key is disabled for encryption or not
func isDisabled(key string, except []string) bool {
	for _, k := range except {
		if key == k {
			return true
		}
	}

	return false
}
