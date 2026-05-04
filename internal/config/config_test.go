package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Default()
	cfg.MaskingStyle = MaskingBlank
	cfg.ClipboardClearSecs = 30
	cfg.ConfirmationPrefs.SkipDeleteVersion = true
	cfg.RecordServerUse(ServerEntry{Address: "https://v1:8200", Namespace: "ns1"})
	time.Sleep(5 * time.Millisecond)
	cfg.RecordServerUse(ServerEntry{Address: "https://v2:8200"})

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.MaskingStyle != MaskingBlank {
		t.Errorf("MaskingStyle: %s", got.MaskingStyle)
	}
	if got.ClipboardClearSecs != 30 {
		t.Errorf("ClipboardClearSecs: %d", got.ClipboardClearSecs)
	}
	if !got.ConfirmationPrefs.SkipDeleteVersion {
		t.Errorf("SkipDeleteVersion not preserved")
	}
	if len(got.ServerHistory) != 2 {
		t.Fatalf("history len: %d", len(got.ServerHistory))
	}
	// Most recent should sort first.
	if got.ServerHistory[0].Address != "https://v2:8200" {
		t.Errorf("expected v2 first, got %s", got.ServerHistory[0].Address)
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaskingStyle != MaskingStars {
		t.Errorf("masking: %s", cfg.MaskingStyle)
	}
	if cfg.ClipboardClearSecs != DefaultClipboardClearSecs {
		t.Errorf("clipboard secs: %d", cfg.ClipboardClearSecs)
	}
}
