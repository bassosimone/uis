// SPDX-License-Identifier: GPL-3.0-or-later

package uis

import (
	"sync"

	"github.com/bassosimone/runtimex"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// VNICFrame models a virtual link-layer frame without addressing.
type VNICFrame struct {
	// Packet contains a raw IP packet (IPv4 or IPv6).
	Packet []byte
}

// VNICNetwork models the network that a VNIC sends packets to.
//
// The [*Internet] implements this interface.
type VNICNetwork interface {
	SendFrame(frame VNICFrame) bool
}

// VNIC models a virtual NIC. This type is compatible with [stack.Stack]
// because it implements the [stack.LinkEndpoint] interface.
//
// To send packets, [stack.Stack] invokes [*VNIC.WritePackets], which, in
// turn, invokes the attached [VNICNetwork] SendFrame.
//
// To receive packets, the attached [VNICNetwork] invokes [*VNIC.InjectFrame],
// which invokes the [stack.NetworkDispatcher] do dispatch it.
//
// The [stack.Stack] configures the [*VNIC] [stack.NetworkDispatcher] used
// for dispatching via the [*VNIC.Attach] method.
//
// Construct using [NewVNIC].
type VNIC struct {
	// closefunc is the function invoked on close.
	closefunc func()

	// disp is set by Attach and used to deliver inbound packets into netstack.
	disp stack.NetworkDispatcher

	// network is the abstract network we're attached to.
	network VNICNetwork

	// isclosed indicates this NIC should not accept more work.
	isclosed bool

	// laddr is the [tcpip.LinkAddress] to use.
	laddr tcpip.LinkAddress

	// mtu holds the link MTU.
	mtu uint32

	// mu provides mutual exclusion.
	mu sync.RWMutex
}

// NewVNIC creates a new [*VNIC] instance.
//
// The mtu parameter sets the MTU in bytes. Common values:
//
// - [MTUEthernet]
// - [MTUMinimumIPv6]
// - [MTUJumbo]
//
// The network parameter is the [*VNICNetwork] to use.
func NewVNIC(mtu uint32, network VNICNetwork) *VNIC {
	return &VNIC{
		closefunc: nil,
		disp:      nil,
		network:   network,
		isclosed:  false,
		laddr:     "",
		mtu:       mtu,
		mu:        sync.RWMutex{},
	}
}

// Ensure that [*VNIC] implements [stack.LinkEndpoint].
var _ stack.LinkEndpoint = &VNIC{}

// ARPHardwareType implements [stack.LinkEndpoint].
func (n *VNIC) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

// AddHeader implements [stack.LinkEndpoint].
func (n *VNIC) AddHeader(pbuf *stack.PacketBuffer) {
	// nothing to do here because we send raw IP packets
}

// Attach implements [stack.LinkEndpoint].
func (n *VNIC) Attach(disp stack.NetworkDispatcher) {
	n.mu.Lock()
	if !n.isclosed {
		n.disp = disp // setting nil implies detaching the dispatcher
	}
	n.mu.Unlock()
}

// Capabilities implements [stack.LinkEndpoint].
func (n *VNIC) Capabilities() stack.LinkEndpointCapabilities {
	return 0 // no capabilities for now
}

// Close implements [stack.LinkEndpoint].
func (n *VNIC) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.isclosed {
		n.isclosed = true
		n.disp = nil
		if n.closefunc != nil {
			n.closefunc()
		}
	}
}

// IsAttached implements [stack.LinkEndpoint].
func (n *VNIC) IsAttached() bool {
	n.mu.RLock()
	attached := n.disp != nil && !n.isclosed
	n.mu.RUnlock()
	return attached
}

// LinkAddress implements [stack.LinkEndpoint].
func (n *VNIC) LinkAddress() tcpip.LinkAddress {
	n.mu.RLock()
	value := n.laddr
	n.mu.RUnlock()
	return value
}

// MTU implements [stack.LinkEndpoint].
func (n *VNIC) MTU() uint32 {
	n.mu.RLock()
	value := n.mtu
	n.mu.RUnlock()
	return value
}

