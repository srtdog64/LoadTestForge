package strategy

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"sync"
	"syscall"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/raw"
)

const (
	IPPROTO_RAW = 255
	IP_HDRINCL  = 2
)

type RawStrategy struct {
	BaseStrategy
	templatePath string
	template     *raw.Template
	spoofIPs     []string
	randomSpoof  bool
	socketFD     syscall.Handle // For Windows raw socket
	bufferPool   *sync.Pool
}

func NewRawStrategy(cfg *config.StrategyConfig, bindIP string, templatePath string) *RawStrategy {
	loader := raw.NewLoader(".")
	tmpl, _ := loader.Load(templatePath)

	s := &RawStrategy{
		BaseStrategy: NewBaseStrategyFromConfig(cfg, bindIP),
		templatePath: templatePath,
		template:     tmpl,
		spoofIPs:     cfg.SpoofIPs,
		randomSpoof:  cfg.RandomSpoof,
		socketFD:     syscall.InvalidHandle,
		bufferPool: &sync.Pool{
			New: func() interface{} {
				// Allocate buffer with size of template + margin if needed
				if tmpl != nil {
					buf := make([]byte, len(tmpl.Raw))
					copy(buf, tmpl.Raw)
					return buf
				}
				return make([]byte, 1500)
			},
		},
	}

	// Try to initialize raw socket once
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, IPPROTO_RAW)
	if err == nil {
		// Set IP_HDRINCL
		if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, IP_HDRINCL, 1); err == nil {
			s.socketFD = fd
		} else {
			syscall.Close(fd)
		}
	}

	return s
}

func (s *RawStrategy) Execute(ctx context.Context, target Target) error {
	// Resolve Target
	u, err := url.Parse(target.URL)
	if err != nil {
		return err
	}

	hostname := u.Hostname()
	dstIPVec, err := net.LookupIP(hostname)
	if err != nil || len(dstIPVec) == 0 {
		return err
	}
	dstIP := dstIPVec[0]

	dstPort := 80
	if port := u.Port(); port != "" {
		fmt.Sscanf(port, "%d", &dstPort)
	}

	// Determine Source IP
	srcIP := net.ParseIP("127.0.0.1")

	if s.randomSpoof {
		// Generate Random IP
		srcIP = net.IPv4(byte(rand.Intn(223)+1), byte(rand.Intn(256)), byte(rand.Intn(256)), byte(rand.Intn(255)))
	} else if len(s.spoofIPs) > 0 {
		// Pick random from spoof list
		pktSrc := s.spoofIPs[rand.Intn(len(s.spoofIPs))]
		srcIP = net.ParseIP(pktSrc)
	} else if s.BindConfig != nil {
		addr := s.BindConfig.GetLocalAddr()
		if addr != nil {
			srcIP = addr.IP
		}
	}

	// Build Packet using Pool
	if s.template == nil {
		return fmt.Errorf("no template")
	}

	packet := s.bufferPool.Get().([]byte)
	defer s.bufferPool.Put(packet)

	// Update packet (init=true only if we suspect pool gave garbage, but New() gives clean? No, New gives zeroes.
	// Actually New() gives zeroes, so we need init=true for new buffers.
	// But reused buffers have old data.
	// We can't easily distinguish new vs reused in sync.Pool without a wrapper.
	// OPTIMIZATION: Always init=true is safer but slower (copy). Use init=true for now to be safe,
	// or rely on overwrite? UpdatePacket overwrites variables. Constants are overwritten if init=true.
	// To support zero-copy fully, we'd need to trust the buffer state.
	// Let's use init=true to ensure correctness first, as it's still way faster than make().
	// Wait, if I use init=true, I copy t.Raw every time. That's a memory copy.
	// For max performance, we want init=false.
	// But new buffers (from New) are empty.
	// If I check if packet[0] == 0 (assuming template starts with non-zero)? Risky.
	// Correct approach: Initialize in New().
	// But New() cannot modify the buffer after allocation easily without knowing t.Raw? New() knows t.Raw!
	// So New() should copy t.Raw!

	// I updated New() to copy t.Raw? No, code above `make([]byte, len(tmpl.Raw))` is just allocation.
	// I will update New() to copy t.Raw.

	s.template.UpdatePacket(packet, raw.PacketParams{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: 0, // Random
		DstPort: dstPort,
	}, false) // init=false because we handle init in Pool.New or assume init

	return s.sendRaw(packet, dstIP, dstPort)
}

func (s *RawStrategy) sendRaw(packet []byte, dstIP net.IP, dstPort int) error {
	// Strip L2 header if present - raw IP socket expects IP header first
	sendPacket := packet
	if s.template != nil && s.template.HasL2Header {
		sendPacket = s.template.GetPacketWithoutL2(packet)
	}

	// Use pre-initialized socket if available
	if s.socketFD != syscall.InvalidHandle {
		// Destination Address
		addr := syscall.SockaddrInet4{
			Port: dstPort,
		}
		copy(addr.Addr[:], dstIP.To4())

		err := syscall.Sendto(s.socketFD, sendPacket, 0, &addr)
		if err != nil {
			return err
		}
		s.IncrementConnections()
		return nil
	}

	// Fallback to UDP if raw socket failed to init
	return s.sendUDP(packet, dstIP, dstPort)
}

func (s *RawStrategy) sendUDP(packet []byte, dstIP net.IP, dstPort int) error {
	// Strip L2 header if present, then strip IP header for UDP payload
	payload := packet
	if s.template != nil && s.template.HasL2Header {
		payload = s.template.GetPacketWithoutL2(packet) // Remove L2 (14 bytes)
	}
	// Strip IP header (20 bytes) - UDP socket adds its own
	if len(payload) > 28 { // IP(20) + UDP(8)
		payload = payload[28:] // Send only UDP payload
	}

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", dstIP.String(), dstPort))
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(payload)
	if err != nil {
		return err
	}

	s.IncrementConnections()
	return nil
}

func (s *RawStrategy) Name() string {
	return "raw"
}
