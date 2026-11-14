// SPDX-License-Identifier: AGPL-3.0-only
//go:build !windows

package proc

import "errors"

// errNotSupported marks the pre-Linux-adaptation stubs.
var errNotSupported = errors.New("proc ops not supported on this platform yet")

func newSysOps() sysOps { return &otherOps{} }

type otherOps struct{}

func (o *otherOps) listProcesses() ([]procInfo, error) {
	return nil, errNotSupported
}

func (o *otherOps) trimWorkingSet(pid uint32) (int64, error) {
	return 0, errNotSupported
}

func (o *otherOps) setPriority(pid uint32, pri uint32) error {
	return errNotSupported
}

func (o *otherOps) connSamples() ([]connSample, error) {
	return nil, nil
}

func (o *otherOps) qosApply(exe string, rateKBps uint32) error {
	return errNotSupported
}

func (o *otherOps) isElevated() bool {
	return false
}

func (o *otherOps) foregroundPID() (uint32, error) { return 0, errNotSupported }
