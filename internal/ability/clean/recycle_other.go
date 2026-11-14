// SPDX-License-Identifier: AGPL-3.0-only
//go:build !windows

package clean

// shellRecycler is a placeholder until the Linux/UOS/麒麟 platform adapter
// defines the recycle-bin equivalent (freedesktop Trash spec).
type shellRecycler struct{}

func (s *shellRecycler) Query(drive string) (int64, int64, error) {
	return 0, 0, nil
}

func (s *shellRecycler) Empty(drive string) error {
	return nil
}

func fixedDrives() []string {
	return nil
}
