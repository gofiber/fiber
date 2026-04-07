package hostauthorization

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// --- Config tests ---

func Test_ConfigDefault(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		AllowedHosts: []string{"example.com"},
	})
	require.NotNil(t, cfg.ErrorHandler)
	require.Equal(t, []string{"example.com"}, cfg.AllowedHosts)
}

func Test_ConfigPanicNoHostsOrFunc(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		configDefault(Config{})
	})
}

func Test_ConfigPanicEmptySlice(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		configDefault(Config{
			AllowedHosts: []string{},
		})
	})
}

func Test_ConfigNoDefaultArgs(t *testing.T) {
	t.Parallel()

	cfg := configDefault()
	require.Equal(t, ConfigDefault, cfg)
}

func Test_ConfigAllowedHostsFuncOnly(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		AllowedHostsFunc: func(host string) bool {
			return host == "example.com"
		},
	})
	require.NotNil(t, cfg.AllowedHostsFunc)
}

func Test_ConfigPanicInvalidCIDR(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			AllowedHosts: []string{"not-a-cidr/99"},
		})
	})
}

func Test_ConfigCustomErrorHandler(t *testing.T) {
	t.Parallel()

	custom := func(c fiber.Ctx, _ error) error {
		return c.Status(fiber.StatusTeapot).SendString("nope")
	}

	cfg := configDefault(Config{
		AllowedHosts: []string{"example.com"},
		ErrorHandler: custom,
	})
	require.NotNil(t, cfg.ErrorHandler)
}

// --- normalizeHost tests ---

func Test_NormalizeHost(t *testing.T) {
	t.Parallel()

	// normalizeHost receives output from c.Hostname() which already strips ports.
	// It handles trailing dots, IPv6 brackets, and lowercasing.
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain host", "example.com", "example.com"},
		{"uppercase", "EXAMPLE.COM", "example.com"},
		{"trailing dot", "example.com.", "example.com"},
		{"ipv4", "192.168.1.1", "192.168.1.1"},
		{"ipv6 brackets", "[::1]", "::1"},
		{"ipv6 bare", "::1", "::1"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, normalizeHost(tt.input))
		})
	}
}

// --- Matching logic tests ---

func Test_MatchExact(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"example.com", "api.myapp.com"})

	require.True(t, matchHost("example.com", parsed, nil))
	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.False(t, matchHost("evil.com", parsed, nil))
}

func Test_MatchExactCaseInsensitive(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"Example.COM"})

	require.True(t, matchHost("example.com", parsed, nil))
}

func Test_MatchSubdomainWildcard(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{".myapp.com"})

	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.True(t, matchHost("www.myapp.com", parsed, nil))
	require.True(t, matchHost("deep.sub.myapp.com", parsed, nil))
	require.False(t, matchHost("myapp.com", parsed, nil), "bare domain must NOT match subdomain wildcard")
	require.False(t, matchHost("evil.com", parsed, nil))
}

func Test_MatchCIDRv4(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"10.0.0.0/8"})

	require.True(t, matchHost("10.0.50.3", parsed, nil))
	require.True(t, matchHost("10.255.255.255", parsed, nil))
	require.False(t, matchHost("192.168.1.1", parsed, nil))
	require.False(t, matchHost("169.254.169.254", parsed, nil))
}

func Test_MatchCIDRv6(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"fd00::/8"})

	require.True(t, matchHost("fd00::1", parsed, nil))
	require.False(t, matchHost("2001:db8::1", parsed, nil))
}

func Test_MatchAllowedHostsFunc(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"example.com"})
	fn := func(host string) bool {
		return host == "dynamic.com"
	}

	require.True(t, matchHost("example.com", parsed, fn))
	require.True(t, matchHost("dynamic.com", parsed, fn))
	require.False(t, matchHost("evil.com", parsed, fn))
}

func Test_MatchMixedCategories(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{
		"example.com",
		".myapp.com",
		"10.0.0.0/8",
		"127.0.0.1",
	})

	require.True(t, matchHost("example.com", parsed, nil))
	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.True(t, matchHost("10.0.50.3", parsed, nil))
	require.True(t, matchHost("127.0.0.1", parsed, nil))
	require.False(t, matchHost("evil.com", parsed, nil))
	require.False(t, matchHost("192.168.1.1", parsed, nil))
}

func Test_MatchEmptyHost(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"example.com"})

	require.False(t, matchHost("", parsed, nil))
}
