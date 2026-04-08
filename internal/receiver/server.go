package receiver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"universal-controller/internal/config"
	"universal-controller/internal/executor"
)

const (
	maxRequestBodyBytes = 64 * 1024
	maxCommandLength    = 4096
	maxTimeoutSeconds   = 600
)

type Server struct {
	cfg      config.Config
	executor executor.LocalExecutor
	server   *http.Server
}

type execRequest struct {
	Command      string `json:"command"`
	Mode         string `json:"mode"`
	RootPassword string `json:"root_password"`
	Dir          string `json:"dir"`
	TimeoutSecs  int    `json:"timeout_secs"`
}

func New(cfg config.Config, local executor.LocalExecutor) *Server {
	s := &Server{cfg: cfg, executor: local}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/exec", s.handleExec)
	s.server = &http.Server{
		Addr:              cfg.Receiver.Host + ":" + strconv.Itoa(cfg.Receiver.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    8 * 1024,
	}
	return s
}

func (s *Server) Start() error {
	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(request) {
		writeError(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"name":      s.cfg.Device.Name,
		"role":      "receiver",
		"platform":  s.cfg.Device.Platform,
		"layout":    s.cfg.UI.Layout,
		"receiver":  true,
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) handleExec(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(request) {
		writeError(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	var payload execRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(payload.Command) == "" {
		writeError(writer, http.StatusBadRequest, "command is required")
		return
	}
	if len(payload.Command) > maxCommandLength {
		writeError(writer, http.StatusBadRequest, "command exceeds maximum length")
		return
	}
	if payload.Mode == "root" || strings.TrimSpace(payload.RootPassword) != "" {
		writeError(writer, http.StatusForbidden, "remote root execution is disabled")
		return
	}
	if payload.Mode != "" && payload.Mode != "normal" {
		writeError(writer, http.StatusBadRequest, "unsupported execution mode")
		return
	}
	timeout := 120 * time.Second
	if payload.TimeoutSecs > 0 {
		timeout = time.Duration(payload.TimeoutSecs) * time.Second
		if timeout > maxTimeoutSeconds*time.Second {
			timeout = maxTimeoutSeconds * time.Second
		}
	}
	result, err := s.executor.Execute(request.Context(), executor.Request{
		Command:      payload.Command,
		Mode:         payload.Mode,
		RootPassword: payload.RootPassword,
		Dir:          payload.Dir,
		Timeout:      timeout,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (s *Server) authorized(request *http.Request) bool {
	if strings.TrimSpace(s.cfg.Receiver.Token) == "" {
		return false
	}
	authHeader := strings.TrimSpace(request.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.Receiver.Token)) == 1
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}
