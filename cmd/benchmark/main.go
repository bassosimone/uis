// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/uis"
)

var (
	// args contains the command line arguments (overridable in tests).
	args = os.Args

	// output is the writer for benchmark output (overridable in tests).
	output io.Writer = os.Stdout
)

// serverMain accepts once and writes bytes until the conn is closed.
func serverMain(listener net.Listener, total *atomic.Uint64) {
	// 1. accept a single client conn
	conn := runtimex.PanicOnError1(listener.Accept())
	defer conn.Close()

	// 2. loop writing data to the client
	data := make([]byte, 65535)
	for {
		count, err := conn.Write(data)
		if err != nil {
			log.Printf("server: Write failed: %s", err.Error())
			return
		}
		total.Add(uint64(count))
	}
}

// clientMain connects and reads bytes until the conn is closed.
func clientMain(ctx context.Context, connector *uis.Connector, remote string, total *atomic.Uint64) {
	// 1. connect to the server address
	conn := runtimex.PanicOnError1(connector.DialContext(ctx, "tcp", remote))
	defer conn.Close()

	// 2. read until possible
	data := make([]byte, 65535)
	for {
		count, err := conn.Read(data)
		if err != nil {
			log.Printf("client: Read failed: %s", err.Error())
			return
		}
		total.Add(uint64(count))
	}
}

// printerMain prints receive speed stats every 250 millisecond.
func printerMain(ctx context.Context, total *atomic.Uint64) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	t0 := time.Now()
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(output, "\n")
			return
		case t := <-ticker.C:
			elapsed := t.Sub(t0).Seconds()
			nbytes := total.Load()
			speed := (8 * float64(nbytes) / elapsed) / (1000 * 1000)
			fmt.Fprintf(output, "\r\t%10.3f Mbit/s", speed)
		}
	}
}

// routerMain routes packets until the context is done.
func routerMain(ctx context.Context, ix *uis.Internet, pcapFile string, snaplen uint16) (err error) {
	var tr *uis.PCAPTrace

	if pcapFile != "" {
		filep := runtimex.PanicOnError1(os.Create(pcapFile))
		tr = uis.NewPCAPTrace(filep, snaplen)
		defer func() {
			err = tr.Close()
		}()
	}

	for {
		select {
		case frame := <-ix.InFlight():
			if tr != nil {
				tr.Dump(frame.Packet)
			}
			ix.Deliver(frame)

		case <-ctx.Done():
			return
		}
	}
}

func main() {
	// 1. create command line parser
	fset := flag.NewFlagSet("benchmark", flag.ExitOnError)

	// 2. add flags to parse
	var (
		clientAddr  = fset.String("client-addr", "10.0.0.2", "Select client IP address.")
		duration    = fset.Duration("duration", 10*time.Second, "Benchmark duration.")
		pcapFile    = fset.String("pcap-file", "", "Write PCAP at the given file.")
		pcapSnaplen = fset.Int("pcap-snaplen", 1500, "PCAP snapshot length in bytes.")
		serverAddr  = fset.String("server-addr", "10.0.0.1", "Select server IP address.")
		serverPort  = fset.String("server-port", "443", "Select server port.")
	)

	// 3. parse command line
	runtimex.PanicOnError0(fset.Parse(args[1:]))

	// 4. create context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// 5. create the internet instance
	ix := uis.NewInternet()

	// 6. create the server virtual stack
	serverIPAddr := netip.MustParseAddr(*serverAddr)
	serverStack := runtimex.PanicOnError1(ix.NewStack(65535, serverIPAddr))
	defer serverStack.Close()

	// 7. create the server listener
	serverEpnt := net.JoinHostPort(*serverAddr, *serverPort)
	lc := uis.NewListenConfig(serverStack)
	listener := runtimex.PanicOnError1(lc.Listen(ctx, "tcp", serverEpnt))
	defer listener.Close()

	// 8. spawn the server goroutine
	wg := &sync.WaitGroup{}
	totalSent := &atomic.Uint64{}
	wg.Go(func() {
		serverMain(listener, totalSent)
	})

	// 9. create the client virtual stack
	clientIPAddr := netip.MustParseAddr(*clientAddr)
	clientStack := runtimex.PanicOnError1(ix.NewStack(65535, clientIPAddr))
	defer clientStack.Close()

	// 10. spawn the client goroutine
	totalRecv := &atomic.Uint64{}
	connector := uis.NewConnector(clientStack)
	wg.Go(func() {
		clientMain(ctx, connector, serverEpnt, totalRecv)
	})

	// 11. spawn the goroutine counting bytes
	wg.Go(func() {
		printerMain(ctx, totalRecv)
	})

	// 12. route packets until done
	runtimex.PanicOnError0(routerMain(ctx, ix, *pcapFile, uint16(*pcapSnaplen)))

	// 13. shut down the stacks explicitly
	clientStack.Close()
	serverStack.Close()

	// 14. wait for goroutines to finish
	wg.Wait()
}
