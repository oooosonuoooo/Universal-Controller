$ErrorActionPreference = "Stop"

$ScriptPath = $MyInvocation.MyCommand.Path
while ((Get-Item $ScriptPath).LinkType) {
    $ScriptPath = (Get-Item $ScriptPath).Target
}
$BaseDir = Split-Path -Parent $ScriptPath
$RootDir = Split-Path -Parent (Split-Path -Parent $BaseDir)
$InstallDir = if ($env:UC_INSTALL_DIR) {
    $env:UC_INSTALL_DIR
} elseif ($env:INSTALL_DIR) {
    $env:INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA "Programs\UniversalController"
}
$BinaryTarget = Join-Path $InstallDir "universal-controller.exe"

function Write-Log {
    param([string]$Message)
    Write-Host "[uc-install] $Message"
}

function Ensure-Go {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        return $true
    }
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Log "Go missing. Installing with winget."
        winget install -e --id GoLang.Go
        
        $MachinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
        $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $env:Path = "$MachinePath;$UserPath"
        
        if (Get-Command go -ErrorAction SilentlyContinue) {
            return $true
        }
    }
    return $false
}

function Ensure-CommandEntrypoint {
    param([string]$SourceRoot)
    $EntrypointDir = Join-Path $SourceRoot "cmd\universal-controller"
    $EntrypointFile = Join-Path $EntrypointDir "main.go"
    if (Test-Path $EntrypointFile) {
        return
    }
    Write-Log "Restoring missing command entrypoint"
    New-Item -ItemType Directory -Force -Path $EntrypointDir | Out-Null
    $MainContent = @'
package main

import (
	"fmt"
	"os"

	"universal-controller/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
'@
    Set-Content -Path $EntrypointFile -Value $MainContent -Encoding UTF8
}

function Build-FromSource {
    Write-Log "Attempting to build from source..."
    if (-not (Ensure-Go)) {
        throw "Go is required to build from source but could not be installed. Please install Go manually."
    }

    if (-not (Test-Path (Join-Path $RootDir "go.mod"))) {
        throw "Cannot find source code root directory (expected at $RootDir)."
    }

    Push-Location $RootDir
    try {
        Ensure-CommandEntrypoint -SourceRoot $RootDir

        function Repair-Modules {
            Write-Log "Repairing Go module cache"
            & go clean -cache -modcache
            & go mod download
        }

        try {
            & go mod download
        }
        catch {
            Repair-Modules
        }

        $BuildDir = Join-Path $RootDir "build"
        New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null
        $BinaryPath = Join-Path $BuildDir "universal-controller.exe"
        $env:CGO_ENABLED = "0"

        try {
            & go build -o "$BinaryPath" ./cmd/universal-controller
        }
        catch {
            Write-Log "Retrying build..."
            Repair-Modules
            try {
                & go build -o "$BinaryPath" ./cmd/universal-controller
            }
            catch {
                throw "Build failed after retry: $_"
            }
        }

        if (-not (Test-Path $BinaryPath)) {
            throw "Build failed, executable not found at $BinaryPath"
        }

        return $BinaryPath
    }
    finally {
        Pop-Location
    }
}

function Find-BinarySource {
    $direct = Join-Path $BaseDir "amd64\universal-controller.exe"
    if (Test-Path $direct) {
        return $direct
    }

    $archive = Get-ChildItem -Path (Join-Path $BaseDir "amd64") -Filter "universal-controller_*.zip" -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($archive) {
        $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("uc-install-" + [System.Guid]::NewGuid().ToString("N"))
        New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
        Expand-Archive -Path $archive.FullName -DestinationPath $tempDir -Force
        $extracted = Join-Path $tempDir "universal-controller.exe"
        if (Test-Path $extracted) {
            return $extracted
        }
    }

    return Build-FromSource
}

$BinarySource = Find-BinarySource

Write-Log "Installing executable to $InstallDir"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item $BinarySource $BinaryTarget -Force
& $BinaryTarget repair | Out-Null

if ($env:UC_SKIP_PATH_UPDATE -ne "1") {
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        $env:Path = "$env:Path;$InstallDir"
    }
}

Write-Host "============================================="
Write-Host "Success! Universal Controller installed to:"
Write-Host "  $BinaryTarget"
Write-Host ""
Write-Host "PLEASE RESTART YOUR TERMINAL for the PATH changes to take effect."
Write-Host "After restarting, you can run the app using the command:"
Write-Host "  universal-controller"
Write-Host "============================================="
