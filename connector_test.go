// SPDX-License-Identifier: GPL-3.0-or-later

package uis_test

import (
	"context"
	"errors"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/bassosimone/uis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectorDialContextRejectsDomain(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack, err := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	require.NoError(t, err)
	t.Cleanup(stack.Close)

	connector := uis.NewConnector(stack)
	_, err = connector.DialContext(context.Background(), "udp", "example.com:53")
	require.Error(t, err)
}

func TestConnectorDialContextRejectsUnknownNetwork(t *testing.T) {
	vnic := uis.NewVNIC(uis.MTUEthernet, nil)
	stack, err := uis.NewStack(vnic, netip.MustParseAddr("10.0.0.1"))
	require.NoError(t, err)
	t.Cleanup(stack.Close)

	connector := uis.NewConnector(stack)
	_, err = connector.DialContext(context.Background(), "tcp4", "10.0.0.1:80")
	require.Error(t, err)
	assert.True(t, errors.Is(err, syscall.EPROTOTYPE))
}

func TestConnectorDialContextRemapsErrors(t *testing.T) {
	ix := uis.NewInternet(uis.InternetOptionMaxInflight(256))

	srv := require.New(t)
	server, err := ix.NewStack(uis.MTUEthernet, netip.MustParseAddr("10.0.0.1"))
	srv.NoError(err)
	t.Cleanup(server.Close)

	client, err := ix.NewStack(uis.MTUEthernet, netip.MustParseAddr("10.0.0.2"))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	routed := make(chan struct{})
	go func() {
		defer close(routed)
		for {
			select {
			case frame := <-ix.InFlight():
				_ = ix.Deliver(frame)
			case <-ctx.Done():
				return
			}
		}
	}()

	connector := uis.NewConnector(client)
	_, err = connector.DialContext(ctx, "tcp", "10.0.0.1:80")
	require.Error(t, err)
	assert.True(t, errors.Is(err, syscall.ECONNREFUSED))
}
