// SPDX-License-Identifier: GPL-3.0-or-later

package uis_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"syscall"
	"testing"

	"github.com/bassosimone/uis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenConfigListenRejectsUnknownNetwork(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	_, err := listenCfg.Listen(context.Background(), "tcp4", "10.0.0.1:80")
	require.Error(t, err)
	assert.True(t, errors.Is(err, syscall.EPROTOTYPE))
}

func TestListenConfigListenRejectsDomain(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	_, err := listenCfg.Listen(context.Background(), "tcp", "example.com:80")
	require.Error(t, err)
}

func TestListenConfigListenPacketRejectsUnknownNetwork(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	_, err := listenCfg.ListenPacket(context.Background(), "udp4", "10.0.0.1:53")
	require.Error(t, err)
	assert.True(t, errors.Is(err, syscall.EPROTOTYPE))
}

func TestListenConfigListenPacketRejectsDomain(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	_, err := listenCfg.ListenPacket(context.Background(), "udp", "example.com:53")
	require.Error(t, err)
}

func TestListenConfigListenAddressInUse(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	listener, err := listenCfg.Listen(context.Background(), "tcp", "10.0.0.1:80")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	_, err = listenCfg.Listen(context.Background(), "tcp", "10.0.0.1:80")
	require.Error(t, err)
}

func TestListenConfigListenPacketAddressInUse(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	pconn, err := listenCfg.ListenPacket(context.Background(), "udp", "10.0.0.1:53")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pconn.Close() })

	_, err = listenCfg.ListenPacket(context.Background(), "udp", "10.0.0.1:53")
	require.Error(t, err)
}

func TestListenerWrapperAcceptAfterClose(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	listener, err := listenCfg.Listen(context.Background(), "tcp", "10.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())

	_, err = listener.Accept()
	require.Error(t, err)
}

func TestListenerWrapperAddr(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(stack.Close)

	listenCfg := uis.NewListenConfig(stack)
	listener, err := listenCfg.Listen(context.Background(), "tcp", "10.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	assert.True(t, addr.IP.Equal(net.ParseIP("10.0.0.1")))
	assert.NotZero(t, addr.Port)
}
