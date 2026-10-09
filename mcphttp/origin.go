package mcphttp

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

// sameMCPOrigin compares addresses without DNS lookup. IPv6 zones name local
// interfaces and are case-sensitive, unlike DNS names or IPv6 hex digits.
func sameMCPOrigin(a, b *url.URL) bool {
	if !strings.EqualFold(a.Scheme, b.Scheme) ||
		(!strings.EqualFold(a.Scheme, "http") && !strings.EqualFold(a.Scheme, "https")) {
		return false
	}
	hostA, hostB := a.Hostname(), b.Hostname()
	if hostA == "" || hostB == "" {
		return false
	}
	ipA, errA := netip.ParseAddr(hostA)
	ipB, errB := netip.ParseAddr(hostB)
	if errA == nil && errB == nil {
		if ipA != ipB {
			return false
		}
	} else {
		if strings.ContainsAny(hostA+hostB, ":%") {
			return false
		}
		dnsA, errA := idna.Lookup.ToASCII(hostA)
		dnsB, errB := idna.Lookup.ToASCII(hostB)
		if errA != nil || errB != nil || !strings.EqualFold(dnsA, dnsB) {
			return false
		}
	}
	port := func(u *url.URL) int {
		if raw := u.Port(); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 65535 {
				return -1
			}
			return value
		}
		if strings.EqualFold(u.Scheme, "https") {
			return 443
		}
		return 80
	}
	portA, portB := port(a), port(b)
	return portA >= 0 && portA == portB
}
