# Universal Controller

Universal Controller is a keyboard-first control surface for local machines, remote receiver agents, IDE task runners, and AI-assisted terminal task execution.

For packaged installers, start with [START-HERE.md](START-HERE.md).

This repository currently ships a working foundation with:

- a full-screen terminal UI built in Go
- three runtime modes: `self-use`, `controller`, and `receiver`
- authenticated receiver execution over HTTP
- local execution on the same machine
- AI provider configuration for ChatGPT, Gemini, and Claude
- IDE-safe JSON commands for VS Code, Antigravity, and any editor that can run shell tasks
- tool discovery for installed utilities like `nmap`, `yt-dlp`, `docker`, `adb`, `tmux`, `terraform`, and more
- self-repair/bootstrap flow and install scripts

## Why this exists

The goal is to make one interface that can:

- control the same device locally
- control a remote device through a receiver service
- work from a terminal UI without requiring mouse usage
- expose machine-readable hooks for IDE tasks and developer tooling
- route plain-language tasks through AI providers and then execute the resulting commands safely

## Modes

- `self-use`: local smart terminal mode for the same device
- `controller`: remote-first mode with no automatic local target
- `receiver`: background-capable agent mode for being controlled by another machine

## Quick start

Build:

```bash
go build -o build/universal-controller ./cmd/universal-controller
```

Run the UI:

```bash
./build/universal-controller
```

Run the receiver:

```bash
./build/universal-controller receiver start
```

Check environment health:

```bash
./build/universal-controller doctor
```

Inspect IDE status:

```bash
./build/universal-controller ide status
```

## Beginner UI Workflow (No Linux Experience Required)

Run the app:

```bash
./build/universal-controller
```

Then follow this exact order inside the UI:

1. `Start Here (Guided)`  
   Read the in-app tutorial once. It explains keyboard controls, copy/paste basics, and command safety.
2. `Choose Device Mode`  
   Pick `self-use` if you are using only this machine.
3. `Use This Device`  
   Makes local shell execution the active target.
4. `Run Command`  
   Start with safe commands:
   - `pwd`
   - `ls -la`
   - `whoami`
5. `Check Device Health`  
   Shows CPU, memory, disk and uptime.
6. `Ask AI To Do A Task`  
   Example prompt: `Check system health and summarize issues in simple English.`
7. `Open Logs`  
   Review what happened in each operation.

### How Commands Work In The App

When you use `Run Command`, the app does this:

1. Reads your exact command text
2. Resolves current target (local or receiver)
3. Builds request with command, mode (`normal`/`root`), timeout
4. Executes using local shell or receiver API
5. Captures stdout, stderr, exit code, duration
6. Prints a full report in the activity log

### How AI Task Works In The App

When you use `Ask AI To Do A Task`, the app:

1. Sends your plain-language goal to selected AI provider
2. Receives explanation, warnings, and proposed commands
3. Executes commands in order on selected target
4. Stores output and errors in logs for review

### Safe nmap Usage (Authorized Systems Only)

Use `nmap` only on systems/networks you own or are explicitly authorized to test.

Examples:

```bash
# Host discovery in your own subnet
nmap -sn 192.168.1.0/24

# Service/version scan on an approved host
nmap -sV 192.168.1.10

# Local machine scan
nmap -sV 127.0.0.1
```

In the UI:

1. Open `Run Command`
2. Paste the `nmap` command
3. Keep mode as `normal`
4. Run and review output + exit code in logs

## Install From Source

Linux and macOS:

```bash
./scripts/install.sh
```

Windows PowerShell:

```powershell
.\scripts\install.ps1
```

Linux receiver service:

```bash
./scripts/install.sh --service
```

The installer is designed to self-repair missing Go module state and regenerate defaults during installation.

## IDE integration

Universal Controller exposes a dedicated `ide` namespace so editors can call it without scraping a TUI:

- `universal-controller ide status`
- `universal-controller ide health`
- `universal-controller ide doctor`
- `universal-controller ide tools --all --limit 25`
- `universal-controller ide exec --tool git -- status --short`

Included templates:

- [VS Code tasks example](integrations/vscode/tasks.example.json)
- [Antigravity integration notes](integrations/antigravity/README.md)

## Tool support

Universal Controller does not hardcode a small fixed tool list. It provides:

- a curated catalog for common developer, DevOps, media, remote, and mobile tools
- PATH discovery so any installed executable can appear in the catalog
- IDE commands that can run discovered tools and return JSON

Examples from the current curated catalog:

| Tool | Description | Link |
|------|-------------|------|
| `nmap` | Network scanning and service enumeration | https://nmap.org |
| `yt-dlp` | Media retrieval from hundreds of sites | https://github.com/yt-dlp/yt-dlp |
| `docker` | Container lifecycle and image workflows | https://www.docker.com |
| `adb` | Android Debug Bridge | https://developer.android.com/tools/adb |
| `tmux` | Terminal multiplexer | https://github.com/tmux/tmux/wiki |
| `terraform` | Infrastructure as code | https://www.terraform.io |
| `kubectl` | Kubernetes control | https://kubernetes.io/docs/reference/kubectl/ |
| `ffmpeg` | Media processing and transcoding | https://ffmpeg.org |

Use these only in environments where you are authorized to run them.

## Verification performed in this workspace

The current build was verified by:

- `go test ./...`
- `go build -o build/universal-controller ./cmd/universal-controller`
- `./build/universal-controller repair`
- `./build/universal-controller doctor`
- `./build/universal-controller ide status`
- `./build/universal-controller ide health`
- `./build/universal-controller tools --all --limit 25`
- starting `receiver start` and executing a remote smoke test through `exec --receiver`
- launching the TUI and exiting it cleanly

## Project layout

```text
cmd/universal-controller/    CLI entrypoint
internal/ai/                 AI provider planning
internal/bootstrap/          self-repair and startup defaults
internal/cli/                command wiring
internal/command/            command parsing helpers
internal/config/             config model and persistence
internal/doctor/             dependency checks
internal/executor/           local and receiver executors
internal/health/             machine health collection
internal/receiver/           receiver HTTP service
internal/tools/              tool catalog and discovery
internal/tui/                keyboard-only UI
integrations/                IDE integration templates
scripts/                     installer and release automation
```

## Documentation

For full setup, architecture, flows, modes, use cases, auto-start behavior, AI setup, and IDE integration, read [instructions.md](instructions.md).
