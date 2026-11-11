//go:build !windows

package media

import "errors"

func confirmScreenMonitoring(int, int, int) bool { return false }
func captureForegroundMediaWindow(string, int) error {
	return errors.New("screen monitor requires Windows")
}
