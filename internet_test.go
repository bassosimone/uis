// SPDX-License-Identifier: GPL-3.0-or-later

package uis_test

import (
	"net/netip"
	"testing"

	"github.com/bassosimone/uis"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func TestInternetAddRouteDuplicateAddress(t *testing.T) {
	ix := uis.NewInternet()
	vnic := ix.NewVNIC(uis.MTUEthernet)
	addr := netip.MustParseAddr("10.0.0.1")

	require.NoError(t, ix.AddRoute(vnic, addr))
	require.Error(t, ix.AddRoute(vnic, addr))
}

func TestInternetNewStackDuplicateAddress(t *testing.T) {
	ix := uis.NewInternet()
	addr := netip.MustParseAddr("10.0.0.1")

	stack, err := ix.NewStack(uis.MTUEthernet, addr)
	require.NoError(t, err)
	t.Cleanup(stack.Close)

	_, err = ix.NewStack(uis.MTUEthernet, addr)
	require.Error(t, err)
}

func TestInternetDeliverFailures(t *testing.T) {
	ix := uis.NewInternet()

	t.Run("empty_packet", func(t *testing.T) {
		require.False(t, ix.Deliver(uis.VNICFrame{}))
	})

	t.Run("unknown_version", func(t *testing.T) {
		require.False(t, ix.Deliver(uis.VNICFrame{Packet: []byte{0x70}}))
	})

	t.Run("ipv4_too_short", func(t *testing.T) {
		require.False(t, ix.Deliver(uis.VNICFrame{Packet: []byte{0x45, 0x00}}))
	})

	t.Run("ipv6_too_short", func(t *testing.T) {
		require.False(t, ix.Deliver(uis.VNICFrame{Packet: []byte{
			0x60, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
		}}))
	})

	t.Run("missing_route_ipv4", func(t *testing.T) {
		require.False(t, ix.Deliver(uis.VNICFrame{Packet: []byte{
			0x45, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x0a, 0x00, 0x00, 0x01,
		}}))
	})
}

func TestInternetSendFrameReturnsFalseWhenFull(t *testing.T) {
	ix := uis.NewInternet(uis.InternetOptionMaxInflight(0))
	vnic := ix.NewVNIC(uis.MTUEthernet)

	pkts := stack.PacketBufferList{}
	pkts.PushBack(stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData([]byte{0x45}),
	}))
	defer pkts.DecRef()

	num, err := vnic.WritePackets(pkts)
	require.True(t, err == nil)
	require.Equal(t, 0, num)
}
