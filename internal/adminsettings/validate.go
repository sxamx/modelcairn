package adminsettings

import (
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Validate(v Resolved) error {
	if v.TrustedProxyCIDRs == nil {
		return failure(CodeInvalidStructure, "$.spec.trustedProxyCidrs")
	}
	if !validString(v.PublicOrigin, 2048) {
		return failure(CodeInvalidValue, "$.spec.publicOrigin")
	}
	if !validString(v.Listen, 128) {
		return failure(CodeInvalidValue, "$.spec.listen")
	}
	if !validOptionalString(v.TLSCertificatePath, 4096) {
		return failure(CodeInvalidValue, "$.spec.tlsCertificatePath")
	}
	if !validOptionalString(v.TLSPrivateKeyPath, 4096) {
		return failure(CodeInvalidValue, "$.spec.tlsPrivateKeyPath")
	}
	origin, err := canonicalOrigin(v.PublicOrigin)
	if err != nil {
		return failure(CodeInvalidValue, "$.spec.publicOrigin")
	}
	listenIP, err := canonicalListen(v.Listen)
	if err != nil {
		return failure(CodeInvalidValue, "$.spec.listen")
	}
	if v.IdleSeconds < 300 || v.IdleSeconds > 86400 {
		return failure(CodeInvalidValue, "$.spec.idleSeconds")
	}
	if v.AbsoluteSeconds < 300 || v.AbsoluteSeconds > 604800 {
		return failure(CodeInvalidValue, "$.spec.absoluteSeconds")
	}
	if v.IdleSeconds > v.AbsoluteSeconds {
		return failure(CodeInvalidValue, "$.spec.idleSeconds")
	}
	if !within(v.GlobalAttemptsPerMinute, 1, 120) {
		return failure(CodeInvalidValue, "$.spec.globalAttemptsPerMinute")
	}
	if !within(v.GlobalBurst, 1, 20) {
		return failure(CodeInvalidValue, "$.spec.globalBurst")
	}
	if !within(v.ClientAttemptsPerMinute, 1, 30) || v.ClientAttemptsPerMinute > v.GlobalAttemptsPerMinute {
		return failure(CodeInvalidValue, "$.spec.clientAttemptsPerMinute")
	}
	if !within(v.ClientBurst, 1, 10) || v.ClientBurst > v.GlobalBurst {
		return failure(CodeInvalidValue, "$.spec.clientBurst")
	}
	if !within(v.MaxClientEntries, 64, 4096) {
		return failure(CodeInvalidValue, "$.spec.maxClientEntries")
	}
	if !within(v.ClientIdleSeconds, 60, 3600) {
		return failure(CodeInvalidValue, "$.spec.clientIdleSeconds")
	}
	if v.FailedLoginRetentionSeconds < 0 || v.FailedLoginRetentionSeconds > 3155760000 {
		return failure(CodeInvalidValue, "$.spec.failedLoginRetentionSeconds")
	}
	if !within(v.ArgonMemoryKiB, 19456, 65536) {
		return failure(CodeInvalidValue, "$.spec.argonMemoryKiB")
	}
	if !within(v.ArgonIterations, 2, 6) {
		return failure(CodeInvalidValue, "$.spec.argonIterations")
	}
	if len(v.TrustedProxyCIDRs) > 32 {
		return failure(CodeInvalidValue, "$.spec.trustedProxyCidrs")
	}
	seen := map[string]struct{}{}
	for _, raw := range v.TrustedProxyCIDRs {
		if !validString(raw, 64) {
			return failure(CodeInvalidValue, "$.spec.trustedProxyCidrs")
		}
		prefix, e := netip.ParsePrefix(raw)
		if e != nil || prefix.String() != raw || prefix != prefix.Masked() {
			return failure(CodeInvalidValue, "$.spec.trustedProxyCidrs")
		}
		if _, ok := seen[raw]; ok {
			return failure(CodeInvalidValue, "$.spec.trustedProxyCidrs")
		}
		seen[raw] = struct{}{}
	}
	switch v.Transport {
	case LoopbackHTTP:
		if origin.Scheme != "http" || !originIsLoopback(origin) || !listenIP.IsLoopback() {
			return failure(CodeInvalidValue, "$.spec.transport")
		}
		if len(v.TrustedProxyCIDRs) > 0 || v.TLSCertificatePath != "" || v.TLSPrivateKeyPath != "" {
			return failure(CodeInvalidValue, "$.spec.transport")
		}
	case DirectTLS:
		if origin.Scheme != "https" || len(v.TrustedProxyCIDRs) > 0 {
			return failure(CodeInvalidValue, "$.spec.transport")
		}
		if !validAbsolutePath(v.TLSCertificatePath) {
			return failure(CodeInvalidValue, "$.spec.tlsCertificatePath")
		}
		if !validAbsolutePath(v.TLSPrivateKeyPath) {
			return failure(CodeInvalidValue, "$.spec.tlsPrivateKeyPath")
		}
	case ProxyTLS:
		if origin.Scheme != "https" || len(v.TrustedProxyCIDRs) == 0 || v.TLSCertificatePath != "" || v.TLSPrivateKeyPath != "" {
			return failure(CodeInvalidValue, "$.spec.transport")
		}
	default:
		return failure(CodeInvalidValue, "$.spec.transport")
	}
	return nil
}
func within(value, min, max int) bool { return value >= min && value <= max }
func validString(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum
}
func validOptionalString(value string, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum
}
func validAbsolutePath(value string) bool {
	return value != "" && !strings.ContainsRune(value, '\x00') && filepath.IsAbs(value)
}
func canonicalListen(raw string) (netip.Addr, error) {
	host, portRaw, err := net.SplitHostPort(raw)
	if err != nil {
		return netip.Addr{}, err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return netip.Addr{}, url.InvalidHostError(raw)
	}
	if net.JoinHostPort(ip.String(), strconv.Itoa(port)) != raw {
		return netip.Addr{}, url.InvalidHostError(raw)
	}
	return ip, nil
}
func canonicalOrigin(raw string) (*url.URL, error) {
	u, canonical, err := normalizedOrigin(raw)
	if err != nil || raw != canonical {
		return nil, url.InvalidHostError(raw)
	}
	return u, nil
}

func normalizePublicOrigin(raw string) (string, error) {
	_, canonical, err := normalizedOrigin(raw)
	return canonical, err
}

func normalizedOrigin(raw string) (*url.URL, string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, "", url.InvalidHostError(raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, "", url.InvalidHostError(raw)
	}
	hostname := u.Hostname()
	if hostname == "" || strings.Contains(hostname, "*") {
		return nil, "", url.InvalidHostError(raw)
	}
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	host := strings.ToLower(hostname)
	if ip, parseErr := netip.ParseAddr(hostname); parseErr == nil {
		host = ip.String()
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		parsed, e := strconv.Atoi(port)
		if e != nil || parsed < 1 || parsed > 65535 {
			return nil, "", url.InvalidHostError(raw)
		}
		host = net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(parsed))
	}
	canonical := scheme + "://" + host
	return &url.URL{Scheme: scheme, Host: host}, canonical, nil
}
func originIsLoopback(u *url.URL) bool {
	if u.Hostname() == "localhost" {
		return true
	}
	ip, _ := netip.ParseAddr(u.Hostname())
	return ip.IsLoopback()
}
