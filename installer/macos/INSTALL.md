# macOS Installer

## Included

- `install.sh`
- `amd64/universal-controller`
- `arm64/universal-controller`
- packaged `tar.gz` archives for both architectures

## Install

1. Open Terminal in `installer/macos`.
2. Run:

```bash
./install.sh
```

The script:

- detects `x86_64` or `arm64`
- copies the correct binary into `~/.local/bin`
- runs `universal-controller repair`

If `~/.local/bin` is not on your PATH, add it to your shell profile.
