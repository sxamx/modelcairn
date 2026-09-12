package openaiadapter

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

func newDirectTransport(allowPrivate bool) *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("split upstream address: %w", err)
			}
			addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("resolve upstream: %w", err)
			}
			var failures []error
			for _, address := range addresses {
				if !allowPrivate && forbiddenAddress(address) {
					failures = append(failures, errors.New("resolved address is not public"))
					continue
				}
				connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				failures = append(failures, dialErr)
			}
			if len(failures) == 0 {
				return nil, errors.New("upstream resolved to no addresses")
			}
			return nil, errors.Join(failures...)
		},
	}
}

func forbiddenAddress(address netip.Addr) bool {
	return address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() || address.IsUnspecified() || address.IsMulticast() ||
		sharedAddressSpace.Contains(address)
}
