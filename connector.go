//
// SPDX-License-Identifier: GPL-3.0-or-later
//
// Adapted from: https://github.com/ooni/netem/blob/061c5671b52a2c064cac1de5d464bb056f7ccaa8/unetstack.go
//

package uis

import (
	"context"
	"net"
	"net/netip"
	"syscall"
)

// Connector allows to dial [net.Conn] connections pretty much
// like [*net.Dialer] except that here we use a [*Stack]
// implementation as the networking backend.
//
// The zero value is invalid. Construct using [NewConnector].
//
// Only IP literal endpoints are supported. Dialing a hostname will fail.
type Connector struct {
	// stack is the uis stack to use.
	stack *Stack
}

// NewConnector creates a new [*Connector] instance.
func NewConnector(stack *Stack) *Connector {
	return &Connector{stack: stack}
}

// DialContext creates a new [net.Conn] connection.
func (c *Connector) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	// 1. parse the address into a [netip.AddrPort]
	addrport, err := netip.ParseAddrPort(address)
	if err != nil {
		return nil, err
	}

	// 2. dial using either TCP or UDP
	var conn net.Conn
	switch network {
	case "tcp":
		conn, err = c.stack.DialTCP(ctx, addrport)

	case "udp":
		conn, err = c.stack.DialUDP(addrport)

	default:
		return nil, syscall.EPROTOTYPE
	}

	// 3. remap the error on failure
	if err != nil {
		return nil, errorsRemap(err)
	}

	// 4. wrap conn to correctly remap errors
	return &connWrapper{conn}, nil
}
