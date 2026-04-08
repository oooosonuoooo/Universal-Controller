package config

import "testing"

func TestNormalizeRepairsCriticalDefaults(t *testing.T) {
	cfg := Config{}
	notes := Normalize(&cfg)
	if len(notes) == 0 {
		t.Fatal("expected repair notes")
	}
	if cfg.Version != 1 {
		t.Fatalf("expected version 1, got %d", cfg.Version)
	}
	if cfg.UI.Layout != "horizontal" {
		t.Fatalf("expected horizontal layout, got %s", cfg.UI.Layout)
	}
	if cfg.Receiver.Host != DefaultHost {
		t.Fatalf("expected receiver host %s, got %s", DefaultHost, cfg.Receiver.Host)
	}
	if cfg.Receiver.Token == "" {
		t.Fatal("expected receiver token to be generated")
	}
	if cfg.Shell.Default == "" {
		t.Fatal("expected shell default to be populated")
	}
}

func TestNormalizeRegeneratesWeakReceiverToken(t *testing.T) {
	cfg := Config{
		Receiver: ReceiverConfig{
			Host:  DefaultHost,
			Port:  DefaultPort,
			Token: "short",
		},
	}

	notes := Normalize(&cfg)
	if cfg.Receiver.Token == "short" {
		t.Fatal("expected weak token to be replaced")
	}
	if len(notes) == 0 {
		t.Fatal("expected repair notes for weak token")
	}
}

func TestReceiverExposed(t *testing.T) {
	if ReceiverExposed("127.0.0.1") {
		t.Fatal("127.0.0.1 should not be treated as exposed")
	}
	if !ReceiverExposed(PublicReceiverHost) {
		t.Fatal("0.0.0.0 should be treated as exposed")
	}
}
