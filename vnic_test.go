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

func TestVNICCloseCallsHook(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	called := atomic.Uint32{}
	vnic.SetOnCloseAction(func() {
		called.Add(1)
	})
	vnic.Close()
	assert.Equal(t, uint32(1), called.Load())
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

type countingNetwork struct {
	allow bool
	count atomic.Uint32
}

func (n *countingNetwork) SendFrame(uis.VNICFrame) bool {
	n.count.Add(1)
	return n.allow
}

func makePacketList(payloads ...[]byte) stack.PacketBufferList {
	var list stack.PacketBufferList
	for _, payload := range payloads {
		list.PushBack(stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(payload),
		}))
	}
	return list
}

func TestVNICWritePacketsCases(t *testing.T) {
	t.Run("closed", func(t *testing.T) {
		net := &countingNetwork{allow: true}
		vnic := uis.NewVNIC(uis.MTUEthernet, net)
		vnic.Close()

		pkts := makePacketList([]byte{0x45})
		defer pkts.DecRef()
		num, err := vnic.WritePackets(pkts)
		require.True(t, err != nil)
		require.True(t, err.String() == (&tcpip.ErrNoNet{}).String())
		assert.Equal(t, 0, num)
		assert.Zero(t, net.count.Load())
	})

	t.Run("no_network", func(t *testing.T) {
		vnic := uis.NewVNIC(uis.MTUEthernet, nil)

		pkts := makePacketList([]byte{0x45})
		defer pkts.DecRef()
		num, err := vnic.WritePackets(pkts)
		require.True(t, err != nil)
		require.True(t, err.String() == (&tcpip.ErrNoNet{}).String())
		assert.Equal(t, 0, num)
	})

	t.Run("zero_length_payload", func(t *testing.T) {
		net := &countingNetwork{allow: true}
		vnic := uis.NewVNIC(uis.MTUEthernet, net)

		pkts := makePacketList([]byte{})
		defer pkts.DecRef()
		num, err := vnic.WritePackets(pkts)
		require.True(t, err == nil)
		assert.Equal(t, 0, num)
		assert.Zero(t, net.count.Load())
	})

	t.Run("larger_than_mtu", func(t *testing.T) {
		net := &countingNetwork{allow: true}
		vnic := uis.NewVNIC(1, net)

		pkts := makePacketList([]byte{0x45, 0x00})
		defer pkts.DecRef()
		num, err := vnic.WritePackets(pkts)
		require.True(t, err == nil)
		assert.Equal(t, 0, num)
		assert.Zero(t, net.count.Load())
	})

	t.Run("send_frame_fails", func(t *testing.T) {
		net := &countingNetwork{allow: false}
		vnic := uis.NewVNIC(uis.MTUEthernet, net)

		pkts := makePacketList([]byte{0x45})
		defer pkts.DecRef()
		num, err := vnic.WritePackets(pkts)
		require.True(t, err == nil)
		assert.Equal(t, 0, num)
		assert.Equal(t, uint32(1), net.count.Load())
	})
}
