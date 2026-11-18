//go:build linux

package platform

import (
	"fmt"
	"net"
)

// interfaces on Linux: the standard library reads them from the kernel
// (netlink), with the same filter as Windows (nicsFrom).
func interfaces() ([]NIC, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("network cards: %w", err)
	}
	return nicsFrom(ifs, func(i net.Interface) ([]net.Addr, error) { return i.Addrs() }), nil
}
