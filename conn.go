//
// SPDX-License-Identifier: GPL-3.0-or-later
//
// Adapted from: https://github.com/ooni/netem/blob/061c5671b52a2c064cac1de5d464bb056f7ccaa8/unetstack.go
//

package uis

import (
	"net"
	"time"
)

// connWrapper wraps a [net.Conn] to remap gVisor errors
// so that we can emulate stdlib errors.
type connWrapper struct {
	conn net.Conn
}

var _ net.Conn = &connWrapper{}

// Close implements [net.Conn].
func (cw *connWrapper) Close() error {
	return cw.conn.Close()
}

// LocalAddr implements [net.Conn].
func (cw *connWrapper) LocalAddr() net.Addr {
	return cw.conn.LocalAddr()
}

// Read implements [net.Conn].
func (cw *connWrapper) Read(buff []byte) (int, error) {
	count, err := cw.conn.Read(buff)
	return count, errorsRemap(err)
}

// RemoteAddr implements [net.Conn].
func (cw *connWrapper) RemoteAddr() net.Addr {
	return cw.conn.RemoteAddr()
}

// SetDeadline implements [net.Conn].
func (cw *connWrapper) SetDeadline(t time.Time) error {
	return cw.conn.SetDeadline(t)
}

// SetReadDeadline implements [net.Conn].
func (cw *connWrapper) SetReadDeadline(t time.Time) error {
	return cw.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements [net.Conn].
func (cw *connWrapper) SetWriteDeadline(t time.Time) error {
	return cw.conn.SetWriteDeadline(t)
}

// Write implements [net.Conn].
func (cw *connWrapper) Write(data []byte) (int, error) {
	count, err := cw.conn.Write(data)
	return count, errorsRemap(err)
}
