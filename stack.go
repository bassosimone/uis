//
// SPDX-License-Identifier: MIT
//
// Adapted from: https://github.com/ooni/netem/blob/061c5671b52a2c064cac1de5d464bb056f7ccaa8/gvisor.go
// Adapted from: https://github.com/WireGuard/wireguard-go
//

package uis

import (
	"context"
	"errors"
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

// Stack is a wrapper for [*stack.Stack] allowing basic network
// operations with gVisor's TCP and UDP conns.
//
// Construct using [NewStack].
type Stack struct {
	Stack *stack.Stack
}

// stackNICID is the NIC ID used by [NewStack] for the single NIC configuration.
const stackNICID = 1

// NewStack creates a new [*Stack] using a [stack.LinkEndpoint].
func NewStack(vnic stack.LinkEndpoint, addrs ...netip.Addr) (*Stack, error) {
	// 1. create options for the new stack
	stackOptions := stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
		HandleLocal: true,
	}

	// 2. create the network stack itself
	nsp := stack.New(stackOptions)

	// 3. attach the provided NIC to the gvisor stack
	if err := nsp.CreateNIC(stackNICID, vnic); err != nil {
		return nil, errors.New(err.String())
	}

	// 4. configure all the provided addresses
	for _, addr := range addrs {
		protoAddr := stackAddrToProtocolAddress(addr)
		properties := stack.AddressProperties{}
		if err := nsp.AddProtocolAddress(stackNICID, protoAddr, properties); err != nil {
			return nil, errors.New(err.String())
		}
	}

	// 5. add default routes for both protocol families
	nsp.AddRoute(tcpip.Route{
		Destination: header.IPv4EmptySubnet,
		NIC:         stackNICID,
	})
	nsp.AddRoute(tcpip.Route{
		Destination: header.IPv6EmptySubnet,
		NIC:         stackNICID,
	})

	return &Stack{nsp}, nil
}

func stackAddrToProtocolAddress(addr netip.Addr) tcpip.ProtocolAddress {
	switch {
	case addr.Is4():
		return tcpip.ProtocolAddress{
			Protocol:          ipv4.ProtocolNumber,
			AddressWithPrefix: tcpip.AddrFromSlice(addr.AsSlice()).WithPrefix(),
		}

	default:
		return tcpip.ProtocolAddress{
			Protocol:          ipv6.ProtocolNumber,
			AddressWithPrefix: tcpip.AddrFromSlice(addr.AsSlice()).WithPrefix(),
		}
	}
}

// DialTCP establishes a new [*gonet.TCPConn].
func (sx *Stack) DialTCP(ctx context.Context, addr netip.AddrPort) (*gonet.TCPConn, error) {
	return gonet.DialContextTCP(ctx, sx.Stack, stackAddrPortToFullAddress(addr),
		stackAddrPortToNetworkProtocolNumber(addr))
}

// ListenTCP creates a new [*gonet.TCPListener].
func (sx *Stack) ListenTCP(addr netip.AddrPort) (*gonet.TCPListener, error) {
	return gonet.ListenTCP(sx.Stack, stackAddrPortToFullAddress(addr),
		stackAddrPortToNetworkProtocolNumber(addr))
}

// DialUDP creates a new connected [*gonet.UDPConn].
func (sx *Stack) DialUDP(addr netip.AddrPort) (*gonet.UDPConn, error) {
	raddr := stackAddrPortToFullAddress(addr)
	return gonet.DialUDP(sx.Stack, nil, &raddr, stackAddrPortToNetworkProtocolNumber(addr))
}

// ListenUDP creates a new listening [*gonet.UDPConn].
func (sx *Stack) ListenUDP(addr netip.AddrPort) (*gonet.UDPConn, error) {
	laddr := stackAddrPortToFullAddress(addr)
	return gonet.DialUDP(sx.Stack, &laddr, nil, stackAddrPortToNetworkProtocolNumber(addr))
}

func stackAddrPortToFullAddress(epnt netip.AddrPort) tcpip.FullAddress {
	// In a single-NIC config, unspecified addresses (`0.0.0.0` or `::`) work as expected
	// when bound to the NIC - they'll accept connections on any configured address.
	return tcpip.FullAddress{
		NIC:  stackNICID,
		Addr: tcpip.AddrFromSlice(epnt.Addr().AsSlice()),
		Port: epnt.Port(),
	}
}

func stackAddrPortToNetworkProtocolNumber(epnt netip.AddrPort) tcpip.NetworkProtocolNumber {
	switch {
	case epnt.Addr().Is4():
		return ipv4.ProtocolNumber
	default:
		return ipv6.ProtocolNumber
	}
}

// Close shuts down the stack and waits for the NIC teardown to finish.
func (sx *Stack) Close() {
	sx.Stack.Destroy()
}
