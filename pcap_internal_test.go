// SPDX-License-Identifier: GPL-3.0-or-later

package uis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPCAPTraceReadOrDrainAfterCancelWithSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tr := &PCAPTrace{
		snaps: make(chan pcapSnapshot, 1),
	}
	tr.testCancellationDrainHook = func() {
		tr.snaps <- pcapSnapshot{data: []byte{0x01}, length: 1}
	}

	snap, ok := tr.readOrDrain(ctx)
	require.True(t, ok)
	require.Equal(t, 1, snap.length)
	require.Equal(t, []byte{0x01}, snap.data)
}

func TestPCAPTraceReadOrDrainAfterCancelEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tr := &PCAPTrace{
		snaps: make(chan pcapSnapshot),
	}

	_, ok := tr.readOrDrain(ctx)
	require.False(t, ok)
}
