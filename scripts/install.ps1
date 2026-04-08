$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$Installer = Join-Path $RootDir "installer\windows\install.ps1"

& $Installer @args
