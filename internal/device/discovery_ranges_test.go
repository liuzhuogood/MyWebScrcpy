package device

import (
	"net"
	"reflect"
	"testing"
)

func TestScanMultipleRanges(t *testing.T) {
	addresses, err := scanAddresses(ScanRequest{Ranges: []string{"10.0.0.1-2", "10.0.0.2-3"}, Ports: []int{5555}})
	if err != nil || !reflect.DeepEqual(addresses, []string{"10.0.0.1:5555", "10.0.0.2:5555", "10.0.0.3:5555"}) {
		t.Fatalf("%v %v", addresses, err)
	}
	for _, req := range []ScanRequest{
		{Range: "10.0.0.1", Ranges: []string{"10.0.0.2"}, Ports: []int{5555}},
		{Ranges: []string{"10.0.0.1", "8.8.8.8"}, Ports: []int{5555}},
		{Ranges: make([]string, 9), Ports: []int{5555}},
	} {
		if _, err := scanAddresses(req); err == nil {
			t.Fatalf("accepted invalid request: %+v", req)
		}
	}
}
func TestLocalNetworkRangeUsesActualMask(t *testing.T) {
	for _, tt := range []struct{ cidr, start, end string }{
		{"10.23.45.67/24", "10.23.45.1", "10.23.45.254"},
		{"192.168.7.135/25", "192.168.7.129", "192.168.7.254"},
		{"10.23.45.67/16", "10.23.45.1", "10.23.45.254"},
		{"172.16.1.2/30", "172.16.1.1", "172.16.1.2"},
	} {
		ip, network, _ := net.ParseCIDR(tt.cidr)
		got, ok := networkScanRange("en0", &net.IPNet{IP: ip, Mask: network.Mask})
		if !ok || got.Start != tt.start || got.End != tt.end || got.Address != ip.String() {
			t.Fatalf("%s => %+v %v", tt.cidr, got, ok)
		}
	}
	for _, cidr := range []string{"8.8.8.8/24", "127.0.0.1/8", "::1/128"} {
		ip, n, _ := net.ParseCIDR(cidr)
		if _, ok := networkScanRange("x", &net.IPNet{IP: ip, Mask: n.Mask}); ok {
			t.Fatal(cidr)
		}
	}
}
