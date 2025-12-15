package netutil

import (
	"net"
	"sync/atomic"

	"github.com/srtdog64/loadtestforge/internal/randutil"
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

// IPPool manages multiple source IPs for round-robin binding.
// Thread-safe for concurrent access.
type IPPool struct {
	ips     []net.IP
	addrs   []*net.TCPAddr
	counter uint64
}

// NewIPPool creates an IP pool from comma-separated IP string.
// Returns nil if input is empty or contains no valid IPs.
func NewIPPool(bindIPs string) *IPPool {
	if bindIPs == "" {
		return nil
	}

	var ips []net.IP
	var addrs []*net.TCPAddr

	for _, ipStr := range splitIPs(bindIPs) {
		ip := net.ParseIP(ipStr)
		if ip != nil {
			ips = append(ips, ip)
			addrs = append(addrs, &net.TCPAddr{IP: ip})
		}
	}

	if len(ips) == 0 {
		return nil
	}

	return &IPPool{
		ips:   ips,
		addrs: addrs,
	}
}

// NewIPPoolFromSlice creates an IP pool from a slice of IP strings.
func NewIPPoolFromSlice(bindIPs []string) *IPPool {
	if len(bindIPs) == 0 {
		return nil
	}

	var ips []net.IP
	var addrs []*net.TCPAddr

	for _, ipStr := range bindIPs {
		ip := net.ParseIP(ipStr)
		if ip != nil {
			ips = append(ips, ip)
			addrs = append(addrs, &net.TCPAddr{IP: ip})
		}
	}

	if len(ips) == 0 {
		return nil
	}

	return &IPPool{
		ips:   ips,
		addrs: addrs,
	}
}

// Next returns the next IP address in round-robin fashion.
// Thread-safe.
func (p *IPPool) Next() net.IP {
	if p == nil || len(p.ips) == 0 {
		return nil
	}

	idx := atomic.AddUint64(&p.counter, 1) - 1
	return p.ips[idx%uint64(len(p.ips))]
}

// NextAddr returns the next TCP address in round-robin fashion.
// Thread-safe.
func (p *IPPool) NextAddr() *net.TCPAddr {
	if p == nil || len(p.addrs) == 0 {
		return nil
	}

	idx := atomic.AddUint64(&p.counter, 1) - 1
	return p.addrs[idx%uint64(len(p.addrs))]
}

// GetRandomAddr returns a random TCP address from the pool.
// Thread-safe and high-performance using randutil.
func (p *IPPool) GetRandomAddr() *net.TCPAddr {
	if p == nil || len(p.addrs) == 0 {
		return nil
	}

	idx := randutil.Intn(len(p.addrs))
	return p.addrs[idx]
}

// Get returns IP at specific index (for worker assignment).
func (p *IPPool) Get(index int) net.IP {
	if p == nil || len(p.ips) == 0 {
		return nil
	}
	return p.ips[index%len(p.ips)]
}

// GetAddr returns TCP address at specific index (for worker assignment).
func (p *IPPool) GetAddr(index int) *net.TCPAddr {
	if p == nil || len(p.addrs) == 0 {
		return nil
	}
	return p.addrs[index%len(p.addrs)]
}

// Len returns the number of IPs in the pool.
func (p *IPPool) Len() int {
	if p == nil {
		return 0
	}
	return len(p.ips)
}

// IPs returns all IPs in the pool.
func (p *IPPool) IPs() []net.IP {
	if p == nil {
		return nil
	}
	return p.ips
}

// String returns comma-separated list of IPs.
func (p *IPPool) String() string {
	if p == nil || len(p.ips) == 0 {
		return ""
	}

	result := p.ips[0].String()
	for i := 1; i < len(p.ips); i++ {
		result += "," + p.ips[i].String()
	}
	return result
}

func splitIPs(s string) []string {
	var result []string
	var current string

	for _, c := range s {
		if c == ',' || c == ' ' || c == ';' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// BindConfig holds binding configuration for connections.
type BindConfig struct {
	Pool     *IPPool
	SingleIP string
	counter  uint64
	Random   bool
}

// NewBindConfig creates a binding configuration.
// Supports both single IP and multiple IPs.
func NewBindConfig(bindIPs string) *BindConfig {
	pool := NewIPPool(bindIPs)

	if pool != nil && pool.Len() > 1 {
		return &BindConfig{Pool: pool}
	}

	return &BindConfig{SingleIP: bindIPs}
}

// GetLocalAddr returns the next local address for binding.
// Supports round-robin (default) or random selection.
func (b *BindConfig) GetLocalAddr() *net.TCPAddr {
	if b == nil {
		return nil
	}

	if b.Pool != nil {
		if b.Random {
			return b.Pool.GetRandomAddr()
		}
		return b.Pool.NextAddr()
	}

	return NewLocalTCPAddr(b.SingleIP)
}

// GetLocalAddrForWorker returns a local address assigned to specific worker.
// Each worker gets a consistent IP based on its index.
func (b *BindConfig) GetLocalAddrForWorker(workerIdx int) *net.TCPAddr {
	if b == nil {
		return nil
	}

	if b.Pool != nil {
		return b.Pool.GetAddr(workerIdx)
	}

	return NewLocalTCPAddr(b.SingleIP)
}

// HasMultipleIPs returns true if pool has more than one IP.
func (b *BindConfig) HasMultipleIPs() bool {
	return b != nil && b.Pool != nil && b.Pool.Len() > 1
}

// Count returns the number of IPs available.
func (b *BindConfig) Count() int {
	if b == nil {
		return 0
	}

	if b.Pool != nil {
		return b.Pool.Len()
	}

	if b.SingleIP != "" {
		return 1
	}

	return 0
}
