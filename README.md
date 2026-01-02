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
wg := &sync.WaitGroup{}
ready := make(chan struct{})
wg.Go(func() {
	listener := runtimex.PanicOnError1(serverStack.ListenTCP(serverEndpoint))
	close(ready)
	conn := runtimex.PanicOnError1(listener.Accept())
	// TODO: do something with the conn
})

// Start the client goroutine.
wg.Go(func() {
	<-ready
	ctx := context.Background()
	conn := runtimex.PanicOnError1(clientStack.DialTCP(ctx, serverEndpoint))
	// TODO: do something with the conn
})

// Wait for both goroutines to finish.
stopped := make(chan struct{})
go func() {
	wg.Wait()
	close(stopped)
}()

// Route packets between stacks until both sides finish.
loop:
for {
	select {
	case pkt := <-internet.InFlight():
		// TODO: alter/drop/inspect packets here
		_ = internet.Deliver(pkt)
	case <-stopped:
		break loop
	}
}
```

The [example_test.go](example_test.go) file shows a complete example.

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
