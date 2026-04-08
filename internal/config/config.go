package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	AppName            = "universal-controller"
	ConfigName         = "config.json"
	DefaultHost        = "127.0.0.1"
	PublicReceiverHost = "0.0.0.0"
	DefaultPort        = 8080
	DefaultModel       = "gpt-4.1-mini"
	minimumTokenLength = 32
	windowsInstallDir  = "UniversalController"
)

type Config struct {
	Version   int                `json:"version"`
	Role      string             `json:"role"`
	Device    DeviceConfig       `json:"device"`
	UI        UIConfig           `json:"ui"`
	Shell     ShellConfig        `json:"shell"`
	Receiver  ReceiverConfig     `json:"receiver"`
	AI        AIConfig           `json:"ai"`
	Profiles  map[string]Profile `json:"profiles"`
	Plugins   PluginsConfig      `json:"plugins"`
	LastError string             `json:"last_error,omitempty"`
}

type DeviceConfig struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Termux   bool   `json:"termux"`
}

type UIConfig struct {
	Layout string `json:"layout"`
	Theme  string `json:"theme"`
}

type ShellConfig struct {
	Default string `json:"default"`
}

type ReceiverConfig struct {
	Enabled   bool   `json:"enabled"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Token     string `json:"token"`
	AutoStart bool   `json:"auto_start"`
}

type AIConfig struct {
	Active string         `json:"active"`
	OpenAI ProviderConfig `json:"openai"`
	Gemini ProviderConfig `json:"gemini"`
	Claude ProviderConfig `json:"claude"`
}

type ProviderConfig struct {
	Enabled     bool   `json:"enabled"`
	APIKey      string `json:"api_key,omitempty"`
	Model       string `json:"model"`
	BaseURL     string `json:"base_url,omitempty"`
	CLICommand  string `json:"cli_command,omitempty"`
	UseCLI      bool   `json:"use_cli,omitempty"`
	Description string `json:"description,omitempty"`
}

type Profile struct {
	Type        string `json:"type"`
	Address     string `json:"address"`
	Token       string `json:"token,omitempty"`
	Description string `json:"description,omitempty"`
}

type PluginsConfig struct {
	ADB    bool `json:"adb"`
	Docker bool `json:"docker"`
	Tmux   bool `json:"tmux"`
	Termux bool `json:"termux"`
}

func Default() Config {
	deviceName, _ := os.Hostname()
	return Config{
		Version: 1,
		Device: DeviceConfig{
			Name:     deviceName,
			Platform: runtime.GOOS,
			Termux:   strings.Contains(strings.ToLower(os.Getenv("PREFIX")), "termux"),
		},
		UI: UIConfig{
			Layout: "horizontal",
			Theme:  "midnight-terminal",
		},
		Shell: ShellConfig{
			Default: "",
		},
		Receiver: ReceiverConfig{
			Enabled:   false,
			Host:      DefaultHost,
			Port:      DefaultPort,
			Token:     newToken(),
			AutoStart: false,
		},
		AI: AIConfig{
			Active: "chatgpt",
			OpenAI: ProviderConfig{
				Enabled:     true,
				Model:       DefaultModel,
				Description: "ChatGPT via the OpenAI Responses API",
			},
			Gemini: ProviderConfig{
				Enabled:     true,
				Model:       "gemini-2.5-flash",
				Description: "Gemini via the Google Gemini generateContent API or Gemini CLI",
			},
			Claude: ProviderConfig{
				Enabled:     true,
				Model:       "claude-sonnet-4-20250514",
				Description: "Claude via the Anthropic Messages API",
			},
		},
		Profiles: map[string]Profile{},
		Plugins: PluginsConfig{
			ADB:    true,
			Docker: true,
			Tmux:   true,
			Termux: true,
		},
	}
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

func StateDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

func BinDir() (string, error) {
	if runtime.GOOS == "windows" {
		if value := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); value != "" {
			return filepath.Join(value, "Programs", windowsInstallDir), nil
		}
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, windowsInstallDir), nil
	}
	if value := strings.TrimSpace(os.Getenv("PREFIX")); strings.Contains(strings.ToLower(value), "termux") {
		return filepath.Join(value, "bin"), nil
	}
	base, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, ".local", "bin"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigName), nil
}

func LogPath() (string, error) {
	stateDir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "universal-controller.log"), nil
}

func EnsureDirs() error {
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}
	stateDir, err := StateDir()
	if err != nil {
		return err
	}
	binDir, err := BinDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	return nil
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	Normalize(&cfg)
	return cfg, nil
}

func LoadOrCreate() (Config, bool, error) {
	if err := EnsureDirs(); err != nil {
		return Config{}, false, err
	}
	path, err := ConfigPath()
	if err != nil {
		return Config{}, false, err
	}
	if _, err := os.Stat(path); err == nil {
		cfg, loadErr := Load()
		if loadErr == nil {
			return cfg, false, nil
		}
		brokenPath := path + ".broken"
		_ = os.Rename(path, brokenPath)
		cfg = Default()
		cfg.LastError = loadErr.Error()
		if err := Save(cfg); err != nil {
			return Config{}, false, err
		}
		return cfg, true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, false, err
	}
	cfg := Default()
	if err := Save(cfg); err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

func Save(cfg Config) error {
	Normalize(&cfg)
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Normalize(cfg *Config) []string {
	notes := []string{}
	if cfg.Version == 0 {
		cfg.Version = 1
		notes = append(notes, "version reset to 1")
	}
	// Repair invalid or empty role - require explicit valid role selection
	switch cfg.Role {
	case "self-use", "controller", "receiver":
		// role is valid
	default:
		cfg.Role = ""
		notes = append(notes, "role repaired")
	}
	switch cfg.UI.Layout {
	case "horizontal", "vertical":
	default:
		cfg.UI.Layout = "horizontal"
		notes = append(notes, "layout repaired")
	}
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = "midnight-terminal"
	}
	if cfg.Shell.Default == "" {
		cfg.Shell.Default = DetectShell()
	}
	if cfg.Device.Platform == "" {
		cfg.Device.Platform = runtime.GOOS
	}
	if cfg.Device.Name == "" {
		name, _ := os.Hostname()
		cfg.Device.Name = name
	}
	cfg.Receiver.Host = strings.TrimSpace(cfg.Receiver.Host)
	if cfg.Receiver.Host == "" {
		cfg.Receiver.Host = DefaultHost
		notes = append(notes, "receiver host repaired")
	}
	if cfg.Receiver.Port <= 0 || cfg.Receiver.Port > 65535 {
		cfg.Receiver.Port = DefaultPort
		notes = append(notes, "receiver port repaired")
	}
	cfg.Receiver.Token = strings.TrimSpace(cfg.Receiver.Token)
	if weakToken(cfg.Receiver.Token) {
		cfg.Receiver.Token = newToken()
		notes = append(notes, "receiver token regenerated")
	}
	if cfg.AI.Active == "" {
		cfg.AI.Active = "chatgpt"
	}
	if cfg.AI.OpenAI.Model == "" {
		cfg.AI.OpenAI.Model = DefaultModel
	}
	if cfg.AI.Gemini.Model == "" {
		cfg.AI.Gemini.Model = "gemini-2.5-flash"
	}
	if cfg.AI.Claude.Model == "" {
		cfg.AI.Claude.Model = "claude-sonnet-4-20250514"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return notes
}

func DetectShell() string {
	if runtime.GOOS == "windows" {
		if value := os.Getenv("COMSPEC"); value != "" {
			return value
		}
		return "powershell.exe"
	}
	if value := os.Getenv("SHELL"); value != "" {
		return value
	}
	for _, candidate := range []string{"/bin/bash", "/bin/sh", "/usr/bin/bash"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "/bin/sh"
}

func ReceiverAddress(cfg Config) string {
	host := strings.TrimSpace(cfg.Receiver.Host)
	if host == "" || ReceiverExposed(host) {
		host = DefaultHost
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.Receiver.Port)
}

func newToken() string {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "repair-me-token"
	}
	return hex.EncodeToString(raw)
}

func ReceiverExposed(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	if host == PublicReceiverHost || host == "::" {
		return true
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return !parsed.IsLoopback()
	}
	return true
}

func weakToken(token string) bool {
	token = strings.TrimSpace(token)
	return token == "" || token == "repair-me-token" || len(token) < minimumTokenLength
}
