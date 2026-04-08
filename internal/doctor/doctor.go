package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"universal-controller/internal/config"
)

type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
)

type Check struct {
	Name    string
	Status  Status
	Details string
	Fix     string
}

func Run(cfg config.Config) []Check {
	checks := []Check{
		checkPath("config", must(config.ConfigPath()), false),
		checkPath("log", must(config.LogPath()), false),
		checkBinary("ssh", true),
		checkBinary("adb", true),
		checkBinary("tmux", true),
		checkBinary("docker", true),
		checkBinary("termux-info", true),
	}
	if runtime.GOOS == "linux" {
		checks = append(checks, checkBinary("systemctl", true))
	}
	if runtime.GOOS == "windows" {
		checks = append(checks, checkBinary("powershell.exe", false), checkBinary("cmd.exe", false))
	}
	if runtime.GOOS != "windows" && strings.TrimSpace(cfg.Shell.Default) == "" {
		checks = append(checks, Check{
			Name:    "shell",
			Status:  StatusWarn,
			Details: "default shell is empty",
			Fix:     "run `universal-controller repair`",
		})
	}
	if cfg.Receiver.Token == "" {
		checks = append(checks, Check{
			Name:    "receiver token",
			Status:  StatusFail,
			Details: "receiver token missing",
			Fix:     "run `universal-controller repair`",
		})
	}
	if config.ReceiverExposed(cfg.Receiver.Host) {
		checks = append(checks, Check{
			Name:    "receiver exposure",
			Status:  StatusWarn,
			Details: fmt.Sprintf("receiver binds to a non-loopback address (%s)", cfg.Receiver.Host),
			Fix:     "bind the receiver to 127.0.0.1 unless you explicitly need remote access",
		})
	}
	return checks
}

func Render(checks []Check) string {
	var builder strings.Builder
	for _, check := range checks {
		builder.WriteString(fmt.Sprintf("[%s] %s: %s", check.Status, check.Name, check.Details))
		if check.Fix != "" {
			builder.WriteString(fmt.Sprintf(" | fix: %s", check.Fix))
		}
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func HasFailures(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

func checkBinary(name string, optional bool) Check {
	if _, err := exec.LookPath(name); err != nil {
		status := StatusFail
		fix := fmt.Sprintf("install `%s` or remove the related workflow", name)
		if optional {
			status = StatusWarn
			fix = fmt.Sprintf("install `%s` to enable that integration", name)
		}
		return Check{
			Name:    name,
			Status:  status,
			Details: "not found on PATH",
			Fix:     fix,
		}
	}
	return Check{
		Name:    name,
		Status:  StatusPass,
		Details: "available",
	}
}

func checkPath(name, path string, optional bool) Check {
	if _, err := os.Stat(path); err != nil {
		status := StatusFail
		if optional {
			status = StatusWarn
		}
		return Check{
			Name:    name,
			Status:  status,
			Details: fmt.Sprintf("missing path %s", path),
			Fix:     "run `universal-controller repair`",
		}
	}
	return Check{
		Name:    name,
		Status:  StatusPass,
		Details: fmt.Sprintf("ready at %s", path),
	}
}

func must(value string, err error) string {
	if err != nil {
		return ""
	}
	return value
}
