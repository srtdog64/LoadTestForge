package netutil

import (
	"net"
)

// NewLocalTCPAddr creates a TCP address for binding outbound connections.
// If bindIP is empty, returns nil (system default).
func NewLocalTCPAddr(bindIP string) *net.TCPAddr {
	if bindIP == "" {
		return nil
	}

	ip := net.ParseIP(bindIP)
	if ip == nil {
		return nil
	}

	return &net.TCPAddr{IP: ip}
}

// IsValidIP checks if the given string is a valid IP address.
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// ResolveHost resolves a hostname to its IP addresses.
func ResolveHost(host string) ([]net.IP, error) {
	return net.LookupIP(host)
}
