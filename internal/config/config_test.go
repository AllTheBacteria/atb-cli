package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.Default()

	wantDataDir := config.DefaultDataDir()
	if cfg.General.DataDir != wantDataDir {
		t.Errorf("General.DataDir = %q, want %q", cfg.General.DataDir, wantDataDir)
	}

	if cfg.General.DefaultFormat != "tsv" {
		t.Errorf("General.DefaultFormat = %q, want %q", cfg.General.DefaultFormat, "tsv")
	}

	if cfg.Fetch.Parallel != 4 {
		t.Errorf("Fetch.Parallel = %d, want 4", cfg.Fetch.Parallel)
	}

	if cfg.Download.Parallel != 4 {
		t.Errorf("Download.Parallel = %d, want 4", cfg.Download.Parallel)
	}

	if cfg.Download.CheckDiskSpace != true {
		t.Errorf("Download.CheckDiskSpace = %v, want true", cfg.Download.CheckDiskSpace)
	}

	if cfg.Download.MinFreeSpaceGB != 10 {
		t.Errorf("Download.MinFreeSpaceGB = %d, want 10", cfg.Download.MinFreeSpaceGB)
	}
}

func TestLoadNonexistent(t *testing.T) {
	nonexistentPath := filepath.Join(t.TempDir(), "does-not-exist.toml")

	cfg, err := config.Load(nonexistentPath)
	if err != nil {
		t.Fatalf("Load(nonexistent) returned error: %v", err)
	}

	defaults := config.Default()
	if cfg.General.DataDir != defaults.General.DataDir {
		t.Errorf("General.DataDir = %q, want %q", cfg.General.DataDir, defaults.General.DataDir)
	}
	if cfg.Fetch.Parallel != defaults.Fetch.Parallel {
		t.Errorf("Fetch.Parallel = %d, want %d", cfg.Fetch.Parallel, defaults.Fetch.Parallel)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.toml")

	original := config.Default()
	original.General.DataDir = "/custom/data/path"
	original.Fetch.BaseURL = "https://example.com/api"
	original.Fetch.Parallel = 8
	original.Download.OutputDir = "/custom/output"
	original.Download.MinFreeSpaceGB = 20

	if err := config.Save(original, path); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loaded.General.DataDir != original.General.DataDir {
		t.Errorf("General.DataDir = %q, want %q", loaded.General.DataDir, original.General.DataDir)
	}
	if loaded.Fetch.BaseURL != original.Fetch.BaseURL {
		t.Errorf("Fetch.BaseURL = %q, want %q", loaded.Fetch.BaseURL, original.Fetch.BaseURL)
	}
	if loaded.Fetch.Parallel != original.Fetch.Parallel {
		t.Errorf("Fetch.Parallel = %d, want %d", loaded.Fetch.Parallel, original.Fetch.Parallel)
	}
	if loaded.Download.OutputDir != original.Download.OutputDir {
		t.Errorf("Download.OutputDir = %q, want %q", loaded.Download.OutputDir, original.Download.OutputDir)
	}
	if loaded.Download.MinFreeSpaceGB != original.Download.MinFreeSpaceGB {
		t.Errorf("Download.MinFreeSpaceGB = %d, want %d", loaded.Download.MinFreeSpaceGB, original.Download.MinFreeSpaceGB)
	}
}

func TestConfigPath(t *testing.T) {
	// Clear XDG_CONFIG_HOME so we get the default fallback path.
	t.Setenv("XDG_CONFIG_HOME", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	got := config.DefaultPath()
	if got == "" {
		t.Error("DefaultPath() returned empty string")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("DefaultPath() = %q, want an absolute path", got)
	}
	// Must live somewhere under the home directory
	if !strings.HasPrefix(got, home) {
		t.Errorf("DefaultPath() = %q, expected path under home dir %q", got, home)
	}
}

func TestResolveDataDir(t *testing.T) {
	cfg := config.Config{}
	cfg.General.DataDir = "/from-config"

	t.Run("flag wins over env and config", func(t *testing.T) {
		t.Setenv("ATB_DATA_DIR", "/from-env")
		t.Setenv("ATB_DATADIR", "")
		got := config.ResolveDataDir(cfg, "/from-flag")
		if got != "/from-flag" {
			t.Errorf("ResolveDataDir(flag=/from-flag) = %q, want /from-flag", got)
		}
	})

	t.Run("ATB_DATA_DIR wins over config when no flag", func(t *testing.T) {
		t.Setenv("ATB_DATA_DIR", "/from-env")
		t.Setenv("ATB_DATADIR", "")
		got := config.ResolveDataDir(cfg, "")
		if got != "/from-env" {
			t.Errorf("ResolveDataDir(env=/from-env) = %q, want /from-env", got)
		}
	})

	t.Run("ATB_DATADIR alias is honored", func(t *testing.T) {
		t.Setenv("ATB_DATA_DIR", "")
		t.Setenv("ATB_DATADIR", "/from-alias")
		got := config.ResolveDataDir(cfg, "")
		if got != "/from-alias" {
			t.Errorf("ResolveDataDir(alias) = %q, want /from-alias", got)
		}
	})

	t.Run("ATB_DATA_DIR wins over ATB_DATADIR alias", func(t *testing.T) {
		t.Setenv("ATB_DATA_DIR", "/canonical")
		t.Setenv("ATB_DATADIR", "/alias")
		got := config.ResolveDataDir(cfg, "")
		if got != "/canonical" {
			t.Errorf("ResolveDataDir(both) = %q, want /canonical", got)
		}
	})

	t.Run("config used when no flag or env", func(t *testing.T) {
		t.Setenv("ATB_DATA_DIR", "")
		t.Setenv("ATB_DATADIR", "")
		got := config.ResolveDataDir(cfg, "")
		if got != "/from-config" {
			t.Errorf("ResolveDataDir(config=/from-config) = %q, want /from-config", got)
		}
	})
}

func TestDefaultDataDirWithEnv(t *testing.T) {
	t.Setenv("ATB_DATA_DIR", "/shared/atb")
	if got := config.DefaultDataDir(); got != "/shared/atb" {
		t.Errorf("DefaultDataDir() with ATB_DATA_DIR=/shared/atb = %q, want /shared/atb", got)
	}
}

func TestDefaultPathWithEnv(t *testing.T) {
	t.Setenv("ATB_CONFIG", "/etc/atb/config.toml")
	if got := config.DefaultPath(); got != "/etc/atb/config.toml" {
		t.Errorf("DefaultPath() with ATB_CONFIG=/etc/atb/config.toml = %q, want /etc/atb/config.toml", got)
	}
}

func TestSaveProducesUnindentedTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Default()
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	// Standard TOML output has no leading whitespace on key lines; the
	// previous default of two-space indent confused users who saw long URL
	// values wrap in narrow terminals (issue #11).
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.HasPrefix(line, " ") {
			t.Errorf("Save produced indented line %q, want no leading spaces", line)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/some/path", filepath.Join(home, "some/path")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", home},
	}

	for _, tt := range tests {
		got, err := config.ExpandPath(tt.input)
		if err != nil {
			t.Errorf("ExpandPath(%q) returned error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
