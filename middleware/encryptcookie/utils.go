package encryptcookie

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// EncryptCookie Encrypts a cookie value with specific encryption key
func EncryptCookie(value, key string) (string, error) {
	keyDecoded, _ := base64.StdEncoding.DecodeString(key)
	plaintext := []byte(value)

	block, err := aes.NewCipher(keyDecoded)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCookie Decrypts a cookie value with specific encryption key
func DecryptCookie(value, key string) (string, error) {
	keyDecoded, _ := base64.StdEncoding.DecodeString(key)
	enc, _ := base64.StdEncoding.DecodeString(value)

	block, err := aes.NewCipher(keyDecoded)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()

	if len(enc) < nonceSize {
		return "", errors.New("encrypted value is not valid")
	}

	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GenerateKey Generates an encryption key
func GenerateKey() string {
	ret := make([]byte, 32)

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
