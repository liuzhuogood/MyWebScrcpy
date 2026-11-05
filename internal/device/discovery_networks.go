package device

import (
	"encoding/binary"
	"net"
)

type ScanNetwork struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix_length"`
	Start        string `json:"start"`
	End          string `json:"end"`
}

// Limit a suggested range to 256 addresses, intersecting the actual interface
// subnet with the host's /24 block. Narrower masks retain their real boundaries.
func networkScanRange(name string, network *net.IPNet) (ScanNetwork, bool) {
	ip := network.IP.To4()
	ones, bits := network.Mask.Size()
	if ip == nil || bits != 32 || !ip.IsPrivate() {
		return ScanNetwork{}, false
	}
	host := binary.BigEndian.Uint32(ip)
	mask := binary.BigEndian.Uint32(network.Mask)
	start, end := host&mask, (host&mask)|^mask
	if ones <= 30 {
		start++
		end--
	}
	block := host & 0xffffff00
	if start < block+1 {
		start = block + 1
	}
	if end > block+254 {
		end = block + 254
	}
	// /31 and /32 can legitimately use .0 or .255.
	if ones >= 31 {
		start = host & mask
		end = start | ^mask
	}
	addr := func(value uint32) string {
		b := make(net.IP, 4)
		binary.BigEndian.PutUint32(b, value)
		return b.String()
	}
	return ScanNetwork{Name: name, Address: ip.String(), PrefixLength: ones, Start: addr(start), End: addr(end)}, true
}
