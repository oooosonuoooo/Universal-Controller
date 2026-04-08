package receiver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"universal-controller/internal/config"
	"universal-controller/internal/executor"
)

func TestHandleHealthRequiresToken(t *testing.T) {
	cfg := config.Default()
	cfg.Device.Name = "test-device"
	cfg.Receiver.Token = "0123456789abcdef0123456789abcdef"

	server := New(cfg, executor.LocalExecutor{Name: "test"})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	server.handleHealth(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestHandleExecRejectsRemoteRoot(t *testing.T) {
	cfg := config.Default()
	cfg.Receiver.Token = "0123456789abcdef0123456789abcdef"

	server := New(cfg, executor.LocalExecutor{Name: "test"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exec", strings.NewReader(`{"command":"whoami","mode":"root"}`))
	request.Header.Set("Authorization", "Bearer "+cfg.Receiver.Token)
	recorder := httptest.NewRecorder()

	server.handleExec(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestHandleExecRejectsEmptyCommand(t *testing.T) {
	cfg := config.Default()
	cfg.Receiver.Token = "0123456789abcdef0123456789abcdef"

	server := New(cfg, executor.LocalExecutor{Name: "test"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exec", strings.NewReader(`{"command":"   "}`))
	request.Header.Set("Authorization", "Bearer "+cfg.Receiver.Token)
	recorder := httptest.NewRecorder()

	server.handleExec(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestReceiverClientExecutesAgainstServer(t *testing.T) {
	cfg := config.Default()
	cfg.Receiver.Token = "0123456789abcdef0123456789abcdef"

	server := New(cfg, executor.LocalExecutor{
		Shell: config.DetectShell(),
		Name:  "test",
	})
	httpServer := httptest.NewServer(server.server.Handler)
	defer httpServer.Close()

	client := executor.ReceiverClient{
		BaseURL: httpServer.URL,
		Token:   cfg.Receiver.Token,
		HTTP:    httpServer.Client(),
		Name:    "receiver",
	}

	result, err := client.Execute(context.Background(), executor.Request{
		Command: "echo receiver-smoke-test",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("expected remote execution to succeed: %v", err)
	}
	if !strings.Contains(result.Stdout, "receiver-smoke-test") {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}
