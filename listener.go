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

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

// ListenConfig allows to listen pretty much like [*net.ListenConfig] except that
// here we use a [*Stack] implementation as the networking backend.
//
// The zero value is invalid. Construct using [NewListenConfig].
//
// Only IP literal endpoints are supported. Listening on a hostname will fail.
type ListenConfig struct {
	// stack is the uis stack to use.
	stack *Stack
}

// NewListenConfig creates a new [*ListenConfig] instance.
func NewListenConfig(stack *Stack) *ListenConfig {
	return &ListenConfig{stack: stack}
}

// ListenPacket creates a listening packet conn.
func (lc *ListenConfig) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	// 1. reject networks different from udp
	if network != "udp" {
		return nil, syscall.EPROTOTYPE
	}

	// 2. convert to [netip.AddrPort]
	addrport, err := netip.ParseAddrPort(address)
	if err != nil {
		return nil, err
	}

	// 3. create a UDP connection
	pconn, err := lc.stack.ListenUDP(addrport)
	if err != nil {
		return nil, errorsRemap(err)
	}

	// 4. wrap the connection to remap the errors
	return &packetConnWrapper{pconn}, nil
}

// Listen creates a listening TCP socket.
func (lc *ListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	// 1. reject networks different from tcp
	if network != "tcp" {
		return nil, syscall.EPROTOTYPE
	}

	// 2. convert to [netip.AddrPort]
	addrport, err := netip.ParseAddrPort(address)
	if err != nil {
		return nil, err
	}

	// 3. create a TCP listener
	listener, err := lc.stack.ListenTCP(addrport)
	if err != nil {
		return nil, errorsRemap(err)
	}

	// 4. wrap the connection to remap the errors
	return &listenerWrapper{listener}, nil
}

// listenerWrapper wraps a [net.Listener] and maps gVisor
// errors to the corresponding stdlib errors.
type listenerWrapper struct {
	listener *gonet.TCPListener
}

var _ net.Listener = &listenerWrapper{}

// Accept implements [net.Listener].
func (lw *listenerWrapper) Accept() (net.Conn, error) {
	conn, err := lw.listener.Accept()
	if err != nil {
		return nil, errorsRemap(err)
	}
	return &connWrapper{conn}, nil
}

// Addr implements [net.Listener].
func (lw *listenerWrapper) Addr() net.Addr {
	return lw.listener.Addr()
}

// Close implements [net.Listener].
func (lw *listenerWrapper) Close() error {
	return lw.listener.Close()
}
