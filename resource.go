package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// runtimeRSSBytes returns one point-in-time resident-set-size sample for the
// managed llama-server process. It is intentionally not labeled peak memory or
// total unified-memory use: Metal/KV-cache accounting can extend beyond process
// RSS, especially on macOS.
func runtimeRSSBytes(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	output, err := exec.Command("/bin/ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	kb, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || kb <= 0 {
		return 0
	}
	return kb * 1024
}
