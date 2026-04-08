# Linux Installer

## Included

- `install.sh`
- `amd64/universal-controller`
- `amd64/universal-controller_*.tar.gz`
- `arm64/universal-controller`
- `arm64/universal-controller_*.tar.gz`

## Install from this folder

1. Open a terminal in `installer/linux`.
2. Run:

```bash
./install.sh
```

3. To install the boot-time receiver service too:

```bash
./install.sh --service
```

## What the script does

- detects CPU architecture
- copies the matching binary into `~/.local/bin`
- runs `universal-controller repair`
- optionally writes and enables the Linux `systemd` receiver service

## Notes

- `--service` needs `sudo`
- the service is designed to start before login under `multi-user.target`
