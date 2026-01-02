//
// SPDX-License-Identifier: GPL-3.0-or-later
//
// Adapted from: https://github.com/ooni/netem/blob/061c5671b52a2c064cac1de5d464bb056f7ccaa8/unetstack.go
//

package uis

import (
	"net"
	"strings"
	"syscall"
)

// errorsMap maps gVisor error suffixes to stdlib errors.
//
// See https://github.com/google/gvisor/blob/master/pkg/tcpip/errors.go
//
// See https://github.com/google/gvisor/blob/master/pkg/syserr/netstack.go
var errorsMap = map[string]error{
	"endpoint is closed for receive": net.ErrClosed,
	"endpoint is closed for send":    net.ErrClosed,
	"connection aborted":             syscall.ECONNABORTED,
	"connection was refused":         syscall.ECONNREFUSED,
	"connection reset by peer":       syscall.ECONNRESET,
	"network is unreachable":         syscall.ENETUNREACH,
	"no route to host":               syscall.EHOSTUNREACH,
	"host is down":                   syscall.EHOSTDOWN,
	"machine is not on the network":  syscall.ENETDOWN,
	"operation timed out":            syscall.ETIMEDOUT,
	"endpoint is in invalid state":   syscall.EINVAL,
}

// errorsRemap maps a gVisor error to a stdlib error.
func errorsRemap(err error) error {
	if err != nil {
		estring := err.Error()
		for suffix, remapped := range errorsMap {
			if strings.HasSuffix(estring, suffix) {
				return remapped
			}
		}
	}
	return err
}
