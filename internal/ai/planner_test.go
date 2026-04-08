package ai

import "testing"

func TestNormalizeCommandsPreservesOrder(t *testing.T) {
	commands := []string{
		"sudo apt install nginx",
		"sudo apt update",
		"sudo apt install nginx",
		"systemctl status nginx",
	}

	got := normalizeCommands(commands)
	want := []string{
		"sudo apt install nginx",
		"sudo apt update",
		"systemctl status nginx",
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected length: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected order at %d: got %q want %q", i, got[i], want[i])
		}
	}
}
