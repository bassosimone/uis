// SPDX-License-Identifier: GPL-3.0-or-later

package uis_test

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"sync"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/uis"
)

// This example creates a client and the server and the client
// downloads a small number of bytes from the server.
func Example_tcpDownloadIPv4() {
	// create the internet instance.
	ix := uis.NewInternet(uis.InternetOptionMaxInflight(256))

	// create the server and client stacks
	const mtu = uis.MTUJumbo
	srv := runtimex.PanicOnError1(ix.NewStack(mtu, netip.MustParseAddr("10.0.0.1")))
	defer srv.Close()

	clnt := runtimex.PanicOnError1(ix.NewStack(mtu, netip.MustParseAddr("10.0.0.2")))
	defer clnt.Close()

	// create a context used by connector and listener
	ctx := context.Background()

	// run the server in the background
	wg := &sync.WaitGroup{}
	ready := make(chan struct{})
	wg.Go(func() {
		listenCfg := uis.NewListenConfig(srv)
		listener := runtimex.PanicOnError1(listenCfg.Listen(ctx, "tcp", "10.0.0.1:80"))
		close(ready)
		conn := runtimex.PanicOnError1(listener.Accept())
		message := []byte("Hello, world!\n")
		_ = runtimex.PanicOnError1(conn.Write(message))
		runtimex.PanicOnError0(conn.Close())
		runtimex.PanicOnError0(listener.Close())
	})

	// run the client in the background
	messagech := make(chan []byte, 1)
	wg.Go(func() {
		<-ready
		connector := uis.NewConnector(clnt)
		conn := runtimex.PanicOnError1(connector.DialContext(ctx, "tcp", "10.0.0.1:80"))
		buffer := make([]byte, 1024)
		count := runtimex.PanicOnError1(conn.Read(buffer))
		messagech <- buffer[:count]
		runtimex.PanicOnError0(conn.Close())
	})

	// know when both goroutines have stopped
	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()

	// route and capture packets in the foreground
	traceFile := runtimex.PanicOnError1(os.Create("tcpDownloadIPv4.pcap"))
	trace := uis.NewPcapTrace(traceFile, uis.MTUJumbo)
loop:
	for {
		select {
		case frame := <-ix.InFlight():
			trace.Dump(frame.Packet)
			_ = ix.Deliver(frame)
		case <-stopped:
			break loop
		}
	}
	runtimex.PanicOnError0(trace.Close())

	// receive and print the server message
	message := <-messagech
	fmt.Printf("%s", string(message))

	// Output:
	// Hello, world!
	//
}

// This example creates a client and server using UDP over IPv4.
// The server echoes back whatever it receives.
func Example_udpEchoIPv4() {
	// create the internet instance.
	ix := uis.NewInternet(uis.InternetOptionMaxInflight(256))

	// create the server and client stacks
	const mtu = uis.MTUJumbo
	srv := runtimex.PanicOnError1(ix.NewStack(mtu, netip.MustParseAddr("10.0.0.1")))
	defer srv.Close()

	clnt := runtimex.PanicOnError1(ix.NewStack(mtu, netip.MustParseAddr("10.0.0.2")))
	defer clnt.Close()

	// create a context used by connector and listener
	ctx := context.Background()

	// run the server in the background
	wg := &sync.WaitGroup{}
	ready := make(chan struct{})
	wg.Go(func() {
		listenCfg := uis.NewListenConfig(srv)
		pconn := runtimex.PanicOnError1(listenCfg.ListenPacket(ctx, "udp", "10.0.0.1:53"))
		defer pconn.Close()
		close(ready)
		buffer := make([]byte, 2048)
		count, addr := runtimex.PanicOnError2(pconn.ReadFrom(buffer))
		_ = runtimex.PanicOnError1(pconn.WriteTo(buffer[:count], addr))
	})

	// run the client in the background
	messagech := make(chan []byte, 1)
	wg.Go(func() {
		<-ready
		connector := uis.NewConnector(clnt)
		conn := runtimex.PanicOnError1(connector.DialContext(ctx, "udp", "10.0.0.1:53"))
		message := []byte("Hello, IPv4!\n")
		_ = runtimex.PanicOnError1(conn.Write(message))
		buffer := make([]byte, 1024)
		count := runtimex.PanicOnError1(conn.Read(buffer))
		messagech <- buffer[:count]
		runtimex.PanicOnError0(conn.Close())
	})

	// know when both goroutines have stopped
	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()

	// route and capture packets in the foreground
	traceFile := runtimex.PanicOnError1(os.Create("udpEchoIPv4.pcap"))
	trace := uis.NewPcapTrace(traceFile, uis.MTUJumbo)
loop:
	for {
		select {
		case frame := <-ix.InFlight():
			trace.Dump(frame.Packet)
			_ = ix.Deliver(frame)
		case <-stopped:
			break loop
		}
	}
	runtimex.PanicOnError0(trace.Close())

	// receive and print the echoed message
	message := <-messagech
	fmt.Printf("%s", string(message))

	// Output:
	// Hello, IPv4!
	//
}

// This example creates a client and server using UDP over IPv6.
// The server echoes back whatever it receives.
func Example_udpEchoIPv6() {
	// create the internet instance.
	ix := uis.NewInternet(uis.InternetOptionMaxInflight(256))

	// create the server and client stacks
	const mtu = uis.MTUJumbo
	srv := runtimex.PanicOnError1(ix.NewStack(mtu, netip.MustParseAddr("2001:db8::1")))
	defer srv.Close()

	clnt := runtimex.PanicOnError1(ix.NewStack(mtu, netip.MustParseAddr("2001:db8::2")))
	defer clnt.Close()

	// create a context used by connector and listener
	ctx := context.Background()

	// run the server in the background
	wg := &sync.WaitGroup{}
	ready := make(chan struct{})
	wg.Go(func() {
		listenCfg := uis.NewListenConfig(srv)
		pconn := runtimex.PanicOnError1(listenCfg.ListenPacket(ctx, "udp", "[2001:db8::1]:53"))
		defer pconn.Close()
		close(ready)
		buffer := make([]byte, 2048)
		count, addr := runtimex.PanicOnError2(pconn.ReadFrom(buffer))
		_ = runtimex.PanicOnError1(pconn.WriteTo(buffer[:count], addr))
	})

	// run the client in the background
	messagech := make(chan []byte, 1)
	wg.Go(func() {
		<-ready
		connector := uis.NewConnector(clnt)
		conn := runtimex.PanicOnError1(connector.DialContext(ctx, "udp", "[2001:db8::1]:53"))
		message := []byte("Hello, IPv6!\n")
		_ = runtimex.PanicOnError1(conn.Write(message))
		buffer := make([]byte, 1024)
		count := runtimex.PanicOnError1(conn.Read(buffer))
		messagech <- buffer[:count]
		runtimex.PanicOnError0(conn.Close())
	})

	// know when both goroutines have stopped
	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()

	// route and capture packets in the foreground
	traceFile := runtimex.PanicOnError1(os.Create("udpEchoIPv6.pcap"))
	trace := uis.NewPcapTrace(traceFile, uis.MTUJumbo)
loop:
	for {
		select {
		case frame := <-ix.InFlight():
			trace.Dump(frame.Packet)
			_ = ix.Deliver(frame)
		case <-stopped:
			break loop
		}
	}
	runtimex.PanicOnError0(trace.Close())

	// receive and print the echoed message
	message := <-messagech
	fmt.Printf("%s", string(message))

	// Output:
	// Hello, IPv6!
	//
}
