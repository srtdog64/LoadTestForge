package raw

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Template represents a parsed packet template
type Template struct {
	Name        string
	Raw         []byte
	Variables   []Variable
	HasL2Header bool // Whether template includes Ethernet header
}

// Variable represents a dynamic field in the packet
type Variable struct {
	Name   string
	Offset int
	Size   int
}

// Loader handles template loading
type Loader struct {
	baseDir string
}

func NewLoader(baseDir string) *Loader {
	return &Loader{baseDir: baseDir}
}

// Load loads a template from a file
func (l *Loader) Load(path string) (*Template, error) {
	// If path is relative, check in baseDir
	if !filepath.IsAbs(path) {
		candidates := []string{
			path,
			filepath.Join(l.baseDir, path),
			filepath.Join(l.baseDir, "templates", path),
			filepath.Join(l.baseDir, "templates", "raw", path),
			filepath.Join(l.baseDir, "templates", "l4", path),
		}

		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	return l.Parse(string(content), filepath.Base(path))
}

// Parse parses the string content of a template
func (l *Loader) Parse(content, name string) (*Template, error) {
	tmpl := &Template{
		Name: name,
		Raw:  make([]byte, 0, 1500),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	offset := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Remove comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		// Detect L2 header
		if strings.Contains(line, "@DMAC") || strings.Contains(line, "@SMAC") {
			tmpl.HasL2Header = true
		}

		tokens := strings.Fields(line)
		i := 0
		for i < len(tokens) {
			token := tokens[i]

			if strings.HasPrefix(token, "@") {
				// Variable field (@VAR:SIZE format)
				name := token
				size := 4 // Default size
				if strings.Contains(token, ":") {
					parts := strings.Split(token, ":")
					name = parts[0]
					if s, err := strconv.Atoi(parts[1]); err == nil {
						size = s
					}
				} else {
					size = getDefaultSize(name)
				}

				tmpl.Variables = append(tmpl.Variables, Variable{
					Name:   name,
					Offset: offset,
					Size:   size,
				})

				// Append placeholders
				tmpl.Raw = append(tmpl.Raw, make([]byte, size)...)
				offset += size
				i++

			} else if token == "GK" && i+1 < len(tokens) && tokens[i+1] == "GG" {
				// GK GG = Random 2 bytes (source port)
				tmpl.Variables = append(tmpl.Variables, Variable{
					Name:   "GK_GG",
					Offset: offset,
					Size:   2,
				})
				tmpl.Raw = append(tmpl.Raw, 0, 0)
				offset += 2
				i += 2

			} else if token == "KK" && i+1 < len(tokens) && tokens[i+1] == "GG" {
				// KK GG = Random 2 bytes (transaction ID, etc)
				tmpl.Variables = append(tmpl.Variables, Variable{
					Name:   "KK_GG",
					Offset: offset,
					Size:   2,
				})
				tmpl.Raw = append(tmpl.Raw, 0, 0)
				offset += 2
				i += 2

			} else if token == "KK" && i+3 < len(tokens) &&
				tokens[i+1] == "KK" && tokens[i+2] == "KK" && tokens[i+3] == "KK" {
				// KK KK KK KK = Random 4 bytes (TCP sequence, etc)
				tmpl.Variables = append(tmpl.Variables, Variable{
					Name:   "KK_KK_KK_KK",
					Offset: offset,
					Size:   4,
				})
				tmpl.Raw = append(tmpl.Raw, 0, 0, 0, 0)
				offset += 4
				i += 4

			} else {
				// Hex value
				b, err := hex.DecodeString(token)
				if err == nil {
					tmpl.Raw = append(tmpl.Raw, b...)
					offset += len(b)
				}
				// Skip invalid tokens silently
				i++
			}
		}
	}

	return tmpl, nil
}

func getDefaultSize(name string) int {
	switch name {
	case "@DMAC", "@SMAC":
		return 6
	case "@SIP", "@DIP":
		return 4
	case "@SIP6", "@DIP6":
		return 16
	case "@SPORT", "@DPORT", "@LEN", "@UDPLEN", "@ID":
		return 2
	case "@PLEN":
		return 2
	case "@IPCHK", "@UDPCHK", "@TCPCHK", "@ICMPCHK", "@IGMPCHK":
		return 2
	case "@ROOTID", "@BRIDGEID":
		return 8
	case "@PORTID":
		return 2
	case "@DATA":
		return 0 // Must be specified with @DATA:SIZE
	default:
		return 4
	}
}

// PacketParams contains parameters for building a packet
type PacketParams struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort int
	DstPort int
	SrcMAC  net.HardwareAddr
	DstMAC  net.HardwareAddr
	SrcIPv6 net.IP
	DstIPv6 net.IP
}

