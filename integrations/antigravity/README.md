# Antigravity Integration

Universal Controller exposes IDE-safe JSON commands through the `ide` namespace.

Recommended calls:

- `universal-controller ide status`
- `universal-controller ide health`
- `universal-controller ide doctor`
- `universal-controller ide tools --all --limit 25`
- `universal-controller ide exec --tool git -- status --short`

Use those commands anywhere Antigravity supports:

- custom actions
- command palette wrappers
- task runners
- integrated terminal shortcuts

The command output is plain JSON, so it can be parsed by editor plugins or automation hooks without scraping terminal colors.
