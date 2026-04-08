package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"universal-controller/internal/config"
)

type Plan struct {
	Commands    []string `json:"commands"`
	Explanation string   `json:"explanation"`
	Warnings    []string `json:"warnings"`
	Provider    string   `json:"provider"`
	Raw         string   `json:"raw"`
	Blocked     bool     `json:"blocked"`
}

type Planner struct {
	Config config.Config
	HTTP   *http.Client
}

func (p Planner) CreatePlan(ctx context.Context, provider, task, target string) (Plan, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(p.Config.AI.Active))
	}
	var plan Plan
	var err error
	switch provider {
	case "chatgpt", "openai":
		plan, err = p.planWithOpenAI(ctx, task, target)
	case "gemini":
		plan, err = p.planWithGemini(ctx, task, target)
	case "claude", "anthropic":
		plan, err = p.planWithClaude(ctx, task, target)
	default:
		return Plan{}, fmt.Errorf("unknown AI provider %q", provider)
	}
	if err != nil {
		return Plan{}, err
	}
	plan.Provider = provider
	plan.Warnings = append(plan.Warnings, safetyWarnings(plan.Commands)...)
	if len(plan.Warnings) > 0 {
		plan.Blocked = true
	}
	return plan, nil
}

func (p Planner) planWithOpenAI(ctx context.Context, task, target string) (Plan, error) {
	cfg := p.Config.AI.OpenAI
	if cfg.UseCLI && cfg.CLICommand != "" {
		return planWithCLI(ctx, cfg.CLICommand, task)
	}
	if cfg.APIKey == "" {
		return Plan{}, errors.New("chatgpt API key is not configured")
	}
	endpoint := cfg.BaseURL
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/responses"
	}
	body := map[string]any{
		"model":             cfg.Model,
		"instructions":      systemPrompt(target),
		"input":             task,
		"max_output_tokens": 700,
	}
	requestBody, err := json.Marshal(body)
	if err != nil {
		return Plan{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return Plan{}, err
	}
	request.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	request.Header.Set("Content-Type", "application/json")
	responseText, err := p.doTextRequest(request)
	if err != nil {
		return Plan{}, err
	}
	return parsePlanResponse(responseText)
}

func (p Planner) planWithGemini(ctx context.Context, task, target string) (Plan, error) {
	cfg := p.Config.AI.Gemini
	if cfg.UseCLI && cfg.CLICommand != "" {
		return planWithCLI(ctx, cfg.CLICommand, task)
	}
	if cfg.APIKey == "" {
		return Plan{}, errors.New("gemini API key is not configured")
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	endpoint := cfg.BaseURL
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	}
	body := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": systemPrompt(target)}},
		},
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": task}},
			},
		},
	}
	requestBody, err := json.Marshal(body)
	if err != nil {
		return Plan{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return Plan{}, err
	}
	request.Header.Set("x-goog-api-key", cfg.APIKey)
	request.Header.Set("Content-Type", "application/json")
	responseText, err := p.doTextRequest(request)
	if err != nil {
		return Plan{}, err
	}
	return parsePlanResponse(responseText)
}

func (p Planner) planWithClaude(ctx context.Context, task, target string) (Plan, error) {
	cfg := p.Config.AI.Claude
	if cfg.UseCLI && cfg.CLICommand != "" {
		return planWithCLI(ctx, cfg.CLICommand, task)
	}
	if cfg.APIKey == "" {
		return Plan{}, errors.New("claude API key is not configured")
	}
	endpoint := cfg.BaseURL
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}
	body := map[string]any{
		"model":      cfg.Model,
		"max_tokens": 700,
		"system":     systemPrompt(target),
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": task,
			},
		},
	}
	requestBody, err := json.Marshal(body)
	if err != nil {
		return Plan{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return Plan{}, err
	}
	request.Header.Set("x-api-key", cfg.APIKey)
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	responseText, err := p.doTextRequest(request)
	if err != nil {
		return Plan{}, err
	}
	return parsePlanResponse(responseText)
}

func (p Planner) doTextRequest(request *http.Request) (string, error) {
	client := p.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode >= 400 {
		return "", fmt.Errorf("provider returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if text := extractOpenAIText(payload); text != "" {
		return text, nil
	}
	if text := extractGeminiText(payload); text != "" {
		return text, nil
	}
	if text := extractClaudeText(payload); text != "" {
		return text, nil
	}
	return string(payload), nil
}

func parsePlanResponse(text string) (Plan, error) {
	plan := Plan{Raw: text}
	jsonBlob := extractJSONObject(text)
	if jsonBlob == "" {
		return plan, fmt.Errorf("provider did not return a valid JSON object: %s", text)
	}
	if err := json.Unmarshal([]byte(jsonBlob), &plan); err != nil {
		return plan, err
	}
	plan.Commands = normalizeCommands(plan.Commands)
	return plan, nil
}

func extractOpenAIText(payload []byte) string {
	var response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return ""
	}
	lines := []string{}
	for _, item := range response.Output {
		for _, content := range item.Content {
			if content.Text != "" {
				lines = append(lines, content.Text)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func extractGeminiText(payload []byte) string {
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return ""
	}
	lines := []string{}
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				lines = append(lines, part.Text)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func extractClaudeText(payload []byte) string {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return ""
	}
	lines := []string{}
	for _, item := range response.Content {
		if item.Text != "" {
			lines = append(lines, item.Text)
		}
	}
	return strings.Join(lines, "\n")
}

func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

func normalizeCommands(commands []string) []string {
	normalized := make([]string, 0, len(commands))
	seen := map[string]struct{}{}
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		if _, ok := seen[command]; ok {
			continue
		}
		seen[command] = struct{}{}
		normalized = append(normalized, command)
	}
	return normalized
}

func systemPrompt(target string) string {
	return fmt.Sprintf(`You are a terminal automation planner for Universal Controller.
Target: %s
Return only JSON with this exact shape:
{"commands":["command 1","command 2"],"explanation":"short explanation","warnings":["warning 1"]}
Rules:
- Prefer safe diagnostic and idempotent commands.
- Do not use markdown.
- Keep commands directly executable.
- If the task is ambiguous, return an empty commands array and explain what is missing.`, target)
}

func safetyWarnings(commands []string) []string {
	patterns := []string{"rm -rf", "shutdown", "reboot", "mkfs", "dd if=", ":(){", "diskpart", "format "}
	warnings := []string{}
	for _, command := range commands {
		lower := strings.ToLower(command)
		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				warnings = append(warnings, fmt.Sprintf("high-risk command blocked: %s", command))
				break
			}
		}
	}
	return warnings
}

func planWithCLI(ctx context.Context, template, task string) (Plan, error) {
	commandLine := template
	if strings.Contains(commandLine, "%TASK%") {
		commandLine = strings.ReplaceAll(commandLine, "%TASK%", shellEscape(task))
	} else {
		commandLine = commandLine + " " + shellEscape(task)
	}
	cmd := exec.CommandContext(ctx, "sh", "-lc", commandLine)
	payload, err := cmd.CombinedOutput()
	if err != nil {
		return Plan{}, fmt.Errorf("AI CLI failed: %w", err)
	}
	return parsePlanResponse(string(payload))
}

func shellEscape(input string) string {
	return "'" + strings.ReplaceAll(input, "'", "'\"'\"'") + "'"
}
