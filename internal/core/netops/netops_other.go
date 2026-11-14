// SPDX-License-Identifier: AGPL-3.0-only
//go:build !windows

package netops

import (
	"net"
	"time"
)

func ping(ip net.IP, timeout time.Duration) bool { return false }

func arpTable() []arpRow { return nil }
