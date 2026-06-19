## atb update

Update atb to the latest version

```
atb update [flags]
```

### Examples

```
  # Check for updates and install interactively
  atb update

  # Update without confirmation (for scripts)
  atb update --force
```

### Options

```
      --force   update without confirmation prompt
  -h, --help    help for update
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

