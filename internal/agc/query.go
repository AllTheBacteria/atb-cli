package agc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
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

// parseList returns the non-empty, whitespace-trimmed lines of r. agc's flat
// list sub-commands (listset, listref) print one name per line to stdout.
// listctg is grouped, not flat — use parseContigList for it.
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

// parseContigList parses agc listctg output, which groups contigs beneath a
// sample-name header: the sample name sits in column 0 and its contigs are
// indented under it. It returns the indented contig names (trimmed), skipping
// the column-0 sample headers and blank lines.
func parseContigList(r io.Reader) []string {
	var out []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		raw := sc.Text()
		if raw == "" || (raw[0] != ' ' && raw[0] != '\t') {
			continue // blank line, or a column-0 sample header
		}
		if name := strings.TrimSpace(raw); name != "" {
			out = append(out, name)
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
	return parseContigList(bytes.NewReader(out)), nil
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

// runStream runs agc with args, streaming its stdout into w. agc's stderr is
// forwarded to the process stderr (so progress is visible) and also captured so
// it can be surfaced in the error on a non-zero exit.
func runStream(args []string, w io.Writer) error {
	bin, err := FindBinary()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout = w
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("agc %s: %s", args[0], msg)
		}
		return fmt.Errorf("agc %s: %w", args[0], err)
	}
	return nil
}

// GetContigs extracts the given contig queries as FASTA into w (agc getctg).
// Each query is contig[@sample][:from-to].
func GetContigs(archive string, queries []string, w io.Writer, o Options) error {
	args := append([]string{"getctg"}, buildGetArgs(o, true)...)
	args = append(args, archive)
	args = append(args, queries...)
	return runStream(args, w)
}

// GetSamples extracts whole samples as FASTA into w (agc getset).
func GetSamples(archive string, samples []string, w io.Writer, o Options) error {
	args := append([]string{"getset"}, buildGetArgs(o, true)...)
	args = append(args, archive)
	args = append(args, samples...)
	return runStream(args, w)
}

// GetCollection extracts the entire archive as FASTA into w (agc getcol).
// getcol does not support streaming mode, so Options.Streaming is ignored.
func GetCollection(archive string, w io.Writer, o Options) error {
	args := append([]string{"getcol"}, buildGetArgs(o, false)...)
	args = append(args, archive)
	return runStream(args, w)
}
