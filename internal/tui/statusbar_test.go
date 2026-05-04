package tui

import (
	"testing"
	"time"

	"github.com/ned1313/vault-tui/internal/vault"
)

func TestFormatTTL(t *testing.T) {
	cases := []struct {
		name string
		ti   *vault.TokenInfo
		want string
	}{
		{"infinite", &vault.TokenInfo{}, "∞"},
		{"seconds", &vault.TokenInfo{ExpireTime: time.Now().Add(45 * time.Second)}, "45s"},
		{"minutes", &vault.TokenInfo{ExpireTime: time.Now().Add(5*time.Minute + 10*time.Second)}, "5m10s"},
		{"hours", &vault.TokenInfo{ExpireTime: time.Now().Add(2*time.Hour + 30*time.Minute)}, "2h30m"},
		{"expired", &vault.TokenInfo{ExpireTime: time.Now().Add(-time.Second)}, "expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatTTL(tc.ti)
			// Allow ±1s drift on relative tests.
			if got != tc.want && !approxEqual(got, tc.want) {
				t.Fatalf("formatTTL: got %q want %q", got, tc.want)
			}
		})
	}
}

// approxEqual handles 1-second jitter on time-dependent strings (e.g.
// "44s" vs "45s") that can occur if the clock advances during the test.
func approxEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	return diff <= 1
}
