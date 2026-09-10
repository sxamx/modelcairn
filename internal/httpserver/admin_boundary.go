package httpserver

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/sxamx/modelcairn/internal/adminsettings"
)

var errAdminBoundary = errors.New("admin_boundary_rejected")

type adminBoundary struct {
	origin    string
	host      string
	transport adminsettings.Transport
	trusted   []netip.Prefix
}

func newAdminBoundary(settings adminsettings.Resolved) (adminBoundary, error) {
	if err := adminsettings.Validate(settings); err != nil {
		return adminBoundary{}, err
	}
	origin, err := url.Parse(settings.PublicOrigin)
	if err != nil {
		return adminBoundary{}, err
	}
	b := adminBoundary{origin: settings.PublicOrigin, host: origin.Host, transport: settings.Transport}
	for _, raw := range settings.TrustedProxyCIDRs {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return adminBoundary{}, err
		}
		b.trusted = append(b.trusted, prefix)
	}
	return b, nil
}

func (b adminBoundary) client(r *http.Request) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, errAdminBoundary
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, errAdminBoundary
	}
	peer = peer.Unmap()
	if !strings.EqualFold(r.Host, b.host) {
		return netip.Addr{}, errAdminBoundary
	}
	forwardedFor := headerValues(r, "X-Forwarded-For")
	forwardedProto := headerValues(r, "X-Forwarded-Proto")
	switch b.transport {
	case adminsettings.LoopbackHTTP:
		if !peer.IsLoopback() || r.TLS != nil || len(forwardedFor) != 0 || len(forwardedProto) != 0 {
			return netip.Addr{}, errAdminBoundary
		}
		return peer, nil
	case adminsettings.DirectTLS:
		if r.TLS == nil || len(forwardedFor) != 0 || len(forwardedProto) != 0 {
			return netip.Addr{}, errAdminBoundary
		}
		return peer, nil
	case adminsettings.ProxyTLS:
		if r.TLS != nil || !b.trusts(peer) || len(forwardedFor) != 1 || len(forwardedProto) != 1 || forwardedProto[0] != "https" || strings.Contains(forwardedFor[0], ",") {
			return netip.Addr{}, errAdminBoundary
		}
		client, err := netip.ParseAddr(forwardedFor[0])
		if err != nil || client.IsUnspecified() {
			return netip.Addr{}, errAdminBoundary
		}
		return client.Unmap(), nil
	default:
		return netip.Addr{}, errAdminBoundary
	}
}

func (b adminBoundary) requireOrigin(r *http.Request) error {
	values := headerValues(r, "Origin")
	if len(values) != 1 || values[0] != b.origin {
		return errAdminBoundary
	}
	return nil
}

func (b adminBoundary) requireRecoveryOrigin(r *http.Request) error {
	values := headerValues(r, "Origin")
	if len(values) > 0 {
		if len(values) != 1 || values[0] != b.origin {
			return errAdminBoundary
		}
		return nil
	}
	fetch := headerValues(r, "Sec-Fetch-Site")
	if len(fetch) != 1 || fetch[0] != "same-origin" {
		return errAdminBoundary
	}
	return nil
}

func (b adminBoundary) trusts(address netip.Addr) bool {
	for _, prefix := range b.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
func headerValues(r *http.Request, name string) []string {
	// Preserve malformed values: treating them as absent could enable fallback
	// to another origin signal or accept forwarded headers on direct transports.
	return r.Header.Values(name)
}

func sessionCookie(r *http.Request) (string, error) {
	value := ""
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == "mc_session" {
			count++
			value = cookie.Value
		}
	}
	if count != 1 || value == "" {
		return "", errors.New("admin_session_invalid")
	}
	return value, nil
}
