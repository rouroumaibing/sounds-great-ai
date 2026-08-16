//go:build !linux && !darwin

package unified

import "time"

// getProcessCPUTime fallback for unsupported platforms. The liveness probe
// still runs (process-alive checks work everywhere); only CPU-growth detection
// is disabled, so a stalled-but-silent process won't be reported there.
func getProcessCPUTime(pid int) time.Duration {
	return 0
}
