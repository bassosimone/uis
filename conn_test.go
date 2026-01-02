// SPDX-License-Identifier: GPL-3.0-or-later

package uis_test

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/bassosimone/uis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnWrapperUDPIPv6DeadlinesAndAddrs(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUMinimumIPv6, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("2001:db8::1"))
	t.Cleanup(stack.Close)

	connector := uis.NewConnector(stack)
	conn, err := connector.DialContext(context.Background(), "udp", "[2001:db8::2]:53")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	laddr, ok := conn.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	assert.True(t, laddr.IP.Equal(net.ParseIP("2001:db8::1")))
	assert.NotZero(t, laddr.Port)

	raddr, ok := conn.RemoteAddr().(*net.UDPAddr)
	require.True(t, ok)
	assert.True(t, raddr.IP.Equal(net.ParseIP("2001:db8::2")))
	assert.Equal(t, 53, raddr.Port)

	buffer := make([]byte, 1)

	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Microsecond)))
	_, err = conn.Read(buffer)
	require.Error(t, err)
	neterr, ok := err.(net.Error)
	require.True(t, ok)
	assert.True(t, neterr.Timeout())

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Microsecond)))
	_, err = conn.Read(buffer)
	require.Error(t, err)
	neterr, ok = err.(net.Error)
	require.True(t, ok)
	assert.True(t, neterr.Timeout())

	require.NoError(t, conn.SetWriteDeadline(time.Now().Add(10*time.Microsecond)))
}
