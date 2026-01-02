//
// SPDX-License-Identifier: GPL-3.0-or-later
//
// Adapted from: https://github.com/ooni/netem/blob/061c5671b52a2c064cac1de5d464bb056f7ccaa8/unetstack.go
//

package uis

import (
	"net"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

// packetConnWrapper wraps a [net.PacketConn] and remaps gVisor errors
// to emulate stdlib errors.
type packetConnWrapper struct {
	pconn *gonet.UDPConn
}

var _ net.PacketConn = &packetConnWrapper{}

// Close implements [net.PacketConn].
func (pcw *packetConnWrapper) Close() error {
	return pcw.pconn.Close()
}

// LocalAddr implements [net.PacketConn].
func (pcw *packetConnWrapper) LocalAddr() net.Addr {
	return pcw.pconn.LocalAddr()
}

// ReadFrom implements [net.PacketConn].
func (pcw *packetConnWrapper) ReadFrom(buff []byte) (int, net.Addr, error) {
	count, addr, err := pcw.pconn.ReadFrom(buff)
	return count, addr, errorsRemap(err)
}

// SetDeadline implements [net.PacketConn].
func (pcw *packetConnWrapper) SetDeadline(t time.Time) error {
	return pcw.pconn.SetDeadline(t)
}

// SetReadDeadline implements [net.PacketConn].
func (pcw *packetConnWrapper) SetReadDeadline(t time.Time) error {
	return pcw.pconn.SetReadDeadline(t)
}

// SetWriteDeadline implements [net.PacketConn].
func (pcw *packetConnWrapper) SetWriteDeadline(t time.Time) error {
	return pcw.pconn.SetWriteDeadline(t)
}

// WriteTo implements [net.PacketConn].
func (pcw *packetConnWrapper) WriteTo(pkt []byte, addr net.Addr) (int, error) {
	count, err := pcw.pconn.WriteTo(pkt, addr)
	return count, errorsRemap(err)
}
