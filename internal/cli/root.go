package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"universal-controller/internal/bootstrap"
	ucclipboard "universal-controller/internal/clipboard"
	"universal-controller/internal/config"
	"universal-controller/internal/doctor"
	"universal-controller/internal/executor"
	"universal-controller/internal/health"
	"universal-controller/internal/receiver"
	"universal-controller/internal/tools"
	"universal-controller/internal/tui"
)

func Execute() error {
	root := &cobra.Command{
		Use:           "universal-controller",
		Aliases:       []string{"uc"},
		Short:         "Keyboard-first device controller with self, controller and receiver modes",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, report, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			return tui.Run(cfg, report)
		},
	}
	root.AddCommand(tuiCommand(), doctorCommand(), repairCommand(), receiverCommand(), execCommand(), clipboardCommand(), configCommand(), toolsCommand(), ideCommand())
	return root.Execute()
}

func tuiCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the full-screen keyboard UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, report, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			return tui.Run(cfg, report)
		},
	}
}

func doctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Inspect local dependencies and config health",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			checks := doctor.Run(cfg)
			fmt.Println(doctor.Render(checks))
			if doctor.HasFailures(checks) {
				return errors.New("doctor found failing checks")
			}
			return nil
		},
	}
}

func repairCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "repair",
		Short: "Repair config, directories and generated defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, report, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			fmt.Printf("role=%s\n", cfg.Role)
			if report.Created {
				fmt.Println("created new config")
			}
			if len(report.Notes) == 0 {
				fmt.Println("no repairs needed")
				return nil
			}
			for _, note := range report.Notes {
				fmt.Println("- " + note)
			}
			return nil
		},
	}
}

func receiverCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "receiver",
		Short: "Manage the receiver agent",
	}
	command.AddCommand(receiverStartCommand(), receiverInstallServiceCommand())
	return command
}

func receiverStartCommand() *cobra.Command {
	var host string
	var port int
	var token string
	var public bool
	command := &cobra.Command{
		Use:   "start",
		Short: "Start the receiver listener",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			if public && strings.TrimSpace(host) == "" {
				host = config.PublicReceiverHost
			}
			if value := strings.TrimSpace(host); value != "" {
				cfg.Receiver.Host = value
			}
			if port > 0 {
				cfg.Receiver.Port = port
			}
			if value := strings.TrimSpace(token); value != "" {
				cfg.Receiver.Token = value
			}
			cfg.Role = "receiver"
			cfg.Receiver.Enabled = true
			if err := config.Save(cfg); err != nil {
				return err
			}
			local := executor.LocalExecutor{Shell: cfg.Shell.Default, Name: "receiver-local"}
			server := receiver.New(cfg, local)
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = server.Shutdown(shutdownCtx)
			}()
			fmt.Printf("Receiver listening on %s:%d\n", cfg.Receiver.Host, cfg.Receiver.Port)
			fmt.Printf("Token: %s\n", cfg.Receiver.Token)
			if config.ReceiverExposed(cfg.Receiver.Host) {
				fmt.Println("Warning: receiver is exposed beyond localhost. Use this only on trusted networks.")
			}
			return server.Start()
		},
	}
	command.Flags().StringVar(&host, "host", "", "receiver bind host; defaults to the saved config")
	command.Flags().IntVar(&port, "port", 0, "receiver bind port; defaults to the saved config")
	command.Flags().StringVar(&token, "token", "", "receiver bearer token; defaults to the saved config")
	command.Flags().BoolVar(&public, "public", false, "bind to 0.0.0.0 instead of localhost")
	return command
}

func receiverInstallServiceCommand() *cobra.Command {
	var binaryPath string
	var servicePath string
	command := &cobra.Command{
		Use:   "install-service",
		Short: "Write a Linux systemd service so receiver mode starts at boot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "linux" {
				return errors.New("systemd service installation is only supported on Linux in this build")
			}
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			if binaryPath == "" {
				binaryPath, err = os.Executable()
				if err != nil {
					return err
				}
			}
			binaryPath, err = normalizeBinaryPath(binaryPath)
			if err != nil {
				return err
			}
			if servicePath == "" {
				servicePath = "/etc/systemd/system/universal-controller-receiver.service"
			}
			content := bootstrap.SystemdService(binaryPath)
			if err := os.WriteFile(servicePath, []byte(content), 0o644); err != nil {
				return err
			}
			cfg.Receiver.AutoStart = true
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Println("service file written to", servicePath)
			fmt.Println("run: sudo systemctl daemon-reload && sudo systemctl enable --now universal-controller-receiver.service")
			return nil
		},
	}
	command.Flags().StringVar(&binaryPath, "binary", "", "optional binary path; defaults to the currently running executable")
	command.Flags().StringVar(&servicePath, "service-path", "", "where to write the systemd service file")
	return command
}

