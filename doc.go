// SPDX-License-Identifier: GPL-3.0-or-later

// Package uis (Userspace Internet Simulation) provides basic building blocks
// for userspace networking tests using gVisor.
//
// The package models a virtual internet where multiple network stacks can
// communicate. It provides direct control over packet flow, leaving routing
// policy and network conditions to the caller.
//
// The typical usage is to create a [*Internet] and use [*Internet.NewStack]
// to create two or more [*Stack] instances. The created instances are already
// configured for sending and receiving raw internet packets.
//
// The [Connector] type is a stdlib-like dialer for IP literal endpoints only.
// The [ListenConfig] type is a stdlib-like listener config for IP literal
// endpoints only. Use these types to plug this package into higher-level
// code that expects the net package interfaces.
//
// To route packets, you need to read packets using [*Internet.InFlight]. If
// you choose to forward the read packets, then you can deliver them to the right
// destination using [*Internet.Deliver]. We don't model L2 frames (we just move
// raw IP packets around) and we don't model multiple hops. These choices keep
// this package focused on fundamental primitives rather than full frameworks.
//
// On the application level side, a [*Stack] allows to create basic TCP and
// UDP connections via methods such as [*Stack.DialTCP].
//
// The [*PCAPTrace] type allows you to capture packets in flight in a PCAP format
// so that you can inspect what happened using tools such as wireshark.
package uis
