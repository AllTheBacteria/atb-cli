package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for atb.
type Config struct {
	General  GeneralConfig  `toml:"general"`
	Fetch    FetchConfig    `toml:"fetch"`
	Download DownloadConfig `toml:"download"`
}

// GeneralConfig holds general application settings.
type GeneralConfig struct {
	DataDir       string `toml:"data_dir"`
	DefaultFormat string `toml:"default_format"`
}

// FetchConfig holds settings for metadata fetching.
type FetchConfig struct {
	BaseURL  string `toml:"base_url"`
	Parallel int    `toml:"parallel"`
}

// DownloadConfig holds settings for genome downloads.
type DownloadConfig struct {
	Parallel       int    `toml:"parallel"`
	OutputDir      string `toml:"output_dir"`
	CheckDiskSpace bool   `toml:"check_disk_space"`
	MinFreeSpaceGB int    `toml:"min_free_space_gb"`
}

// DefaultDataDir returns the OS-standard data directory for ATB.
// If ATB_DATA_DIR (or its compact alias ATB_DATADIR) is set, it takes
// precedence on every platform — useful for shared installations where each
// user (or a system-wide profile) wants to point at a common data volume
// without editing per-user config files.
// On Windows: %LOCALAPPDATA%\atb\data
// On macOS: ~/Library/Application Support/atb/data
// On Linux/other: $XDG_DATA_HOME/atb/data or ~/.local/share/atb/data
func DefaultDataDir() string {
	if env := dataDirEnv(); env != "" {
		return env
	}
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			return filepath.Join(appdata, "atb", "data")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "atb", "data")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "atb", "data")
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "atb", "data")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "atb", "data")
	}
}

// ResolveDataDir returns the effective data directory using this precedence:
//  1. flagValue (--data-dir CLI flag) if non-empty
//  2. ATB_DATA_DIR environment variable (alias: ATB_DATADIR)
//  3. cfg.General.DataDir from the loaded config
//
// Use this everywhere a command needs the data directory so the env var works
// in shared-server setups without forcing each user to maintain their own
// config file.
func ResolveDataDir(cfg Config, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := dataDirEnv(); env != "" {
		return env
	}
	return cfg.General.DataDir
}

// dataDirEnv returns the data-dir override from the environment.
// ATB_DATA_DIR is the canonical name (matches ATB_INSTALL_DIR / docker-entrypoint.sh);
// ATB_DATADIR is accepted as an alias to match what users naturally type.
func dataDirEnv() string {
	if v := os.Getenv("ATB_DATA_DIR"); v != "" {
		return v
	}
	return os.Getenv("ATB_DATADIR")
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		General: GeneralConfig{
			DataDir:       DefaultDataDir(),
			DefaultFormat: "tsv",
		},
		Fetch: FetchConfig{
			BaseURL:  "https://ftp.ebi.ac.uk/pub/databases/AllTheBacteria",
			Parallel: 4,
		},
		Download: DownloadConfig{
			Parallel:       4,
			OutputDir:      filepath.Join(home, "atb", "genomes"),
			CheckDiskSpace: true,
			MinFreeSpaceGB: 10,
		},
	}
}

// DefaultPath returns the default config file path.
// If ATB_CONFIG is set, it takes precedence — letting a shared-server admin
// point all users at a single config file via /etc/profile.d.
// On Windows: %LOCALAPPDATA%\atb\config.toml
// On macOS: ~/Library/Application Support/atb/config.toml
// On Linux/other: $XDG_CONFIG_HOME/atb/config.toml or ~/.config/atb/config.toml
func DefaultPath() string {
	if env := os.Getenv("ATB_CONFIG"); env != "" {
		return env
	}
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			return filepath.Join(appdata, "atb", "config.toml")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "atb", "config.toml")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "atb", "config.toml")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "atb", "config.toml")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "atb", "config.toml")
	}
}

// Load reads a TOML config from path. If the file does not exist, it returns
// the default config without error.
func Load(path string) (Config, error) {
	cfg := Default()

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	return cfg, nil
}

// Save writes cfg to path as TOML, creating parent directories as needed.
func Save(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	enc.Indent = ""
	return enc.Encode(cfg)
}

// ExpandPath expands a leading ~ to the user's home directory.
func ExpandPath(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}
