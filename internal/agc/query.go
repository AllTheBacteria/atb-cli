package agc

import (
	"runtime"
	"strconv"
)

// Options maps to agc's common get-flags. Zero values mean "use agc's default":
// LineLength 0 keeps agc's 80, GzipLevel 0 leaves output uncompressed, Threads
// 0 uses NumCPU-1. Streaming applies to getctg/getset only (getcol ignores it).
type Options struct {
	Threads    int  // -t
	LineLength int  // -l
	GzipLevel  int  // -g
	Streaming  bool // -s
}

// resolveThreads returns the thread count to pass to agc: if n <= 0, it uses
// NumCPU-1 to leave one core for the system.
func resolveThreads(n int) int {
	if n <= 0 {
		cpus := runtime.NumCPU() - 1
		if cpus < 1 {
			cpus = 1
		}
		return cpus
	}
	return n
}

// buildGetArgs renders the shared get-flags. supportsStreaming must be false
// for getcol, which does not accept -s.
func buildGetArgs(o Options, supportsStreaming bool) []string {
	args := []string{"-t", strconv.Itoa(resolveThreads(o.Threads))}
	if o.LineLength > 0 {
		args = append(args, "-l", strconv.Itoa(o.LineLength))
	}
	if o.GzipLevel > 0 {
		args = append(args, "-g", strconv.Itoa(o.GzipLevel))
	}
	if supportsStreaming && o.Streaming {
		args = append(args, "-s")
	}
	return args
}
