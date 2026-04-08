package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ReceiverClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	Name    string
}

func (c ReceiverClient) Label() string {
	if c.Name != "" {
		return c.Name
	}
	return c.BaseURL
}

func (c ReceiverClient) Execute(ctx context.Context, req Request) (Result, error) {
	if req.Mode == "root" || strings.TrimSpace(req.RootPassword) != "" {
		return Result{}, fmt.Errorf("remote root execution is disabled")
	}
	baseURL, err := c.normalizedBaseURL()
	if err != nil {
		return Result{}, err
	}
	token := strings.TrimSpace(c.Token)
	if token == "" {
		return Result{}, fmt.Errorf("receiver token is required")
	}
	body, err := json.Marshal(map[string]any{
		"command":       req.Command,
		"mode":          req.Mode,
		"root_password": req.RootPassword,
		"dir":           req.Dir,
		"timeout_secs":  int(req.Timeout.Seconds()),
	})
	if err != nil {
		return Result{}, err
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/exec", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := httpClient.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return Result{}, fmt.Errorf("receiver returned status %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	var result Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (c ReceiverClient) Health(ctx context.Context) (map[string]any, error) {
	baseURL, err := c.normalizedBaseURL()
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(c.Token)
	if token == "" {
		return nil, fmt.Errorf("receiver token is required")
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("receiver health returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c ReceiverClient) normalizedBaseURL() (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("receiver base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("receiver URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("receiver URL must include a host")
	}
	return baseURL, nil
}
