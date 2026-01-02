// SPDX-License-Identifier: GPL-3.0-or-later

package uis_test

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bassosimone/iotest"
	"github.com/bassosimone/uis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPCAPTraceCloseHeaderWriteError(t *testing.T) {
	writeErr := errors.New("mocked write error")
	closeErr := errors.New("mocked close error")
	wc := &iotest.FuncWriteCloser{
		WriteFunc: func([]byte) (int, error) {
			return 0, writeErr
		},
		CloseFunc: func() error {
			return closeErr
		},
	}
	trace := uis.NewPCAPTrace(wc, uis.MTUEthernet)
	err := trace.Close()
	require.Error(t, err)
	assert.True(t, errors.Is(err, writeErr))
	assert.True(t, errors.Is(err, closeErr))
}

func TestPCAPTraceDroppedWhenBufferFull(t *testing.T) {
	gate := make(chan struct{})
	wc := &iotest.FuncWriteCloser{
		WriteFunc: func(b []byte) (int, error) {
			<-gate
			return len(b), nil
		},
		CloseFunc: func() error {
			return nil
		},
	}
	trace := uis.NewPCAPTrace(wc, uis.MTUEthernet, uis.PCAPTraceOptionBuffer(1))
	trace.Dump([]byte{0x00})
	trace.Dump([]byte{0x01})
	assert.Equal(t, uint64(1), trace.Dropped())
	close(gate)
	require.NoError(t, trace.Close())
}

func TestPCAPTraceFirstPacketWriteFails(t *testing.T) {
	// prepare the mock for failing during the first write
	writeErr := errors.New("mocked write error")
	closeErr := errors.New("mocked close error")
	var countWrites uint32
	packetWrite := make(chan struct{})
	wc := &iotest.FuncWriteCloser{
		WriteFunc: func(b []byte) (int, error) {
			if atomic.AddUint32(&countWrites, 1) == 1 {
				return len(b), nil
			}
			close(packetWrite)
			return 0, writeErr
		},
		CloseFunc: func() error {
			return closeErr
		},
	}

	// create the dumper and dump the first packet whose write should fail
	trace := uis.NewPCAPTrace(wc, uis.MTUEthernet)
	trace.Dump([]byte{0x00})

	// wait for the first write to happen befor continuing
	<-packetWrite

	// close the dumper and check we see both errors
	err := trace.Close()
	t.Log(err)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), writeErr.Error()))
	assert.True(t, errors.Is(err, closeErr))
}
