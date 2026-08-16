//go:build darwin

package unified

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// getProcessCPUTime on macOS parses `ps -o time= -p <pid>`, which reports
// cumulative CPU time as [[hh:]mm:]ss[.ccc]. This avoids pulling in a syscall
// package for the relatively infrequent (per-poll) probe; the value is only
// compared for growth, so sub-second precision is irrelevant.
func getProcessCPUTime(pid int) time.Duration {
	out, err := exec.Command("ps", "-o", "time=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	return parsePSTime(strings.TrimSpace(string(out)))
}

func parsePSTime(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	// Normalise potential fractional seconds (.ccc) — drop them, not needed.
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		s = s[:dot]
	}
	parts := strings.Split(s, ":")
	var h, m, sec int
	switch len(parts) {
	case 3:
		h, _ = strconv.Atoi(parts[0])
		m, _ = strconv.Atoi(parts[1])
		sec, _ = strconv.Atoi(parts[2])
	case 2:
		m, _ = strconv.Atoi(parts[0])
		sec, _ = strconv.Atoi(parts[1])
	case 1:
		sec, _ = strconv.Atoi(parts[0])
	default:
		return 0
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}