// BuildPacket constructs a packet from the template with given parameters
func (t *Template) BuildPacket(srcIP, dstIP net.IP, srcPort, dstPort int) []byte {
	params := PacketParams{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: srcPort,
		DstPort: dstPort,
	}
	return t.BuildPacketWithParams(params)
}

// UpdatePacket updates an existing packet buffer with new parameters.
func (t *Template) UpdatePacket(packet []byte, params PacketParams, init bool) {
	if init {
		copy(packet, t.Raw)
	}

	// Generate random source port if not specified
	srcPort := params.SrcPort
	if srcPort == 0 {
		srcPort = rand.Intn(64511) + 1024 // 1024-65535
	}

	// First pass: substitute all variables
	for _, v := range t.Variables {
		switch v.Name {
		case "@DMAC":
			if params.DstMAC != nil {
				copy(packet[v.Offset:], params.DstMAC)
			} else {
				// Broadcast MAC
				copy(packet[v.Offset:], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
			}

		case "@SMAC":
			if params.SrcMAC != nil {
				copy(packet[v.Offset:], params.SrcMAC)
			} else {
				// Random MAC
				mac := make([]byte, 6)
				rand.Read(mac)
				mac[0] = (mac[0] | 0x02) & 0xfe // Locally administered, unicast
				copy(packet[v.Offset:], mac)
			}

		case "@SIP":
			if params.SrcIP != nil {
				ip4 := params.SrcIP.To4()
				if ip4 != nil {
					copy(packet[v.Offset:], ip4)
				}
			}

		case "@DIP":
			if params.DstIP != nil {
				ip4 := params.DstIP.To4()
				if ip4 != nil {
					copy(packet[v.Offset:], ip4)
				}
			}

		case "@SIP6":
			if params.SrcIPv6 != nil {
				copy(packet[v.Offset:], params.SrcIPv6.To16())
			}

		case "@DIP6":
			if params.DstIPv6 != nil {
				copy(packet[v.Offset:], params.DstIPv6.To16())
			}

		case "@SPORT":
			binary.BigEndian.PutUint16(packet[v.Offset:], uint16(srcPort))

		case "@DPORT":
			binary.BigEndian.PutUint16(packet[v.Offset:], uint16(params.DstPort))

		case "@ID":
			binary.BigEndian.PutUint16(packet[v.Offset:], uint16(rand.Intn(65536)))

		case "GK_GG":
			// Random source port
			binary.BigEndian.PutUint16(packet[v.Offset:], uint16(srcPort))

		case "KK_GG":
			// Random 2 bytes (transaction ID, sequence, etc)
			binary.BigEndian.PutUint16(packet[v.Offset:], uint16(rand.Intn(65536)))

		case "KK_KK_KK_KK":
			// Random 4 bytes (TCP sequence number, etc)
			binary.BigEndian.PutUint32(packet[v.Offset:], rand.Uint32())

		case "@DATA":
			// Fill with random data
			randData := make([]byte, v.Size)
			rand.Read(randData)
			copy(packet[v.Offset:], randData)

		case "@ROOTID":
			// STP Root Bridge ID: Priority (2 bytes) + MAC (6 bytes)
			// Priority 0 = highest priority
			packet[v.Offset] = 0x00
			packet[v.Offset+1] = 0x00
			rand.Read(packet[v.Offset+2 : v.Offset+8])

		case "@BRIDGEID":
			// STP Bridge ID: Priority 32768 (0x8000) + MAC
			packet[v.Offset] = 0x80
			packet[v.Offset+1] = 0x00
			rand.Read(packet[v.Offset+2 : v.Offset+8])

		case "@PORTID":
			binary.BigEndian.PutUint16(packet[v.Offset:], uint16(rand.Intn(255)+1))
		}
	}

	// Second pass: calculate lengths and checksums
	t.calculateLengths(packet)
	t.calculateChecksums(packet)
}

// BuildPacketWithParams constructs a packet with full parameters
func (t *Template) BuildPacketWithParams(params PacketParams) []byte {
	packet := make([]byte, len(t.Raw))
	t.UpdatePacket(packet, params, true)
	return packet
}

// calculateLengths calculates and fills length fields
func (t *Template) calculateLengths(packet []byte) {
	l2Offset := 0
	if t.HasL2Header {
		l2Offset = 14 // Ethernet header size
	}

	isIPv6 := t.isIPv6(packet)
	ipHeaderLen := 20 // Standard IPv4 header
	if isIPv6 {
		ipHeaderLen = 40
	}

	for _, v := range t.Variables {
		switch v.Name {
		case "@LEN":
			if !isIPv6 {
				// IPv4 Total Length = packet length - L2 header
				totalLen := len(packet) - l2Offset
				binary.BigEndian.PutUint16(packet[v.Offset:], uint16(totalLen))
			}

		case "@PLEN":
			// IPv6 Payload Length = packet length - L2 header - IPv6 header
			payloadLen := len(packet) - l2Offset - ipHeaderLen
			if payloadLen >= 0 {
				binary.BigEndian.PutUint16(packet[v.Offset:], uint16(payloadLen))
			}

		case "@UDPLEN":
			// UDP Length = packet length - L2 header - IP header
			udpLen := len(packet) - l2Offset - ipHeaderLen
			if udpLen >= 0 {
				binary.BigEndian.PutUint16(packet[v.Offset:], uint16(udpLen))
			}
		}
	}
}

// calculateChecksums calculates and fills checksum fields
func (t *Template) calculateChecksums(packet []byte) {
	l2Offset := 0
	if t.HasL2Header {
		l2Offset = 14
	}

	isIPv6 := t.isIPv6(packet)
	ipHeaderLen := 20
	if isIPv6 {
		ipHeaderLen = 40
	}

	for _, v := range t.Variables {
		switch v.Name {
		case "@IPCHK":
			// IP header checksum
			if !isIPv6 && len(packet) >= l2Offset+ipHeaderLen {
				// Zero out checksum field first
				binary.BigEndian.PutUint16(packet[v.Offset:], 0)
				// Calculate checksum over IP header
				ipHeader := packet[l2Offset : l2Offset+ipHeaderLen]
				checksum := calculateChecksum(ipHeader)
				binary.BigEndian.PutUint16(packet[v.Offset:], checksum)
			}

		case "@UDPCHK":
			// UDP checksum (with pseudo-header)
			if len(packet) >= l2Offset+ipHeaderLen+8 {
				// Zero out checksum field first
				binary.BigEndian.PutUint16(packet[v.Offset:], 0)
				checksum := t.calculateUDPChecksum(packet, l2Offset, isIPv6, ipHeaderLen)
				if checksum == 0 {
					checksum = 0xFFFF // UDP checksum of 0 is transmitted as 0xFFFF
				}
				binary.BigEndian.PutUint16(packet[v.Offset:], checksum)
			}

		case "@TCPCHK":
			// TCP checksum (with pseudo-header)
			if len(packet) >= l2Offset+ipHeaderLen+20 {
				binary.BigEndian.PutUint16(packet[v.Offset:], 0)
				checksum := t.calculateTCPChecksum(packet, l2Offset, isIPv6, ipHeaderLen)
				binary.BigEndian.PutUint16(packet[v.Offset:], checksum)
			}

		case "@ICMPCHK":
			// ICMP checksum
			if len(packet) >= l2Offset+ipHeaderLen+8 {
				binary.BigEndian.PutUint16(packet[v.Offset:], 0)
				icmpData := packet[l2Offset+ipHeaderLen:]
				checksum := calculateChecksum(icmpData)
				binary.BigEndian.PutUint16(packet[v.Offset:], checksum)
			}

		case "@IGMPCHK":
			// IGMP checksum
			if len(packet) >= l2Offset+ipHeaderLen+8 {
				binary.BigEndian.PutUint16(packet[v.Offset:], 0)
				igmpData := packet[l2Offset+ipHeaderLen:]
				checksum := calculateChecksum(igmpData)
				binary.BigEndian.PutUint16(packet[v.Offset:], checksum)
			}
		}
	}
}

// calculateChecksum calculates the Internet checksum
func calculateChecksum(data []byte) uint16 {
	// Pad to even length
	if len(data)%2 != 0 {
		data = append(data, 0)
	}

	var sum uint32
	for i := 0; i < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}

	// Fold 32-bit sum to 16 bits
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return ^uint16(sum)
}

