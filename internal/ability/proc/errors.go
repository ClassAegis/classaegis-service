package proc

import "errors"

// ErrElevation maps to E_PLATFORM_UNSUPPORTED in receipts: the throttle
// needs an elevated process to write HKLM policy.
var ErrElevation = errors.New("elevation required")
