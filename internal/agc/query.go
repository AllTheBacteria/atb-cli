package agc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
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

// parseList returns the non-empty, whitespace-trimmed lines of r. agc's list
// sub-commands print one name per line to stdout.
func parseList(r io.Reader) []string {
	var out []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// runOutput runs agc with args, returns its stdout, and surfaces stderr on a
// non-zero exit (the exec.ExitError pattern used elsewhere in atb).
func runOutput(args []string) ([]byte, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("agc %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("agc %s: %w", args[0], err)
	}
	return out, nil
}

// ListSamples returns the sample names in the archive (agc listset).
func ListSamples(archive string) ([]string, error) {
	out, err := runOutput([]string{"listset", archive})
	if err != nil {
		return nil, err
	}
	return parseList(bytes.NewReader(out)), nil
}

// ListContigs returns the contig names within sample (agc listctg).
func ListContigs(archive, sample string) ([]string, error) {
	out, err := runOutput([]string{"listctg", archive, sample})
	if err != nil {
		return nil, err
	}
	return parseList(bytes.NewReader(out)), nil
}

// ReferenceSample returns the archive's reference sample name (agc listref).
func ReferenceSample(archive string) (string, error) {
	out, err := runOutput([]string{"listref", archive})
	if err != nil {
		return "", err
	}
	names := parseList(bytes.NewReader(out))
	if len(names) == 0 {
		return "", fmt.Errorf("agc listref: no reference sample reported")
	}
	return names[0], nil
}

// Info returns agc's archive metadata report verbatim (agc info).
func Info(archive string) (string, error) {
	out, err := runOutput([]string{"info", archive})
	if err != nil {
		return "", err
	}
	return string(out), nil
}
