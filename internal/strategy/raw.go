package strategy

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
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
	socketFD     syscall.Handle // For Windows raw socket (keep specific handle type if needed)
}

func NewRawStrategy(cfg *config.StrategyConfig, bindIP string, templatePath string) *RawStrategy {
	loader := raw.NewLoader(".")
	tmpl, _ := loader.Load(templatePath)

	return &RawStrategy{
		BaseStrategy: NewBaseStrategyFromConfig(cfg, bindIP),
		templatePath: templatePath,
		template:     tmpl,
		spoofIPs:     cfg.SpoofIPs,
		randomSpoof:  cfg.RandomSpoof,
		socketFD:     syscall.InvalidHandle, // Init logic needed
	}
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

	// Build Packet
	var packet []byte
	if s.template != nil {
		packet = s.template.BuildPacket(srcIP, dstIP, 0, dstPort)
	} else {
		return fmt.Errorf("no template")
	}

	// Send
	// If spoofing is active (randomSpoof or spoofIPs provided), we usage raw socket.
	// Else we try standard Dial (if not raw socket available).

	// Actually, l4_attack.py ALWAYS tries raw socket first.
	// I will implement raw socket logic here.

	return s.sendRaw(packet, dstIP, dstPort)
}

func (s *RawStrategy) sendRaw(packet []byte, dstIP net.IP, dstPort int) error {
	// Strip L2 header if present - raw IP socket expects IP header first
	sendPacket := packet
	if s.template != nil && s.template.HasL2Header {
		sendPacket = s.template.GetPacketWithoutL2(packet)
	}

	// Windows Raw Socket
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, IPPROTO_RAW)
	if err != nil {
		// Fallback to UDP dial if raw socket fails (likely permission issue)
		return s.sendUDP(packet, dstIP, dstPort)
	}
	defer syscall.Close(fd)

	// Set IP_HDRINCL - we provide our own IP header
	// On Windows IP_HDRINCL = 2, on Linux = 3
	err = syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, IP_HDRINCL, 1)
	if err != nil {
		return s.sendUDP(packet, dstIP, dstPort)
	}

	// Destination Address
	addr := syscall.SockaddrInet4{
		Port: dstPort,
	}
	copy(addr.Addr[:], dstIP.To4())

	err = syscall.Sendto(fd, sendPacket, 0, &addr)
	if err != nil {
		return err
	}

	s.IncrementConnections()
	return nil
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
