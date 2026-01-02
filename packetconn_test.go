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

func TestPacketConnWrapperUDPIPv6DeadlinesAndAddrs(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUMinimumIPv6, nil)
	stack, err := uis.NewStack(vnic, netip.MustParseAddr("2001:db8::1"))
	require.NoError(t, err)
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	pconn, err := listenCfg.ListenPacket(context.Background(), "udp", "[2001:db8::1]:53")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pconn.Close() })

	laddr, ok := pconn.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	assert.True(t, laddr.IP.Equal(net.ParseIP("2001:db8::1")))
	assert.Equal(t, 53, laddr.Port)

	buffer := make([]byte, 1)

	require.NoError(t, pconn.SetDeadline(time.Now().Add(10*time.Microsecond)))
	_, _, err = pconn.ReadFrom(buffer)
	require.Error(t, err)
	neterr, ok := err.(net.Error)
	require.True(t, ok)
	assert.True(t, neterr.Timeout())

	require.NoError(t, pconn.SetReadDeadline(time.Now().Add(10*time.Microsecond)))
	_, _, err = pconn.ReadFrom(buffer)
	require.Error(t, err)
	neterr, ok = err.(net.Error)
	require.True(t, ok)
	assert.True(t, neterr.Timeout())

	require.NoError(t, pconn.SetWriteDeadline(time.Now().Add(10*time.Microsecond)))
}