// calculateUDPChecksum calculates UDP checksum including pseudo-header
func (t *Template) calculateUDPChecksum(packet []byte, l2Offset int, isIPv6 bool, ipHeaderLen int) uint16 {
	// UDP data (header + payload)
	udpData := packet[l2Offset+ipHeaderLen:]
	udpLen := len(udpData)
	proto := byte(17) // UDP

	if isIPv6 {
		srcIP := packet[l2Offset+8 : l2Offset+24]
		dstIP := packet[l2Offset+24 : l2Offset+40]
		if len(srcIP) < 16 || len(dstIP) < 16 {
			return 0
		}

		pseudoHeader := make([]byte, 40)
		copy(pseudoHeader[0:16], srcIP)
		copy(pseudoHeader[16:32], dstIP)
		pseudoHeader[32] = byte(udpLen >> 24)
		pseudoHeader[33] = byte(udpLen >> 16)
		pseudoHeader[34] = byte(udpLen >> 8)
		pseudoHeader[35] = byte(udpLen)
		pseudoHeader[39] = proto

		checksumData := append(pseudoHeader, udpData...)
		return calculateChecksum(checksumData)
	}

	// IPv4
	srcIP := packet[l2Offset+12 : l2Offset+16]
	dstIP := packet[l2Offset+16 : l2Offset+20]

	pseudoHeader := make([]byte, 12)
	copy(pseudoHeader[0:4], srcIP)
	copy(pseudoHeader[4:8], dstIP)
	pseudoHeader[8] = 0
	pseudoHeader[9] = proto
	pseudoHeader[10] = byte(udpLen >> 8)
	pseudoHeader[11] = byte(udpLen)

	checksumData := append(pseudoHeader, udpData...)
	return calculateChecksum(checksumData)
}

