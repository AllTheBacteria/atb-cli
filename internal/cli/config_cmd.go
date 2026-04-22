package cli

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage atb configuration",
	}

	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigGetCmd())

	return cmd
}

func resolveConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return config.DefaultPath()
}

func loadConfig() (config.Config, error) {
	return config.Load(resolveConfigPath())
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "init",
		Short:   "Create a default config file",
		Example: `  atb config init`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolveConfigPath()

			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config file already exists: %s", path)
			}

			cfg := config.Default()
			if err := config.Save(cfg, path); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Config file created at %s\n", path)
			return nil
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "Print current configuration as TOML",
		Example: `  atb config show`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			var buf bytes.Buffer
			if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), buf.String())
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "set <key> <value>",
		Short:   "Set a config value (key in dotted form, e.g. general.data_dir)",
		Example: `  atb config set general.data_dir /path/to/data`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			path := resolveConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			if err := setField(&cfg, key, value); err != nil {
				return err
			}

			if err := config.Save(cfg, path); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", key, value)
			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <key>",
		Short:   "Get a config value (key in dotted form, e.g. general.data_dir)",
		Example: `  atb config get general.data_dir`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			value, err := getField(cfg, args[0])
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}
}

// setField sets a dotted key (e.g. "general.data_dir") on cfg using struct TOML tags.
func setField(cfg *config.Config, key, value string) error {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("key must be in section.field format, got: %s", key)
	}

	section, field := parts[0], parts[1]

	rv := reflect.ValueOf(cfg).Elem()
	sectionVal, err := findFieldByTOMLTag(rv, section)
	if err != nil {
		return fmt.Errorf("unknown section %q: %w", section, err)
	}

	fieldVal, err := findFieldByTOMLTag(sectionVal, field)
	if err != nil {
		return fmt.Errorf("unknown field %q in section %q: %w", field, section, err)
	}

	return assignStringToField(fieldVal, value)
}

// getField retrieves the string representation of a dotted key from cfg.
func getField(cfg config.Config, key string) (string, error) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("key must be in section.field format, got: %s", key)
	}

	section, field := parts[0], parts[1]

	rv := reflect.ValueOf(cfg)
	sectionVal, err := findFieldByTOMLTag(rv, section)
	if err != nil {
		return "", fmt.Errorf("unknown section %q: %w", section, err)
	}

	fieldVal, err := findFieldByTOMLTag(sectionVal, field)
	if err != nil {
		return "", fmt.Errorf("unknown field %q in section %q: %w", field, section, err)
	}

	return fmt.Sprintf("%v", fieldVal.Interface()), nil
}

func findFieldByTOMLTag(v reflect.Value, tag string) (reflect.Value, error) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tomlTag := f.Tag.Get("toml")
		// strip options like omitempty
		tomlName := strings.Split(tomlTag, ",")[0]
		if tomlName == tag {
			return v.Field(i), nil
		}
	}
	return reflect.Value{}, fmt.Errorf("field with toml tag %q not found", tag)
}

func assignStringToField(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as integer: %w", s, err)
		}
		v.SetInt(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("cannot parse %q as bool: %w", s, err)
		}
		v.SetBool(b)
	default:
		return fmt.Errorf("unsupported field type: %s", v.Kind())
	}
	return nil
}
