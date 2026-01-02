// SPDX-License-Identifier: GPL-3.0-or-later

package uis

import (
	"fmt"
	"net/netip"
	"sync"
)

// Internet models the entire internet.
//
// Construct using [NewInternet].
type Internet struct {
	// inflight is the channel receiving inflight packets.
	inflight chan VNICFrame

	// mu provides mutual exclusion.
	mu sync.RWMutex

	// routes contains the known routes.
	routes map[netip.Addr]*VNIC
}

// InternetOption is an option for [NewInternet].
type InternetOption func(cfg *internetConfig)

// internetConfig is the internal type modified by [InternetOption].
type internetConfig struct {
	maxInflight int
}

// DefaultMaxInflight is the default maximum number of inflight packets.
const DefaultMaxInflight = 1024

// InternetOptionMaxInflight sets the maximum number of inflight packets.
//
// The default is [DefaultMaxInflight] packets. When the channel is
// full, additional packets are silently dropped.
func InternetOptionMaxInflight(max int) InternetOption {
	return func(cfg *internetConfig) {
		cfg.maxInflight = max
	}
}

// NewInternet creates and returns a new [*Internet] instance.
func NewInternet(options ...InternetOption) *Internet {
	cfg := &internetConfig{
		maxInflight: DefaultMaxInflight,
	}
	for _, opt := range options {
		opt(cfg)
	}

	return &Internet{
		inflight: make(chan VNICFrame, cfg.maxInflight),
		mu:       sync.RWMutex{},
		routes:   make(map[netip.Addr]*VNIC),
	}
}

// NewVNIC constructs a new [*VNIC] attached to the [*Internet].
//
// The mtu parameter sets the MTU in bytes. Common values:
//
// - [MTUEthernet]
// - [MTUMinimumIPv6]
// - [MTUJumbo]
//
// This method internally invokes the [NewVNIC] factory func.
func (ix *Internet) NewVNIC(mtu uint32) *VNIC {
	return NewVNIC(mtu, internetVNICNetwork{ix: ix})
}

// AddRoute registers the given [*VNIC] to have the given addresses
// such that it is possible to route packets to it.
//
// This method fails if the claimed addresses are already in use.
func (ix *Internet) AddRoute(vnic *VNIC, addrs ...netip.Addr) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	for _, addr := range addrs {
		if _, found := ix.routes[addr]; found {
			return fmt.Errorf("duplicate address detected: %s", addr.String())
		}
		ix.routes[addr] = vnic
	}
	return nil
}

// NewStack creates and attaches a [*Stack] to the [*Internet].
//
// The mtu parameter sets the MTU in bytes. Common values:
//
// - [MTUEthernet]
// - [MTUMinimumIPv6]
// - [MTUJumbo]
//
// The addrs argument contains the IPv4/IPv6 addresses to configure.
//
// This method implementation combines:
//
// 1. [*Internet.NewVNIC] to create a virtual NIC
//
// 2. [NewStack] to create a [*Stack] associated to a virtual NIC
//
// 3. [*Internet.AddrRoute] to create the return routes
func (ix *Internet) NewStack(mtu uint32, addrs ...netip.Addr) (*Stack, error) {
	vnic := ix.NewVNIC(mtu)
	stack := NewStack(vnic, addrs...)
	if err := ix.AddRoute(vnic, addrs...); err != nil {
		return nil, err
	}
	return stack, nil
}

// internetVNICNetwork adapts the [*Internet] to be a [VNICNetwork].
type internetVNICNetwork struct {
	ix *Internet
}

// Ensure that [internetVNICAdapter] implements [VNICNetwork].
var _ VNICNetwork = internetVNICNetwork{}

// SendFrame implements [VNICNetwork].
func (n internetVNICNetwork) SendFrame(frame VNICFrame) bool {
	select {
	case n.ix.inflight <- frame:
		return true
	default:
		return false
	}
}

// InFlight returns the channel where the in flight [VNICFrame] are posted.
func (ix *Internet) InFlight() <-chan VNICFrame {
	return ix.inflight
}

// Deliver routes a frame to the appropriate host based on destination IP.
//
// It parses the destination IP from the raw packet, looks up the registered
// host for that address, and injects the frame into that host stack.
//
// Returns false if the destination IP cannot be parsed, is not routable
// (no host registered for that address), or injection fails.
func (ix *Internet) Deliver(frame VNICFrame) bool {
	// Parse the destination IP from the raw packet
	dstIP, ok := internetParseDestinationIP(frame.Packet)
	if !ok {
		return false
	}

	// Look up the NIC for this destination
	ix.mu.RLock()
	nic := ix.routes[dstIP]
	ix.mu.RUnlock()

	// Drop if no route exists (including broadcast/multicast/unknown)
	if nic == nil {
		return false
	}

	// Inject the frame into the destination NIC
	return nic.InjectFrame(frame)
}

// internetParseDestinationIP extracts the destination IP from a raw IP packet.
func internetParseDestinationIP(pkt []byte) (netip.Addr, bool) {
	if len(pkt) < 1 {
		return netip.Addr{}, false
	}

	version := pkt[0] >> 4
	switch version {
	case 4:
		// IPv4: destination is at bytes 16-19
		if len(pkt) < 20 {
			return netip.Addr{}, false
		}
		addr, ok := netip.AddrFromSlice(pkt[16:20])
		return addr, ok

	case 6:
		// IPv6: destination is at bytes 24-39
		if len(pkt) < 40 {
			return netip.Addr{}, false
		}
		addr, ok := netip.AddrFromSlice(pkt[24:40])
		return addr, ok

	default:
		return netip.Addr{}, false
	}
}
