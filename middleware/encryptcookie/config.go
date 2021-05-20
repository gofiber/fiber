package encryptcookie

import "github.com/gofiber/fiber/v2"

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Array of cookies that should not encrypt
	//
	// Optional. Default: []
	Except []string

	// Base64 unique key to encode & decode cookies
	//
	// Optional. Default: Generating new key on every run
	Key string

	// Custom function to encrypt cookies
	//
	// Optional. Default: EncryptCookie
	Encryptor func(decryptedString, key string) (string, error)

	// Custom function to decrypt cookies
	//
	// Optional. Default: DecryptCookie
	Decryptor func(encryptedString, key string) (string, error)
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:      nil,
	Except:    make([]string, 0),
	Key:       GenerateKey(32),
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

		if cfg.Key == "" {
			cfg.Key = ConfigDefault.Key
		}

		if cfg.Encryptor == nil {
			cfg.Encryptor = ConfigDefault.Encryptor
		}

		if cfg.Decryptor == nil {
			cfg.Decryptor = ConfigDefault.Decryptor
		}
	}

	return cfg
}