// calculateTCPChecksum calculates TCP checksum including pseudo-header
func (t *Template) calculateTCPChecksum(packet []byte, l2Offset int, isIPv6 bool, ipHeaderLen int) uint16 {
	// TCP data (header + payload)
	tcpData := packet[l2Offset+ipHeaderLen:]
	tcpLen := len(tcpData)
	proto := byte(6) // TCP

	if isIPv6 {
		srcIP := packet[l2Offset+8 : l2Offset+24]
		dstIP := packet[l2Offset+24 : l2Offset+40]
		if len(srcIP) < 16 || len(dstIP) < 16 {
			return 0
		}

		pseudoHeader := make([]byte, 40)
		copy(pseudoHeader[0:16], srcIP)
		copy(pseudoHeader[16:32], dstIP)
		pseudoHeader[32] = byte(tcpLen >> 24)
		pseudoHeader[33] = byte(tcpLen >> 16)
		pseudoHeader[34] = byte(tcpLen >> 8)
		pseudoHeader[35] = byte(tcpLen)
		pseudoHeader[39] = proto

		checksumData := append(pseudoHeader, tcpData...)
		return calculateChecksum(checksumData)
	}

	// IPv4
	srcIP := packet[l2Offset+12 : l2Offset+16]
	dstIP := packet[l2Offset+16 : l2Offset+20]

	pseudoHeader := make([]byte, 12)
	copy(pseudoHeader[0:4], srcIP)
	copy(pseudoHeader[4:8], dstIP)
	pseudoHeader[8] = 0
	pseudoHeader[9] = proto
	pseudoHeader[10] = byte(tcpLen >> 8)
	pseudoHeader[11] = byte(tcpLen)

	checksumData := append(pseudoHeader, tcpData...)
	return calculateChecksum(checksumData)
}

// isIPv6 tries to determine whether the packet uses IPv6
func (t *Template) isIPv6(packet []byte) bool {
	l2Offset := 0
	if t.HasL2Header {
		l2Offset = 14
	}

	if len(packet) > l2Offset {
		version := packet[l2Offset] >> 4
		if version == 6 {
			return true
		}
		if version == 4 {
			return false
		}
	}

	for _, v := range t.Variables {
		if v.Name == "@SIP6" || v.Name == "@DIP6" {
			return true
		}
	}

	return false
}

// GetPacketWithoutL2 returns the packet without Ethernet header
func (t *Template) GetPacketWithoutL2(packet []byte) []byte {
	if t.HasL2Header && len(packet) > 14 {
		return packet[14:]
	}
	return packet
}

// GetInfo returns template information
func (t *Template) GetInfo() map[string]interface{} {
	vars := make([]string, len(t.Variables))
	for i, v := range t.Variables {
		vars[i] = v.Name
	}
	return map[string]interface{}{
		"name":      t.Name,
		"size":      len(t.Raw),
		"has_l2":    t.HasL2Header,
		"variables": vars,
	}
}
