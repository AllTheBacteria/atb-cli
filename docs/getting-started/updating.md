# Updating

`atb` checks for new versions in the background (once every 24 hours). If a newer release is found, you'll see a notice on every run until you upgrade:

```
  A new version of atb is available: v0.9.0 (current: v0.8.0)

  What's new:
    feat: find closest ATB genomes via sketch distances
    ...

  Release: https://github.com/allthebacteria/atb-cli/releases/tag/v0.9.0

  Run 'atb update' to upgrade.
```

To update:

```bash
# Interactive update (asks for confirmation)
atb update

# Non-interactive (for scripts/CI)
atb update --force
```

The updater downloads the correct binary for your OS/architecture from GitHub Releases and replaces the current binary in place.