// MaxHeaderLength implements [stack.LinkEndpoint].
func (n *VNIC) MaxHeaderLength() uint16 {
	return 0 // we send raw IP packets
}

// ParseHeader implements [stack.LinkEndpoint].
func (n *VNIC) ParseHeader(pbuf *stack.PacketBuffer) bool {
	return true // no header to parse
}

// SetLinkAddress implements [stack.LinkEndpoint].
func (n *VNIC) SetLinkAddress(addr tcpip.LinkAddress) {
	n.mu.Lock()
	n.laddr = addr
	n.mu.Unlock()
}

// SetMTU implements [stack.LinkEndpoint].
func (n *VNIC) SetMTU(mtu uint32) {
	n.mu.Lock()
	n.mtu = mtu
	n.mu.Unlock()
}

// SetOnCloseAction implements [stack.LinkEndpoint].
func (n *VNIC) SetOnCloseAction(action func()) {
	n.mu.Lock()
	n.closefunc = action
	n.mu.Unlock()
}

// Wait implements [stack.LinkEndpoint].
func (n *VNIC) Wait() {
	// nothing because we do not create background goroutines
}

// WritePackets implements [stack.LinkEndpoint].
func (n *VNIC) WritePackets(pkts stack.PacketBufferList) (int, tcpip.Error) {
	// 1. access mutex protected fields
	n.mu.RLock()
	network := n.network
	isclosed := n.isclosed
	mtu := n.mtu
	n.mu.RUnlock()

	// 2. bail if the stack has been closed or there's no internet
	if isclosed || network == nil {
		return 0, nil
	}

	// 3. try sending the packets
	var numSent int
	for _, pb := range pkts.AsSlice() {
		// 3.1. serialize the packet buffer to bytes
		payload := vnicPacketBufferToBytes(pb)
		if len(payload) <= 0 {
			continue
		}

		// 3.2. drop the packet if larger than the MTU
		if uint32(len(payload)) > mtu {
			continue
		}

		// 3.3. deliver the frame to the internet
		if !network.SendFrame(VNICFrame{Packet: payload}) {
			continue
		}
		numSent++
	}

	// 4. return number of packets sent
	return numSent, nil
}

// InjectFrame injects an inbound raw IPv4/IPv6 packet into the stack.
func (n *VNIC) InjectFrame(frame VNICFrame) bool {
	// 1. drop the zero-length frames
	pkt := frame.Packet
	if len(pkt) <= 0 {
		return false
	}

	// 2. obtain the corresponding network protocol
	proto, ok := vnicDetectNetworkProtocol(pkt)
	if !ok {
		return false
	}

	// 3. access mutex protected fields
	n.mu.RLock()
	disp := n.disp
	isclosed := n.isclosed
	mtu := n.mtu
	n.mu.RUnlock()

	// 4. do not deliver if we have been closed or have no dispatcher
	if isclosed || disp == nil {
		return false
	}

	// 5. do not deliver if larger than MTU
	if uint32(len(pkt)) > mtu {
		return false
	}

	// 6. deliver A COPY OF the raw network packet
	copied := make([]byte, len(pkt))
	copy(copied, pkt)
	pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(copied),
	})
	disp.DeliverNetworkPacket(proto, pkb)
	return true
}

// vnicDetectNetworkProtocol extracts the protocol number from the raw packet bytes.
//
// This function PANICs if the given pkt is zero length.
func vnicDetectNetworkProtocol(pkt []byte) (tcpip.NetworkProtocolNumber, bool) {
	runtimex.Assert(len(pkt) > 0)
	switch pkt[0] >> 4 {
	case 4:
		return ipv4.ProtocolNumber, true
	case 6:
		return ipv6.ProtocolNumber, true
	default:
		return 0, false
	}
}

// vnicPacketBufferToBytes returns a slice containing A COPY OF the packet bytes.
func vnicPacketBufferToBytes(pb *stack.PacketBuffer) []byte {
	v := pb.ToView()
	out := make([]byte, v.Size())
	_ = runtimex.PanicOnError1(v.Read(out))
	return out
}
