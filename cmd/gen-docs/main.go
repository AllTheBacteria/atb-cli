// Command gen-docs renders the atb CLI reference as Markdown for the docs site.
// Output goes to docs/reference/cli/ and is committed; run it via `make docs`.
// CI fails if the committed output is stale, so the reference never drifts.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/allthebacteria/atb-cli/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	root := cli.NewRootCmd("docs")
	disableAutoGenTag(root) // strip the dated footer so output is deterministic

	repo, err := repoRoot()
	if err != nil {
		fail(err)
	}
	outDir := filepath.Join(repo, "docs", "reference", "cli")

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}

	filePrepender := func(string) string { return "" }
	linkHandler := func(name string) string { return name } // e.g. "atb_query.md"

	if err := doc.GenMarkdownTreeCustom(root, outDir, filePrepender, linkHandler); err != nil {
		fail(err)
	}
	if err := writeSummary(root, outDir); err != nil {
		fail(err)
	}
	if err := normalizeHomePaths(outDir); err != nil {
		fail(err)
	}
}

// disableAutoGenTag turns off the "Auto generated ... on <date>" footer on every
// command in the tree; the doc package reads the flag per-command, so it must be
// set recursively, not just on the root.
func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, c := range cmd.Commands() {
		disableAutoGenTag(c)
	}
}

// writeSummary emits a literate-nav SUMMARY.md mirroring the generated files, so
// new commands appear in the docs nav automatically.
func writeSummary(root *cobra.Command, dir string) error {
	var b strings.Builder
	var walk func(cmd *cobra.Command, depth int)
	walk = func(cmd *cobra.Command, depth int) {
		label := cmd.Name()
		if depth == 0 {
			label = "atb"
		}
		file := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
		fmt.Fprintf(&b, "%s* [%s](%s)\n", strings.Repeat("    ", depth), label, file)

		children := cmd.Commands()
		sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
		for _, c := range children {
			if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
				continue
			}
			walk(c, depth+1)
		}
	}
	walk(root, 0)
	return os.WriteFile(filepath.Join(dir, "SUMMARY.md"), []byte(b.String()), 0o644)
}

// normalizeHomePaths rewrites the build machine's absolute home directory to a
// portable "~" in every generated page. Cobra renders flag defaults like
// --data-dir / --output-dir with the home-resolved path captured at generation
// time; without this the committed reference would embed the build host's $HOME,
// and the freshness CI (which regenerates on a different host) would flag every
// page as stale. The CLI's own --help is unaffected — only the docs are.
func normalizeHomePaths(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil // nothing to normalize; leave output untouched
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		normalized := strings.ReplaceAll(string(data), home, "~")
		if normalized == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(normalized), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// repoRoot walks up from the working directory to the module root (the
// directory that holds go.mod) so the reference is always written to the
// repository's docs/ tree, not relative to wherever the generator was
// launched. make docs runs from the repo root, but resolving the root
// explicitly removes a silent wrong-directory trap if gen-docs is ever run
// from a subdirectory or by CI from a different working directory.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("gen-docs: go.mod not found above %s; run inside the atb-cli repo (e.g. via `make docs`)", dir)
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
