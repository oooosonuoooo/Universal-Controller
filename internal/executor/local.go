package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type LocalExecutor struct {
	Shell string
	Name  string
}

func (e LocalExecutor) Label() string {
	if e.Name != "" {
		return e.Name
	}
	return "local"
}

func (e LocalExecutor) Execute(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Command) == "" {
		return Result{}, errors.New("empty command")
	}
	started := time.Now()
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd, stdin, err := e.commandForRequest(ctx, req)
	if err != nil {
		return Result{}, err
	}
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	runErr := cmd.Run()
	finished := time.Now()
	result := Result{
		Command:   req.Command,
		Stdout:    strings.TrimRight(stdout.String(), "\n"),
		Stderr:    strings.TrimRight(stderr.String(), "\n"),
		ExitCode:  0,
		Duration:  finished.Sub(started),
		StartedAt: started,
		Finished:  finished,
	}
	if runErr == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitCode(exitErr)
		return result, nil
	}
	return result, runErr
}

func (e LocalExecutor) commandForRequest(ctx context.Context, req Request) (*exec.Cmd, string, error) {
	if runtime.GOOS == "windows" {
		return windowsCommand(ctx, e.Shell, req)
	}
	if req.Mode == "root" {
		if req.RootPassword == "" {
			return nil, "", errors.New("root mode requires a password for sudo execution")
		}
		return exec.CommandContext(ctx, "sudo", "-S", "-p", "", "sh", "-c", req.Command), req.RootPassword + "\n", nil
	}
	shell := e.Shell
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.CommandContext(ctx, shell, "-c", req.Command), "", nil
}

func windowsCommand(ctx context.Context, shell string, req Request) (*exec.Cmd, string, error) {
	if req.Mode == "root" {
		return nil, "", errors.New("root mode is not supported on Windows in this build")
	}
	resolved := strings.TrimSpace(shell)
	if resolved == "" {
		resolved = "powershell.exe"
	}
	if strings.Contains(strings.ToLower(resolved), "powershell") || strings.Contains(strings.ToLower(resolved), "pwsh") {
		return exec.CommandContext(ctx, resolved, "-NoLogo", "-NoProfile", "-Command", req.Command), "", nil
	}
	return exec.CommandContext(ctx, resolved, "/C", req.Command), "", nil
}

func exitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}
