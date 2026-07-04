package aigateway

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_ConfigDefault_Values(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		Upstreams: []Upstream{{Name: "test", URL: "https://api.example.com/", Key: "sk-test"}},
	})

	require.Equal(t, 1, cfg.Retry.Attempts)
	require.Equal(t, 250*time.Millisecond, cfg.Retry.Backoff)
	require.Equal(t, 2*time.Second, cfg.Retry.MaxBackoff)
	require.Equal(t, 30*time.Second, cfg.HeaderTimeout)
	require.Equal(t, 90*time.Second, cfg.StreamIdleTimeout)
	require.NotNil(t, cfg.Client)
	require.NotNil(t, cfg.KeyExtractor.Extract)

	// Upstream normalization
	require.Equal(t, "https://api.example.com", cfg.Upstreams[0].URL)
	require.Equal(t, AuthBearer(), cfg.Upstreams[0].Auth)
}

func Test_ConfigDefault_PathPrefixNormalization(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		PathPrefix: "openai/",
		Upstreams:  []Upstream{{Name: "test", URL: "https://api.example.com", Key: "k"}},
	})
	require.Equal(t, "/openai", cfg.PathPrefix)
}

func Test_ConfigDefault_Panics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { configDefault() })
	require.Panics(t, func() { configDefault(Config{}) })
	require.Panics(t, func() {
		configDefault(Config{Upstreams: []Upstream{{URL: "https://api.example.com", Key: "k"}}})
	})
	require.Panics(t, func() {
		configDefault(Config{Upstreams: []Upstream{{Name: "test", Key: "k"}}})
	})
	require.Panics(t, func() {
		configDefault(Config{Upstreams: []Upstream{{Name: "test", URL: "not-a-url", Key: "k"}}})
	})
	require.Panics(t, func() {
		configDefault(Config{Upstreams: []Upstream{{Name: "test", URL: "ftp://api.example.com", Key: "k"}}})
	})
	// Missing key in unified-key mode
	require.Panics(t, func() {
		configDefault(Config{Upstreams: []Upstream{{Name: "test", URL: "https://api.example.com"}}})
	})
	// ForwardClientKey makes the key optional
	require.NotPanics(t, func() {
		configDefault(Config{
			ForwardClientKey: true,
			Upstreams:        []Upstream{{Name: "test", URL: "https://api.example.com"}},
		})
	})
	// Contradictory key modes
	require.Panics(t, func() {
		configDefault(Config{
			ForwardClientKey:      true,
			AllowClientKeyMissing: true,
			Upstreams:             []Upstream{{Name: "test", URL: "https://api.example.com"}},
		})
	})
}

func Test_AuthSchemes(t *testing.T) {
	t.Parallel()

	require.Equal(t, AuthScheme{Header: fiber.HeaderAuthorization, Scheme: "Bearer"}, AuthBearer())
	require.Equal(t, AuthScheme{Header: "x-api-key"}, AuthHeader("x-api-key"))
}

func Test_ProviderPresets(t *testing.T) {
	t.Parallel()

	openai := OpenAI("sk-1")
	require.Equal(t, "openai", openai.Name)
	require.Equal(t, "https://api.openai.com", openai.URL)
	require.Equal(t, AuthBearer(), openai.Auth)
	require.Equal(t, "sk-1", openai.Key)

	anthropic := Anthropic("sk-2")
	require.Equal(t, "anthropic", anthropic.Name)
	require.Equal(t, "https://api.anthropic.com", anthropic.URL)
	require.Equal(t, AuthHeader("x-api-key"), anthropic.Auth)
	require.Equal(t, "sk-2", anthropic.Key)

	openrouter := OpenRouter("sk-3")
	require.Equal(t, "openrouter", openrouter.Name)
	require.Equal(t, "https://openrouter.ai/api", openrouter.URL)
	require.Equal(t, AuthBearer(), openrouter.Auth)

	azure := AzureOpenAI("https://my-res.openai.azure.com/", "sk-4")
	require.Equal(t, "azure-openai", azure.Name)
	require.Equal(t, AuthHeader("api-key"), azure.Auth)
	// The preset keeps the endpoint as given; configDefault trims the slash.
	cfg := configDefault(Config{Upstreams: []Upstream{azure}})
	require.Equal(t, "https://my-res.openai.azure.com", cfg.Upstreams[0].URL)
}

func Test_StripPrefix(t *testing.T) {
	t.Parallel()

	require.Equal(t, "/v1/chat", stripPrefix("/openai/v1/chat", "/openai"))
	require.Equal(t, "/", stripPrefix("/openai", "/openai"))
	require.Equal(t, "/v1/chat", stripPrefix("/v1/chat", ""))
	require.Equal(t, "/v1/chat", stripPrefix("/v1/chat", "/other"))
	// Prefix matching inside a segment is not a mount-point match.
	require.Equal(t, "/openai/v1", stripPrefix("/openai/v1", "/open"))
}

func Test_ContainsDotDot(t *testing.T) {
	t.Parallel()

	require.True(t, containsDotDot("/v1/../admin"))
	require.True(t, containsDotDot("/.."))
	require.False(t, containsDotDot("/v1/chat"))
	require.False(t, containsDotDot("/v1/..chat"))
}

func Test_MatchAny(t *testing.T) {
	t.Parallel()

	require.True(t, matchAny([]string{"gpt-4o"}, "gpt-4o"))
	require.True(t, matchAny([]string{"gpt-4o*"}, "gpt-4o-mini"))
	require.False(t, matchAny([]string{"gpt-4o"}, "gpt-4o-mini"))
	require.False(t, matchAny([]string{"gpt-4o*"}, "claude-3"))
	require.False(t, matchAny([]string{"gpt-4o*"}, ""))
	require.False(t, matchAny([]string{""}, "anything"))
}
