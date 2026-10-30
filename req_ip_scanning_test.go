package fiber

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Ctx_ForwardedIPScanning(t *testing.T) {
	t.Parallel()
	const remoteIP = "10.0.0.2"
	for _, tc := range []struct {
		name             string
		header           string
		firstValidIP     string
		firstUntrustedIP string
		ips              []string
		validatedIPs     []string
	}{
		{
			name: "empty header", header: "", ips: []string{}, validatedIPs: []string{},
			firstValidIP: remoteIP, firstUntrustedIP: remoteIP,
		},
		{
			name: "only commas", header: ",,,", ips: []string{"", ""}, validatedIPs: []string{},
			firstValidIP: remoteIP, firstUntrustedIP: remoteIP,
		},
		{
			name: "leading and trailing commas", header: ",203.0.113.50,",
			ips: []string{"203.0.113.50"}, validatedIPs: []string{"203.0.113.50"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "two adjacent commas", header: "203.0.113.50,,10.0.0.1",
			ips: []string{"203.0.113.50", "10.0.0.1"}, validatedIPs: []string{"203.0.113.50", "10.0.0.1"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "three adjacent commas", header: "203.0.113.50,,,10.0.0.1",
			ips: []string{"203.0.113.50", "", "10.0.0.1"}, validatedIPs: []string{"203.0.113.50", "10.0.0.1"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "whitespace-only elements", header: " \t,\t203.0.113.50\t, \t",
			ips: []string{"", "203.0.113.50", ""}, validatedIPs: []string{"203.0.113.50"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "non-OWS stays invalid", header: "\v203.0.113.50, ::ffff:192.0.2.1",
			ips: []string{"\v203.0.113.50", "::ffff:192.0.2.1"}, validatedIPs: []string{"::ffff:192.0.2.1"},
			firstValidIP: "::ffff:192.0.2.1", firstUntrustedIP: "::ffff:192.0.2.1",
		},
		{
			name: "selection direction", header: "198.51.100.9,,203.0.113.50,10.0.0.1,10.0.0.2,",
			ips:          []string{"198.51.100.9", "203.0.113.50", "10.0.0.1", "10.0.0.2"},
			validatedIPs: []string{"198.51.100.9", "203.0.113.50", "10.0.0.1", "10.0.0.2"},
			firstValidIP: "198.51.100.9", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "only trusted valid IPs", header: "invalid,,10.0.0.1,10.0.0.2",
			ips: []string{"invalid", "10.0.0.1", "10.0.0.2"}, validatedIPs: []string{"10.0.0.1", "10.0.0.2"},
			firstValidIP: "10.0.0.1", firstUntrustedIP: "10.0.0.1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, mode := range []string{"raw", "validated", "trusted"} {
				t.Run(mode, func(t *testing.T) {
					t.Parallel()
					config := Config{ProxyHeader: HeaderXForwardedFor, EnableIPValidation: mode != "raw"}
					if mode == "trusted" {
						config.TrustProxy = true
						config.TrustProxyConfig.Proxies = []string{"10.0.0.1", remoteIP}
					}
					app := New(config)
					fc := &fasthttp.RequestCtx{}
					fc.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP(remoteIP)})
					c := app.AcquireCtx(fc)
					defer app.ReleaseCtx(c)
					c.Request().Header.Set(HeaderXForwardedFor, tc.header)

					wantIPs, wantIP := tc.validatedIPs, tc.firstValidIP
					switch mode {
					case "raw":
						wantIPs, wantIP = tc.ips, tc.header
					case "trusted":
						wantIP = tc.firstUntrustedIP
					}
					require.Equal(t, wantIPs, c.IPs())
					require.Equal(t, wantIP, c.extractIPFromHeader(HeaderXForwardedFor))
				})
			}
		})
	}
}
