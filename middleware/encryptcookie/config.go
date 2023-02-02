package encryptcookie

import (
	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Array of cookie keys that should not be encrypted.
	//
	// Optional. Default: []
	Except []string

	// Base64 encoded unique key to encode & decode cookies.
	//
	// Required. Key length should be 32 characters.
	// You may use `encryptcookie.GenerateKey()` to generate a new key.
	Key string

	// Custom function to encrypt cookies.
	//
	// Optional. Default: EncryptCookie
	Encryptor func(decryptedString, key string) (string, error)

	// Custom function to decrypt cookies.
	//
	// Optional. Default: DecryptCookie
	Decryptor func(encryptedString, key string) (string, error)
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:      nil,
	Except:    []string{"csrf_"},
	Key:       "",
	Encryptor: EncryptCookie,
	Decryptor: DecryptCookie,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values

		if cfg.Next == nil {
			cfg.Next = ConfigDefault.Next
		}

		if cfg.Except == nil {
			cfg.Except = ConfigDefault.Except
		}

		if cfg.Encryptor == nil {
			cfg.Encryptor = ConfigDefault.Encryptor
		}

		if cfg.Decryptor == nil {
			cfg.Decryptor = ConfigDefault.Decryptor
		}
	}

	if cfg.Key == "" {
		panic("fiber: encrypt cookie middleware requires key")
	}

	return cfg
}
