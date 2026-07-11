package metrics

import (
	"syscall"
	"time"
)

// processCPUTime returns the cumulative user+system CPU time consumed by this
// process. It reads the OS resource usage, which is available on Linux (the
// container runtime target) and macOS (local development).
func processCPUTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	user := time.Duration(ru.Utime.Sec)*time.Second + time.Duration(ru.Utime.Usec)*time.Microsecond
	sys := time.Duration(ru.Stime.Sec)*time.Second + time.Duration(ru.Stime.Usec)*time.Microsecond
	return user + sys
}
