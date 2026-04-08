package command

import "testing"

func TestParseAICommand(t *testing.T) {
	parsed := Parse("chatgpt check server health")
	if parsed.Kind != KindAI {
		t.Fatalf("expected AI kind, got %s", parsed.Kind)
	}
	if parsed.Provider != "chatgpt" {
		t.Fatalf("expected chatgpt provider, got %s", parsed.Provider)
	}
	if len(parsed.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(parsed.Args))
	}
}

func TestParseLayoutCommand(t *testing.T) {
	parsed := Parse(":layout vertical")
	if parsed.Kind != KindLayout {
		t.Fatalf("expected layout kind, got %s", parsed.Kind)
	}
	if len(parsed.Args) != 1 || parsed.Args[0] != "vertical" {
		t.Fatalf("unexpected args: %#v", parsed.Args)
	}
}

func TestParseShellFallback(t *testing.T) {
	parsed := Parse("ls -la")
	if parsed.Kind != KindShell {
		t.Fatalf("expected shell kind, got %s", parsed.Kind)
	}
}
