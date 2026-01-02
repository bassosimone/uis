//
// SPDX-License-Identifier: BSD-3-Clause
//
// Adapted from: https://github.com/ooni/netem/blob/6e0d618f0cb48b96c78cd066e23cf3aa1208b1dd/pcap.go
//

package uis

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// pcapSnapshot is a packet snapshot.
type pcapSnapshot struct {
	// data is the data inside the snapshot.
	data []byte

	// length is the original length.
	length int
}

// PcapTrace is an open pcap trace.
type PcapTrace struct {
	// cancel allows to cancel the background goroutine.
	cancel context.CancelFunc

	// dropped is the number of packets dropped.
	dropped atomic.Uint64

	// errch contains the error returned by the background goroutine.
	errch chan error

	// snaps contains an snaps snapshot.
	snaps chan pcapSnapshot

	// once provides "once" semantics for Close.
	once sync.Once

	// snapSize is the number of bytes to capture.
	snapSize uint16

	// wc is the open writer we're using.
	wc io.WriteCloser
}

// NewPcapTrace creates a new [*PcapTrace] instance.
func NewPcapTrace(wc io.WriteCloser, snapSize uint16) *PcapTrace {
	// Initialize the trace struct
	ctx, cancel := context.WithCancel(context.Background())
	const manyPackets = 4096
	tr := &PcapTrace{
		cancel:   cancel,
		dropped:  atomic.Uint64{},
		errch:    make(chan error, 1),
		snaps:    make(chan pcapSnapshot, manyPackets),
		once:     sync.Once{},
		snapSize: snapSize,
		wc:       wc,
	}

	// Start the worker and return
	go tr.saveLoop(ctx)
	return tr
}

// Dump dumps the information about the given raw IPv4/IPv6 packet.
func (tr *PcapTrace) Dump(packet []byte) {
	snapSize := min(len(packet), int(tr.snapSize))
	packetSnap := make([]byte, snapSize)
	copy(packetSnap, packet)
	select {
	case tr.snaps <- pcapSnapshot{length: len(packet), data: packetSnap}:
	default:
		tr.dropped.Add(1)
	}
}

// Dropped returns the number of packets dropped due to buffer overflow.
//
// Packets are dropped when Dump is called but the internal buffer is full.
// This happens when disk I/O cannot keep up with packet capture rate.
func (tr *PcapTrace) Dropped() uint64 {
	return tr.dropped.Load()
}

// saveLoop is the loop that dumps packets
func (tr *PcapTrace) saveLoop(ctx context.Context) {
	// Write the PCAP header
	w := pcapgo.NewWriter(tr.wc)
	if err := w.WriteFileHeader(uint32(tr.snapSize), layers.LinkTypeRaw); err != nil {
		tr.errch <- err
		return
	}

	// Loop until we're done and write each entry.
	//
	// Make sure we drain the buffer on exit.
	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case snap := <-tr.snaps:
					if err := tr.savePacket(w, snap); err != nil {
						tr.errch <- nil
						return
					}
				default:
					tr.errch <- nil
					return
				}
			}

		case snap := <-tr.snaps:
			if err := tr.savePacket(w, snap); err != nil {
				tr.errch <- nil
				return
			}
		}
	}
}

func (tr *PcapTrace) savePacket(w *pcapgo.Writer, pinfo pcapSnapshot) error {
	ci := gopacket.CaptureInfo{
		Timestamp:      time.Now(),
		CaptureLength:  len(pinfo.data),
		Length:         pinfo.length,
		InterfaceIndex: 0,
		AncillaryData:  []any{},
	}
	return w.WritePacket(ci, pinfo.data)
}

// Close interrupts the background goroutine and waits for it to join
// before closing the packet capture file.
func (tr *PcapTrace) Close() (err error) {
	tr.once.Do(func() {
		// notify the background goroutine to terminate
		tr.cancel()

		// wait for the goroutine to terminate
		err1 := <-tr.errch

		// close the open capture file
		err2 := tr.wc.Close()

		// assemble a common error (nil on success)
		err = errors.Join(err1, err2)
	})
	return
}
