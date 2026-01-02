# Userspace Internet Simulation

[![GoDoc](https://pkg.go.dev/badge/github.com/bassosimone/uis)](https://pkg.go.dev/github.com/bassosimone/uis) [![Build Status](https://github.com/bassosimone/uis/actions/workflows/go.yml/badge.svg)](https://github.com/bassosimone/uis/actions) [![codecov](https://codecov.io/gh/bassosimone/uis/branch/main/graph/badge.svg)](https://codecov.io/gh/bassosimone/uis)

Userspace Internet Simulation (`uis`) is a Go package that provides
you with the ability to: create userspace TCP/IP client and server
stacks that communicate exchanging raw IP packets.

As such, it is a fundamental building block for writing integration
tests where the test cases interfere with the exchanged packets.

Basic usage is like:

```go
import (
	"context"
	"net/netip"
	"os"
	"sync"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/uis"
)

// Create the virtual internet and two TCP/IP stacks.
internet := uis.NewInternet()

serverAddr := netip.MustParseAddr("10.0.0.1")
serverEndpoint := netip.AddrPortFrom(serverAddr, 80)
serverStack := runtimex.PanicOnError1(internet.NewStack(uis.MTUEthernet, serverAddr))
defer serverStack.Close()

clientAddr := netip.MustParseAddr("10.0.0.2")
clientStack := runtimex.PanicOnError1(internet.NewStack(uis.MTUEthernet, clientAddr))
defer clientStack.Close()

// Start the server goroutine.
ctx := context.Background()
wg := &sync.WaitGroup{}
ready := make(chan struct{})
wg.Go(func() {
	listenCfg := uis.NewListenConfig(serverStack)
	listener := runtimex.PanicOnError1(listenCfg.Listen(ctx, "tcp", serverEndpoint.String()))
	close(ready)
	conn := runtimex.PanicOnError1(listener.Accept())
	// TODO: do something with the conn
})

// Start the client goroutine.
wg.Go(func() {
	<-ready
	connector := uis.NewConnector(clientStack)
	conn := runtimex.PanicOnError1(connector.DialContext(ctx, "tcp", serverEndpoint.String()))
	// TODO: do something with the conn
})

// Wait for both goroutines to finish.
stopped := make(chan struct{})
go func() {
	wg.Wait()
	close(stopped)
}()

// Route and capture packets between stacks until both sides finish.
traceFile := runtimex.PanicOnError1(os.Create("capture.pcap"))
trace := uis.NewPCAPTrace(traceFile, uis.MTUEthernet)
loop:
for {
	select {
	case frame := <-internet.InFlight():
		trace.Dump(frame.Packet)
		_ = internet.Deliver(frame)
	case <-stopped:
		break loop
	}
}
runtimex.PanicOnError0(trace.Close())
```

The [example_test.go](example_test.go) file shows a complete example.

## Stdlib Compatibility

- Connector: a stdlib-like dialer for IP literal endpoints only.
- ListenConfig: a stdlib-like listener config for IP literal endpoints only.

Because we implement these two fundamental stdlib-like interfaces, `uis` is
suitable to be used *instead of* stdlib-based code in tests. Common networking
code could depend on `DialContext`, `Listen`, and `ListenPacket` like
functions. For example:

```go
// This is how you could define a Dialer that uses either the [*net.Dialer]
// or [*uis.Connector] to establish TCP/UDP connections.

type Connector interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Resolver interface {
	LookupHost(ctx context.Context, name string) ([]string, error)
}

type Dialer struct {
	Connector Connector
	Resolver  Resolver
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addrs, err := d.Resolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		conn, err := d.Connector.DialContext(ctx, network, net.JoinHostPort(addr, port))
		if err != nil {
			continue
		}
		return conn, nil
	}
	return nil, errors.New("dial failed")
}

// This is how you would use the above code in production
dialerProd := &Dialer{Connector: &net.Dialer{}, Resolver: &net.Resolver{}}

// This is instead how you would use the above code for tests
// assuming you also implemented a Resolver based on `uis`.
dialerTests := &Dialer{
	Connector: uis.NewConnector(stack),
	Resolver: NewUISResolver(stack),
}
```

## Installation

To add this package as a dependency to your module:

```sh
go get github.com/bassosimone/uis
```

## Development

To run the tests:
```sh
go test -v .
```

To measure test coverage:
```sh
go test -v -cover .
```

## License

```
SPDX-License-Identifier: GPL-3.0-or-later
```

## History

Adapted from [ooni/netem](https://github.com/ooni/netem).
