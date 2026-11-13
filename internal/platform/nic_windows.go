package platform

import (
	"fmt"
	"net"
)

func interfaces() ([]NIC, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	return nicsFrom(ifs, func(i net.Interface) ([]net.Addr, error) { return i.Addrs() }), nil
}
