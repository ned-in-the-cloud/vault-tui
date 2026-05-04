package screens

import (
	"testing"
	"time"

	"github.com/ned1313/vault-tui/internal/vault"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "∞"},
		{30 * time.Second, "30s"},
		{2*time.Minute + 5*time.Second, "2m05s"},
		{1*time.Hour + 2*time.Minute + 3*time.Second, "1h02m03s"},
	}
	for _, c := range cases {
		got := formatDuration(c.in)
		if got != c.want {
			t.Errorf("formatDuration(%v) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestRemaining(t *testing.T) {
	if got := remaining(nil); got != 0 {
		t.Fatalf("nil token: got %v want 0", got)
	}
	now := time.Now()
	ti := &vault.TokenInfo{ExpireTime: now.Add(10 * time.Second)}
	got := remaining(ti)
	if got <= 0 || got > 10*time.Second {
		t.Fatalf("expected ~10s, got %v", got)
	}
	expired := &vault.TokenInfo{ExpireTime: now.Add(-time.Second)}
	if got := remaining(expired); got != 0 {
		t.Fatalf("expected 0 for expired, got %v", got)
	}
	noExpire := &vault.TokenInfo{TTL: 5 * time.Minute}
	if got := remaining(noExpire); got != 5*time.Minute {
		t.Fatalf("expected 5m fallback, got %v", got)
	}
}