func execCommand() *cobra.Command {
	var receiverURL string
	var token string
	var mode string
	var rootPassword string
	var timeoutSeconds int
	command := &cobra.Command{
		Use:   "exec [command]",
		Short: "Execute one command locally or on a receiver",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			if receiverURL != "" && strings.TrimSpace(token) == "" {
				return errors.New("receiver token is required for remote execution")
			}
			var client executor.Client
			if receiverURL != "" {
				client = executor.ReceiverClient{BaseURL: receiverURL, Token: token, Name: "receiver"}
			} else {
				client = executor.LocalExecutor{Shell: cfg.Shell.Default, Name: "local"}
			}
			result, err := client.Execute(context.Background(), executor.Request{
				Command:      args[0],
				Mode:         mode,
				RootPassword: rootPassword,
				Timeout:      time.Duration(timeoutSeconds) * time.Second,
			})
			if err != nil {
				return err
			}
			raw, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(raw))
			return nil
		},
	}
	command.Flags().StringVar(&receiverURL, "receiver", "", "receiver base URL such as http://192.168.1.5:8080")
	command.Flags().StringVar(&token, "token", "", "receiver token")
	command.Flags().StringVar(&mode, "mode", "normal", "execution mode: normal or root")
	command.Flags().StringVar(&rootPassword, "root-password", "", "sudo password used only when mode=root")
	command.Flags().IntVar(&timeoutSeconds, "timeout", 120, "execution timeout in seconds")
	return command
}

func clipboardCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "clipboard",
		Short: "Basic clipboard copy and paste helpers",
	}
	command.AddCommand(clipboardCopyCommand(), clipboardPasteCommand(), clipboardClearCommand())
	return command
}

func clipboardCopyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "copy [text]",
		Short: "Copy text into the system clipboard",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := readClipboardInput(args)
			if err != nil {
				return err
			}
			if err := ucclipboard.Write(value); err != nil {
				return err
			}
			fmt.Println("clipboard updated")
			return nil
		},
	}
}

func clipboardPasteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "paste",
		Short: "Print the current clipboard contents",
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := ucclipboard.Read()
			if err != nil {
				return err
			}
			fmt.Println(value)
			return nil
		},
	}
}

func clipboardClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the current clipboard contents",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ucclipboard.Write(""); err != nil {
				return err
			}
			fmt.Println("clipboard cleared")
			return nil
		},
	}
}

func configCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Inspect or change saved configuration",
	}
	command.AddCommand(configShowCommand(), configSetRoleCommand(), configSetAIKeyCommand())
	return command
}

func toolsCommand() *cobra.Command {
	var all bool
	var limit int
	command := &cobra.Command{
		Use:   "tools",
		Short: "List discovered tools and curated integrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := tools.CuratedCatalog()
			if all {
				catalog = append(catalog, tools.DiscoverPathTools(limit)...)
			}
			fmt.Println(tools.Render(catalog))
			return nil
		},
	}
	command.Flags().BoolVar(&all, "all", false, "append every executable discovered on PATH")
	command.Flags().IntVar(&limit, "limit", 200, "maximum number of discovered PATH tools when --all is set")
	return command
}

func ideCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "ide",
		Short: "IDE-friendly JSON and task entrypoints",
	}
	command.AddCommand(ideStatusCommand(), ideDoctorCommand(), ideHealthCommand(), ideToolsCommand(), ideExecCommand())
	return command
}

func ideStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Emit config and environment summary as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, report, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			payload := map[string]any{
				"role":             cfg.Role,
				"layout":           cfg.UI.Layout,
				"shell":            cfg.Shell.Default,
				"receiver_host":    cfg.Receiver.Host,
				"receiver_port":    cfg.Receiver.Port,
				"receiver_token":   cfg.Receiver.Token,
				"receiver_exposed": config.ReceiverExposed(cfg.Receiver.Host),
				"repairs":          report.Notes,
				"created":          report.Created,
			}
			return printJSON(payload)
		},
	}
}

func ideDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Emit dependency checks as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			return printJSON(doctor.Run(cfg))
		},
	}
}

func ideHealthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Emit local health metrics as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := health.Collect()
			if err != nil {
				return err
			}
			return printJSON(report)
		},
	}
}

func ideToolsCommand() *cobra.Command {
	var all bool
	var limit int
	command := &cobra.Command{
		Use:   "tools",
		Short: "Emit tool discovery results as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := tools.CuratedCatalog()
			if all {
				catalog = append(catalog, tools.DiscoverPathTools(limit)...)
			}
			return printJSON(catalog)
		},
	}
	command.Flags().BoolVar(&all, "all", true, "include executables discovered on PATH")
	command.Flags().IntVar(&limit, "limit", 200, "maximum number of discovered PATH tools")
	return command
}

func ideExecCommand() *cobra.Command {
	var toolName string
	var receiverURL string
	var token string
	var mode string
	var timeoutSeconds int
	command := &cobra.Command{
		Use:   "exec [arguments...]",
		Short: "Run a discovered tool with JSON output for IDE tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			if toolName == "" {
				return errors.New("tool name is required")
			}
			tool, ok := tools.FindByName(toolName)
			if !ok {
				return fmt.Errorf("tool %q was not found on PATH or in the curated catalog", toolName)
			}
			commandLine := tool.Command
			if len(args) > 0 {
				commandLine += " " + shellJoin(args)
			}
			if receiverURL != "" && strings.TrimSpace(token) == "" {
				return errors.New("receiver token is required for remote execution")
			}
			var client executor.Client
			if receiverURL != "" {
				client = executor.ReceiverClient{BaseURL: receiverURL, Token: token, Name: "receiver"}
			} else {
				client = executor.LocalExecutor{Shell: cfg.Shell.Default, Name: "local"}
			}
			result, err := client.Execute(context.Background(), executor.Request{
				Command: commandLine,
				Mode:    mode,
				Timeout: time.Duration(timeoutSeconds) * time.Second,
			})
			if err != nil {
				return err
			}
			return printJSON(map[string]any{
				"tool":   tool.Name,
				"target": client.Label(),
				"result": result,
			})
		},
	}
	command.Flags().StringVar(&toolName, "tool", "", "tool or executable name, such as nmap, yt-dlp, docker or tmux")
	command.Flags().StringVar(&receiverURL, "receiver", "", "receiver base URL for remote execution")
	command.Flags().StringVar(&token, "token", "", "receiver token")
	command.Flags().StringVar(&mode, "mode", "normal", "execution mode: normal or root")
	command.Flags().IntVar(&timeoutSeconds, "timeout", 120, "execution timeout in seconds")
	return command
}

func configShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the saved config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			raw, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(raw))
			return nil
		},
	}
}

func configSetRoleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-role [controller|receiver|self-use]",
		Short: "Persist the default role used at startup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			cfg.Role = args[0]
			return config.Save(cfg)
		},
	}
}

func configSetAIKeyCommand() *cobra.Command {
	var provider string
	var model string
	command := &cobra.Command{
		Use:   "set-ai-key [value]",
		Short: "Store an API key for chatgpt, gemini or claude",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := bootstrap.Ensure()
			if err != nil {
				return err
			}
			switch provider {
			case "chatgpt", "openai":
				cfg.AI.OpenAI.APIKey = args[0]
				if model != "" {
					cfg.AI.OpenAI.Model = model
				}
			case "gemini":
				cfg.AI.Gemini.APIKey = args[0]
				if model != "" {
					cfg.AI.Gemini.Model = model
				}
			case "claude":
				cfg.AI.Claude.APIKey = args[0]
				if model != "" {
					cfg.AI.Claude.Model = model
				}
			default:
				return fmt.Errorf("unsupported provider %q", provider)
			}
			return config.Save(cfg)
		},
	}
	command.Flags().StringVar(&provider, "provider", "", "chatgpt, gemini or claude")
	command.Flags().StringVar(&model, "model", "", "override the model name")
	_ = command.MarkFlagRequired("provider")
	return command
}

func printJSON(payload any) error {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\"'\"'")+"'")
	}
	return strings.Join(quoted, " ")
}

func normalizeBinaryPath(input string) (string, error) {
	resolved, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	if symlinkTarget, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = symlinkTarget
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("binary path points to a directory: %s", resolved)
	}
	return resolved, nil
}

func readClipboardInput(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", errors.New("provide text as arguments or pipe stdin into the command")
	}
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	value := strings.TrimRight(string(payload), "\r\n")
	if value == "" {
		return "", errors.New("clipboard input is empty")
	}
	return value, nil
}
