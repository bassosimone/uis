// SPDX-License-Identifier: GPL-3.0-or-later

package uis_test

import (
	"sync/atomic"
	"testing"

	"github.com/bassosimone/uis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type vnicDispatcher struct{}

func (vnicDispatcher) DeliverNetworkPacket(tcpip.NetworkProtocolNumber, *stack.PacketBuffer) {
	// nothing
}

func (vnicDispatcher) DeliverLinkPacket(tcpip.NetworkProtocolNumber, *stack.PacketBuffer) {
	// nothing
}

type countingDispatcher struct {
	count atomic.Uint32
}

func (d *countingDispatcher) DeliverNetworkPacket(tcpip.NetworkProtocolNumber, *stack.PacketBuffer) {
	d.count.Add(1)
}

func (d *countingDispatcher) DeliverLinkPacket(tcpip.NetworkProtocolNumber, *stack.PacketBuffer) {
	d.count.Add(1)
}

func TestVNICInterfaceMethods(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)

	assert.Equal(t, header.ARPHardwareNone, vnic.ARPHardwareType())
	assert.Equal(t, uint16(0), vnic.MaxHeaderLength())
	assert.Equal(t, uint32(uis.MTUEthernet), vnic.MTU())
	assert.Equal(t, tcpip.LinkAddress(""), vnic.LinkAddress())

	vnic.SetLinkAddress(tcpip.LinkAddress("test"))
	assert.Equal(t, tcpip.LinkAddress("test"), vnic.LinkAddress())

	vnic.SetMTU(uis.MTUJumbo)
	assert.Equal(t, uint32(uis.MTUJumbo), vnic.MTU())

	pbuf := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData([]byte{0x01}),
	})
	assert.True(t, vnic.ParseHeader(pbuf))
	vnic.AddHeader(pbuf)

	assert.False(t, vnic.IsAttached())
	vnic.Attach(vnicDispatcher{})
	assert.True(t, vnic.IsAttached())
	vnic.Close()
	assert.False(t, vnic.IsAttached())

	require.NotPanics(t, vnic.Wait)
}

func TestVNICInjectFrameDiscardCases(t *testing.T) {
	t.Run("zero_length", func(t *testing.T) {
		vnic := uis.NewVNIC(uis.MTUEthernet, nil)
		assert.False(t, vnic.InjectFrame(uis.VNICFrame{}))
	})

	t.Run("unknown_protocol", func(t *testing.T) {
		vnic := uis.NewVNIC(uis.MTUEthernet, nil)
		disp := &countingDispatcher{}
		vnic.Attach(disp)
		assert.False(t, vnic.InjectFrame(uis.VNICFrame{Packet: []byte{0x70}}))
		assert.Zero(t, disp.count.Load())
	})

	t.Run("closed", func(t *testing.T) {
		vnic := uis.NewVNIC(uis.MTUEthernet, nil)
		disp := &countingDispatcher{}
		vnic.Attach(disp)
		vnic.Close()
		assert.False(t, vnic.InjectFrame(uis.VNICFrame{Packet: []byte{0x40}}))
		assert.Zero(t, disp.count.Load())
	})

	t.Run("no_dispatcher", func(t *testing.T) {
		vnic := uis.NewVNIC(uis.MTUEthernet, nil)
		assert.False(t, vnic.InjectFrame(uis.VNICFrame{Packet: []byte{0x40}}))
	})

	t.Run("larger_than_mtu", func(t *testing.T) {
		vnic := uis.NewVNIC(1, nil)
		disp := &countingDispatcher{}
		vnic.Attach(disp)
		assert.False(t, vnic.InjectFrame(uis.VNICFrame{Packet: []byte{0x40, 0x00}}))
		assert.Zero(t, disp.count.Load())
	})
}
