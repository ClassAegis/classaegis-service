// SPDX-License-Identifier: AGPL-3.0-only
//go:build !windows

package audit

// readESE is a placeholder until Linux/UOS/麒麟 audit sources are defined.
func (e *Engine) readESE(kind string, limit int) ([]record, error) {
	return nil, errNoSource
}
