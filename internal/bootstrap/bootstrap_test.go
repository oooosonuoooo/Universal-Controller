package bootstrap

import (
	"strings"
	"testing"
)

func TestSystemdServiceQuotesBinaryPath(t *testing.T) {
	service := SystemdService("/tmp/path with spaces/universal-controller")
	if !strings.Contains(service, `ExecStart="/tmp/path with spaces/universal-controller" "receiver" "start"`) {
		t.Fatalf("service did not quote ExecStart correctly: %s", service)
	}
}
