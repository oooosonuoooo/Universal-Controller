# Installer Layout

This folder contains packaged installers and install instructions organized by target platform.

## Folders

- `linux/`
- `macos/`
- `windows/`
- `termux/`

Each platform folder contains:

- install instructions
- a platform-specific installer script
- one or more architecture-specific binaries
- packaged archives

## Verify downloads

Run this from inside the `installer/` folder:

```bash
sha256sum -c checksums.sha256
```

## Rebuild installers

From the repository root:

```bash
./scripts/package_installers.sh
```

This runs tests, cross-builds the binaries, creates archives, and refreshes `checksums.sha256`.
