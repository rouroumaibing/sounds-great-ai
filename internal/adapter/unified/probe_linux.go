//go:build linux

package unified

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// getProcessCPUTime reads /proc/<pid>/stat and sums utime+stime (fields 14 and
// 15), expressed in clock ticks. USER_HZ (CLK_TCK) is fixed at 100 on Linux
// regardless of the kernel HZ setting, so we convert with that constant.
func getProcessCPUTime(pid int) time.Duration {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0
	}
	// The comm field may contain spaces/parens; find the last ')' to anchor
	// the start of the numeric fields.
	idx := strings.LastIndexByte(string(data), ')')
	if idx < 0 || idx+2 >= len(data) {
		return 0
	}
	fields := strings.Fields(string(data)[idx+2:])
	if len(fields) < 15 {
		return 0
	}
	utime, err1 := strconv.ParseInt(fields[11], 10, 64) // utime (14th, 0-based 11)
	stime, err2 := strconv.ParseInt(fields[12], 10, 64) // stime (15th, 0-based 12)
	if err1 != nil || err2 != nil {
		return 0
	}
	const clkTck = 100 // USER_HZ
	totalTicks := utime + stime
	return time.Duration(totalTicks) * time.Second / time.Duration(clkTck)
}
