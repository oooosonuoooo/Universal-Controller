package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"universal-controller/internal/config"
)

type Report struct {
	Created  bool
	Repaired bool
	Notes    []string
}

func Ensure() (config.Config, Report, error) {
	cfg, created, err := config.LoadOrCreate()
	if err != nil {
		return config.Config{}, Report{}, err
	}
	report := Report{Created: created}
	report.Notes = append(report.Notes, config.Normalize(&cfg)...)
	if err := config.EnsureDirs(); err != nil {
		return config.Config{}, report, err
	}
	logPath, err := config.LogPath()
	if err != nil {
		return config.Config{}, report, err
	}
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		if err := os.WriteFile(logPath, []byte(""), 0o600); err != nil {
			return config.Config{}, report, err
		}
		report.Notes = append(report.Notes, "log file created")
	}
	if len(report.Notes) > 0 {
		report.Repaired = true
	}
	if err := config.Save(cfg); err != nil {
		return config.Config{}, report, err
	}
	return cfg, report, nil
}

func DefaultBinaryPath() (string, error) {
	binDir, err := config.BinDir()
	if err != nil {
		return "", err
	}
	name := "universal-controller"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(binDir, name), nil
}

func SystemdService(binaryPath string) string {
	if binaryPath == "" {
		binaryPath = "/usr/local/bin/universal-controller"
	}
	execStart := strings.Join([]string{
		strconv.Quote(binaryPath),
		strconv.Quote("receiver"),
		strconv.Quote("start"),
	}, " ")
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=Universal Controller Receiver
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=2
Environment=UC_BOOT_MODE=receiver

[Install]
WantedBy=multi-user.target
`, execStart)) + "\n"
}
