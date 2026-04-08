# Windows Installer

## Included

- `install.ps1`
- `amd64/universal-controller.exe`
- packaged ZIP archive

## Install

1. Open PowerShell inside `installer/windows`.
2. If execution policy blocks scripts for this session, run:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
```

3. Run:

```powershell
.\install.ps1
```

The installer copies the app into:

```text
%LOCALAPPDATA%\Programs\UniversalController
```

and adds that directory to the user PATH if needed.
