# Termux Installer

## Included

- `install.sh`
- `arm64/universal-controller`
- packaged ARM64 archive

## Install inside Termux

1. Copy the `installer/termux` folder to the Android device.
2. Open Termux.
3. Change into the folder.
4. Run:

```bash
chmod +x install.sh
./install.sh
```

The installer copies the binary into:

```text
$PREFIX/bin
```

## Notes

- This package is intended for modern ARM64 Termux environments.
- If packaged artifacts are missing, the installer installs `golang` with `pkg` and builds from source.
- If the repository root is not present, the installer clones the source into a temporary checkout automatically.
- If the `cmd/universal-controller` entrypoint is missing, the installer recreates it before building.
- Receiver mode can still be run manually inside Termux.
