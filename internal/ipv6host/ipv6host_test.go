package ipv6host

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Canonical(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ host, want string }{
		{host: "[::1]", want: "[::1]"},
		{host: "[0:0:0:0:0:0:0:1]:3000", want: "[::1]:3000"},
		{host: "[2001:0DB8:0000::0001]", want: "[2001:db8::1]"},
		{host: "[2001:db8:0:1:0:0:0:1]", want: "[2001:db8:0:1::1]"},
		{host: "[2001:db8:0:0:1:0:0:1]", want: "[2001:db8::1:0:0:1]"},
		{host: "[2001:db8::0:1]", want: "[2001:db8::1]"},
		{host: "[1:0:2:3:4:5:6:7]", want: "[1:0:2:3:4:5:6:7]"},
		{host: "[::ffff:127.0.0.1]", want: "[::ffff:7f00:1]"},
		{host: "[::ffff:7f00:1]", want: "[::ffff:7f00:1]"},
		{host: "[64:ff9b::192.0.2.33]", want: "[64:ff9b::c000:221]"},
		{host: "[::1.2.3.4]", want: "[::102:304]"},
		{host: "[fe80:0::1%25eth0]", want: "[fe80:0::1%25eth0]"},
		{host: "[fe80::1%eth0]", want: "[fe80::1%eth0]"},
		{host: "[allowed.com]", want: "[allowed.com]"},
		{host: "[127.0.0.1]", want: "[127.0.0.1]"},
		{host: "[0::1", want: "[0::1"},
		{host: "example.com", want: "example.com"},
		{host: "0:0::1", want: "0:0::1"},
		{host: "", want: ""},
	} {
		t.Run(tc.host, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Canonical(tc.host))
		})
	}
}

// AllocsPerRun panics under t.Parallel, so this test stays sequential.
func Test_Canonical_Allocations(t *testing.T) {
	for _, host := range []string{"[::1]:3000", "[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]", "[::ffff:7f00:1]", "example.com"} {
		t.Run(host, func(t *testing.T) {
			require.Zero(t, testing.AllocsPerRun(100, func() { Canonical(host) }))
		})
	}
}

// Every spelling parses back to its address, and the canonical one is a fixed point.
func FuzzString(f *testing.F) {
	for _, seed := range []string{"::1", "0:0:0:0:0:0:0:1", "2001:DB8::1", "::ffff:127.0.0.1", "64:ff9b::1.2.3.4", "fe80::1%eth0", "1:0:2:3:4:5:6:7"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		t.Parallel()
		addr, ok := Parse(s)
		if !ok {
			return
		}
		canonical := String(addr, s)
		back, err := netip.ParseAddr(canonical)
		require.NoError(t, err)
		require.Equal(t, addr, back)
		require.Equal(t, canonical, String(back, canonical))
		require.Equal(t, strings.ToLower(canonical), canonical)
		if !addr.Is4In6() {
			require.Equal(t, addr.String(), canonical)
		}
	})
}
