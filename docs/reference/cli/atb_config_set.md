## atb config set

Set a config value (key in dotted form, e.g. general.data_dir)

```
atb config set <key> <value> [flags]
```

### Examples

```
  atb config set general.data_dir /path/to/data
```

### Options

```
  -h, --help   help for set
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb config](atb_config.md)	 - Manage atb configuration

