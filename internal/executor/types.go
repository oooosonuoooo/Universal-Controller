package executor

import (
	"context"
	"fmt"
	"time"
)

type Request struct {
	Command      string        `json:"command"`
	Mode         string        `json:"mode"`
	RootPassword string        `json:"root_password,omitempty"`
	Dir          string        `json:"dir,omitempty"`
	Timeout      time.Duration `json:"-"`
}

type Result struct {
	Command   string        `json:"command"`
	Stdout    string        `json:"stdout"`
	Stderr    string        `json:"stderr"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration"`
	StartedAt time.Time     `json:"started_at"`
	Finished  time.Time     `json:"finished_at"`
}

func (r Result) CombinedOutput() string {
	if r.Stdout == "" {
		return r.Stderr
	}
	if r.Stderr == "" {
		return r.Stdout
	}
	return fmt.Sprintf("%s\n%s", r.Stdout, r.Stderr)
}

type Client interface {
	Execute(context.Context, Request) (Result, error)
	Label() string
}
