package strategy

import (
	"net"
)

// newLocalTCPAddr creates a TCP address for binding outbound connections.
// If bindIP is empty, returns nil (system default).
func newLocalTCPAddr(bindIP string) *net.TCPAddr {
	if bindIP == "" {
		return nil
	}

	ip := net.ParseIP(bindIP)
	if ip == nil {
		return nil
	}

	return &net.TCPAddr{IP: ip}
}
