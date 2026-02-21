// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"io"
	"testing"
)

// Test_main exercises the benchmark for a short duration.
func Test_main(t *testing.T) {
	args = []string{"benchmark", "-duration", "500ms", "-pcap-file", "capture.pcap"}
	output = io.Discard
	main()
}
