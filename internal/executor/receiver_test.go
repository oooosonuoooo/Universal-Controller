package executor

import (
	"context"
	"testing"
)

func TestReceiverClientRejectsRemoteRoot(t *testing.T) {
	client := ReceiverClient{
		BaseURL: "http://127.0.0.1:8080",
		Token:   "0123456789abcdef0123456789abcdef",
	}

	_, err := client.Execute(context.Background(), Request{
		Command: "whoami",
		Mode:    "root",
	})
	if err == nil {
		t.Fatal("expected remote root execution to be rejected")
	}
}

func TestReceiverClientRequiresToken(t *testing.T) {
	client := ReceiverClient{
		BaseURL: "http://127.0.0.1:8080",
	}

	_, err := client.Execute(context.Background(), Request{Command: "whoami"})
	if err == nil {
		t.Fatal("expected missing token to be rejected")
	}
}

func TestReceiverClientValidatesBaseURL(t *testing.T) {
	client := ReceiverClient{
		BaseURL: "127.0.0.1:8080",
		Token:   "0123456789abcdef0123456789abcdef",
	}

	_, err := client.Execute(context.Background(), Request{Command: "whoami"})
	if err == nil {
		t.Fatal("expected invalid receiver URL to be rejected")
	}
}
