// SPDX-License-Identifier: GPL-3.0-or-later

// Package uis (Userspace Internet Simulation) provides a simple internet
// simulation framework using gVisor for censorship testing.
//
// The package models a virtual internet where multiple network stacks can
// communicate. It provides full control over packet flow, enabling simulation
// of delays, packet loss, censorship, and other network conditions.
//
// The typical usage is to create a [*Internet] and use [*Internet.NewStack]
// to create two or more [*Stack] instances. The created instances are already
// configured for sending and receiving raw internet packets.
//
// To route packets, you need to read packets using [*Internet.InFlight]. If
// you choose to forward the read packets, then you can deliver them to the right
// destination using [*Internet.Deliver]. We don't model L2 frames (we just move
// raw IP packets around) and we don't model multiple hops. These choices are
// consistent with our goals of modelling censorship at L4 and above.
//
// On the application level side, a [*Stack] allows to create basic TCP and
// UDP connections via methods such as [*Stack.DialTCP].
//
// The [*PcapTrace] type allows you to capture packets in flight in a PCAP format
// so that you can inspect what happened using tools such as wireshark.
package uis
