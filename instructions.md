# Universal Controller Instructions

## 1. What this app is

Universal Controller is a keyboard-driven control platform for:

- running commands on the same device
- running commands on remote devices through a receiver agent
- orchestrating installed tools through a unified catalog
- exposing machine-readable commands to IDEs
- optionally asking ChatGPT, Gemini, or Claude to build a command plan and execute it

If you are looking for installers first, use [START-HERE.md](START-HERE.md).

The current implementation is a working production-style foundation focused on the features that are actually shipped in this repository.

## 2. Current implementation status

### Implemented now

- full-screen TUI with keyboard navigation
- first-run mode selection
- `self-use`, `controller`, and `receiver` modes
- local command execution
- authenticated receiver server
- remote execution through receiver
- health collection
- environment doctor and repair
- AI provider configuration
- IDE JSON bridge
- tool catalog and PATH discovery
- Linux boot service template for receiver mode
- self-repairing installers

The rest of this document describes only the implemented features.

## 3. Core modes

### Self-use mode

Purpose:

- treat the same device as the active target
- use the TUI as a local smart terminal surface
- run local health checks, AI plans, and discovered tools

This is the mode for controlling the same machine from the app itself.

### Controller mode

Purpose:

- keep the UI open without assuming local execution
- require the user to connect a receiver before running tasks

Use this when the current machine is mainly acting as the operator console.

### Receiver mode

Purpose:

- let the machine act as a controlled endpoint
- optionally run as a boot-time service on Linux
- accept authenticated execution requests from another device

Use this on servers, office machines, lab boxes, remote Linux devices, and other always-on targets.

## 4. First-run flow

When you launch the app for the first time:

1. It creates the config directory if it does not exist.
2. It creates a config file with safe defaults.
3. It generates a receiver token automatically.
4. It creates a log file.
5. It opens a first-run mode picker.

The first-run mode picker lets you choose:

- Controller
- Receiver
- Self Use

You move with:

- `Up`
- `Down`
- `J`
- `K`

You confirm with:

- `Enter`

## 5. Keyboard navigation inside the UI

The UI is built so you do not need to type shell-style commands to operate it.

Main keys:

- `Up` / `Down`: move through actions
- `J` / `K`: move through actions
- `Enter`: run the selected action
- `Tab`: move to the next screen
- `Shift+Tab`: move to the previous screen
- `Esc`: return to the dashboard or close a dialog
- `Ctrl+H`: force horizontal layout
- `Ctrl+V`: force vertical layout
- `Q` or `Ctrl+C`: quit

Dialog keys:

- `Tab`: move to the next field
- `Shift+Tab`: move to the previous field
- `Left` / `Right`: change option fields like provider selectors
- `Enter` on the last field: submit
- `Esc`: cancel dialog

## 6. TUI screens

### Dashboard

The dashboard is the main control surface.

Actions currently available:

- Use Self Mode
- Use Controller Mode
- Use Receiver Mode
- Select This Device
- Connect Remote Receiver
- Disconnect Current Receiver
- Start Or Stop Embedded Receiver
- Run Device Health Check
- Ask AI Assistant
- Show Tool Catalog
- Toggle Horizontal Or Vertical Layout
- Toggle Root Access
- Open Settings
- Open Logs
- Open Help
- Quit

### Settings

The settings screen lets you:

- choose the default AI provider
- store OpenAI credentials
- store Gemini credentials or Gemini CLI command
- store Claude credentials
- toggle the receiver auto-start intent flag
- run the environment doctor

### Logs

The log screen shows recent UI actions and task results.

### Help

The help screen explains keyboard usage and current module boundaries.

## 7. How self-use mode works step by step

This is the same-device flow.

1. Launch `universal-controller`.
2. Choose `Self Use` on first run if you have not configured it yet.
3. On later runs, choose the `Use Self Mode` dashboard action if needed.
4. Choose `Select This Device`.
5. Choose `Run Device Health Check` to verify the machine state.
6. Choose `Ask AI Assistant` to describe a task in plain language.
7. If you need elevated execution, choose `Toggle Root Access` and enter your password once.
8. The app will keep that root password only for the current session in memory.

