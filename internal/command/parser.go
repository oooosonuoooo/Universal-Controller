package command

import "strings"

const (
	KindEmpty    = "empty"
	KindAI       = "ai"
	KindLayout   = "layout"
	KindMode     = "mode"
	KindConnect  = "connect"
	KindSettings = "settings"
	KindHelp     = "help"
	KindDoctor   = "doctor"
	KindReceiver = "receiver"
	KindShell    = "shell"
)

type Parsed struct {
	Kind     string
	Provider string
	Args     []string
	Raw      string
}

func Parse(input string) Parsed {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return Parsed{Kind: KindEmpty}
	}
	if strings.HasPrefix(raw, ":") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, ":"))
	}
	if raw == "" {
		return Parsed{Kind: KindEmpty}
	}
	parts := strings.Fields(raw)
	first := strings.ToLower(parts[0])
	switch first {
	case "chatgpt", "gemini", "claude":
		return Parsed{Kind: KindAI, Provider: first, Args: parts[1:], Raw: raw}
	case "layout":
		return Parsed{Kind: KindLayout, Args: parts[1:], Raw: raw}
	case "mode":
		return Parsed{Kind: KindMode, Args: parts[1:], Raw: raw}
	case "connect":
		return Parsed{Kind: KindConnect, Args: parts[1:], Raw: raw}
	case "settings":
		return Parsed{Kind: KindSettings, Raw: raw}
	case "help":
		return Parsed{Kind: KindHelp, Raw: raw}
	case "doctor":
		return Parsed{Kind: KindDoctor, Raw: raw}
	case "receiver":
		return Parsed{Kind: KindReceiver, Args: parts[1:], Raw: raw}
	default:
		return Parsed{Kind: KindShell, Args: parts, Raw: raw}
	}
}
