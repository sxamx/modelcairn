package openaiadapter

import (
	"net/netip"
	"testing"
)

func TestForbiddenAddressPolicy(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.96.178.46", "::1", "fd00::1"} {
		if !forbiddenAddress(netip.MustParseAddr(value)) {
			t.Errorf("%s should require allowPrivateNetwork", value)
		}
	}
	for _, value := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if forbiddenAddress(netip.MustParseAddr(value)) {
			t.Errorf("%s should be public", value)
		}
	}
}