Example self-use actions:

- health inspection
- package diagnostics
- git inspection
- docker tasks
- tmux sessions
- Terraform plan or state inspection
- ADB usage on the same workstation

## 8. How receiver mode works step by step

Receiver mode can be used in two ways.

### A. Embedded receiver from inside the UI

1. Open the app.
2. Select or switch to `Receiver Mode`.
3. Use `Start Or Stop Embedded Receiver`.
4. The UI process opens the listener and shows the host, port, and token.
5. Another device can connect with the receiver URL and token.

This is useful for quick sessions and testing.

### B. Background receiver service on Linux

1. Build or install the app.
2. Run:

```bash
./build/universal-controller receiver install-service
```

3. Enable it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now universal-controller-receiver.service
```

4. The receiver will then start at boot under `multi-user.target`, before user login.

This is the recommended path for Linux devices that should act as always-on receiver endpoints.

## 9. How controller mode works step by step

1. Open the app.
2. Switch to `Controller Mode`.
3. Choose `Connect Remote Receiver`.
4. Enter the receiver URL.
5. Enter the receiver token.
6. The app verifies receiver health.
7. The active target becomes the remote receiver.
8. Now run health checks, tool commands, or AI-assisted tasks against that receiver.

## 10. Remote smoke-tested flow that works now

The following pattern was tested in this workspace:

1. Start receiver:

```bash
./build/universal-controller receiver start
```

2. In a second terminal, execute against it:

```bash
./build/universal-controller exec --receiver http://127.0.0.1:8080 --token <token> "echo receiver-smoke-test"
```

3. The result comes back as JSON and includes stdout, stderr, exit code, and timing.

## 11. AI configuration

The UI supports three providers:

- ChatGPT
- Gemini
- Claude

### ChatGPT

Open Settings:

1. Choose `ChatGPT Settings`
2. Enter the API key
3. Enter the model or keep the default
4. Optionally override the base URL
5. Save

### Gemini

Open Settings:

1. Choose `Gemini Settings`
2. Enter the Gemini API key
3. Enter the model
4. Optionally enter a Gemini CLI command
5. Save

If the Gemini CLI command is configured, the app can route planning through that external CLI instead of the hosted API.

### Claude

Open Settings:

1. Choose `Claude Settings`
2. Enter the Anthropic API key
3. Enter the model
4. Optionally override the base URL
5. Save

## 12. How AI task execution works

When you choose `Ask AI Assistant`:

1. The app asks for provider and task text.
2. It sends a planning prompt to the selected provider.
3. The provider must return strict JSON with:
   - commands
   - explanation
   - warnings
4. The app validates the plan.
5. High-risk commands are blocked automatically.
6. If the plan is accepted, the commands execute on the active target.
7. Output is written into the activity log.

Examples of tasks you can ask for:

- "check server health"
- "inspect docker containers and summarize problems"
- "show me why disk usage is high"
- "inspect Terraform state related files in this repo"

Use only tasks you are authorized to perform on the target system.

## 13. Tool catalog

Universal Controller supports any tool installed on the machine through two mechanisms:

- a curated catalog for commonly used tools
- automatic discovery of every executable already on `PATH`

Discovered tools are exposed through CLI and IDE integrations and can be targeted via `ide exec`.

The app does **not** bundle or download tools itself. It makes existing tools on the machine accessible through a unified interface.

### Current curated tools

The shipped catalog includes common tools such as:

- `adb`
- `docker`
- `ffmpeg`
- `git`
- `jq`
- `kubectl`
- `nmap`
- `pwsh`
- `python3`
- `ssh`
- `terraform`
- `tmux`
- `uv`
- `yt-dlp`

### Listing tools

CLI:

```bash
./build/universal-controller tools --all --limit 25
```

IDE JSON:

```bash
./build/universal-controller ide tools --all --limit 25
```

## 14. IDE integration

The `ide` namespace is designed so editors can call Universal Controller without requiring a full-screen TUI.

### Supported IDE-style entrypoints

- `ide status`
- `ide doctor`
- `ide health`
- `ide tools`
- `ide exec`

### Why this matters

Editors usually need:

- JSON output
- commands that can run without a full-screen TUI
- task-runner compatibility

The `ide` namespace provides exactly that.

### VS Code

There is a ready example at:

- [integrations/vscode/tasks.example.json](integrations/vscode/tasks.example.json)

Copy the tasks you want into `.vscode/tasks.json` in your own project.

### Antigravity

There is a note file at:

- [integrations/antigravity/README.md](integrations/antigravity/README.md)

Use the `ide` commands in:

- command wrappers
- integrated terminal actions
- task runners
- editor commands that accept external processes

## 15. Installation

### Linux and macOS

Run:

```bash
./scripts/install.sh
```

What the script does:

1. Detects whether Go is installed
2. Installs Go if possible through a package manager
3. Runs `go mod tidy`
4. Repairs module state if needed
5. Builds the binary
6. Installs it into `~/.local/bin` by default
7. Runs `universal-controller repair`

If you also want the Linux receiver service:

```bash
./scripts/install.sh --service
```

### Windows

Run in PowerShell:

```powershell
.\scripts\install.ps1
```

What it does:

1. Ensures Go exists
2. Repairs module state if required
3. Builds `universal-controller.exe`
4. Copies it to the install directory
5. Runs `repair`

## 16. Self-repair behavior

The app is designed to heal the following problems automatically:

- missing config directory
- missing state directory
- missing log file
- corrupt or missing config file
- missing receiver token
- invalid layout value
- invalid or empty shell value

The installer also attempts to self-heal:

- broken `go.sum`
- stale Go module cache
- failed first build caused by module state

Repair entrypoint:

```bash
./build/universal-controller repair
```

Doctor entrypoint:

```bash
./build/universal-controller doctor
```

## 17. Security model

### Root mode

Root access is not permanent.

Current behavior:

- normal mode does not require a password
- root mode prompts once
- the password stays in memory only for the session
- local root execution uses `sudo`

### Receiver authentication

Receiver access uses:

- a bearer token
- per-request verification

### Operational caution

Do not use the tool against systems you do not own or administer.
Do not run network or system tools without authorization and policy approval.

## 18. File paths and persistence

Config file:

```text
~/.config/universal-controller/config.json
```

State and logs:

```text
~/.local/state/universal-controller/
```

Installed binary by default:

```text
~/.local/bin/universal-controller
```

## 19. CLI commands

### Main UI

```bash
universal-controller
universal-controller tui
```

### Repair and health

```bash
universal-controller repair
universal-controller doctor
```

### Receiver

```bash
universal-controller receiver start
universal-controller receiver install-service
```

### Direct execution

```bash
universal-controller exec "pwd"
universal-controller exec --receiver http://127.0.0.1:8080 --token <token> "echo hello"
```

### Config

```bash
universal-controller config show
universal-controller config set-role self-use
universal-controller config set-ai-key --provider chatgpt <key>
```

### Tool catalog

```bash
universal-controller tools
universal-controller tools --all --limit 50
```

### IDE bridge

```bash
universal-controller ide status
universal-controller ide doctor
universal-controller ide health
universal-controller ide tools --all --limit 25
universal-controller ide exec --tool git -- status --short
```

## 20. Verified build commands

These commands were executed successfully and confirm the build is working:

- `go test ./...`
- `go build -o build/universal-controller ./cmd/universal-controller`
- `./build/universal-controller repair`
- `./build/universal-controller doctor`
- `./build/universal-controller ide status`
- `./build/universal-controller ide health`
- `./build/universal-controller ide tools --all --limit 10`
- `./build/universal-controller ide exec --tool git -- status --short`
- `./build/universal-controller receiver start`
- `./build/universal-controller exec --receiver http://127.0.0.1:8080 --token <token> "echo receiver-smoke-test"`
- launching the TUI and exiting it cleanly
