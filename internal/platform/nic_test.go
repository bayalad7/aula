package platform

import (
	"errors"
	"net"
	"runtime"
	"testing"
)

func TestNICsFromKeepsUpCardsWithMACAndIPv4(t *testing.T) {
	mac := net.HardwareAddr{0x3c, 0x52, 0x82, 0x1a, 0x9f, 0x07}
	ifs := []net.Interface{
		{Index: 1, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast, HardwareAddr: mac},
		{Index: 2, Name: "Loopback", Flags: net.FlagUp | net.FlagLoopback},
		{Index: 3, Name: "Desconectada", Flags: net.FlagBroadcast, HardwareAddr: mac},
		{Index: 4, Name: "Túnel", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{1, 2, 3, 4, 5, 6, 7, 8}},
		{Index: 5, Name: "Solo IPv6", Flags: net.FlagUp, HardwareAddr: mac},
		{Index: 6, Name: "Sin red", Flags: net.FlagUp, HardwareAddr: mac},
	}
	addrs := map[int][]net.Addr{
		1: {
			&net.IPNet{IP: net.ParseIP("192.168.10.7"), Mask: net.CIDRMask(24, 32)},
			&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
			&net.IPNet{IP: net.ParseIP("169.254.3.4").To4(), Mask: net.CIDRMask(16, 32)},
		},
		3: {&net.IPNet{IP: net.ParseIP("10.0.0.2").To4(), Mask: net.CIDRMask(8, 32)}},
		5: {&net.IPNet{IP: net.ParseIP("fe80::2"), Mask: net.CIDRMask(64, 128)}},
	}
	got := nicsFrom(ifs, func(i net.Interface) ([]net.Addr, error) {
		if i.Index == 6 {
			return nil, errors.New("sin acceso")
		}
		return addrs[i.Index], nil
	})
	if len(got) != 1 || got[0].Name != "Ethernet" {
		t.Fatalf("nicsFrom = %+v, want only Ethernet", got)
	}
	if len(got[0].Addrs) != 1 || got[0].Addrs[0].String() != "192.168.10.7/24" {
		t.Fatalf("addrs = %v, want 192.168.10.7/24", got[0].Addrs)
	}
}

func TestInterfaces(t *testing.T) {
	nics, err := Interfaces()
	if runtime.GOOS != "windows" {
		if !errors.Is(err, ErrNotSupported) {
			t.Fatalf("err = %v, want ErrNotSupported", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nics {
		if len(n.MAC) != 6 || len(n.Addrs) == 0 {
			t.Fatalf("card %q without MAC or IPv4", n.Name)
		}
	}
}
