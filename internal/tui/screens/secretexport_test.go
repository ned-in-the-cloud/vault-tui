package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/vault"
)

func makeSecret(t *testing.T, kv map[string]interface{}) *vault.KVSecret {
	t.Helper()
	out := &vault.KVSecret{Data: map[string]*secure.SecureString{}}
	for k, v := range kv {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out.Data[k] = secure.NewSecureString(b)
	}
	return out
}

func TestRenderJSON(t *testing.T) {
	sec := makeSecret(t, map[string]interface{}{"a": "alpha", "b": 2})
	defer sec.Destroy()
	got, err := renderJSON(sec)
	if err != nil {
		t.Fatalf("renderJSON: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["a"] != "alpha" {
		t.Errorf("a = %v", decoded["a"])
	}
	if decoded["b"].(float64) != 2 {
		t.Errorf("b = %v", decoded["b"])
	}
}

func TestRenderDotenv(t *testing.T) {
	sec := makeSecret(t, map[string]interface{}{"db-password": "p\"ass", "api.key": "xyz"})
	defer sec.Destroy()
	got, err := renderDotenv(sec)
	if err != nil {
		t.Fatalf("renderDotenv: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `API_KEY="xyz"`) {
		t.Errorf("missing API_KEY line: %s", s)
	}
	if !strings.Contains(s, `DB_PASSWORD="p\"ass"`) {
		t.Errorf("missing escaped DB_PASSWORD: %s", s)
	}
}

func TestRenderYAML(t *testing.T) {
	sec := makeSecret(t, map[string]interface{}{"plain": "value", "tricky": "with: colon"})
	defer sec.Destroy()
	got, err := renderYAML(sec)
	if err != nil {
		t.Fatalf("renderYAML: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "plain: value") {
		t.Errorf("missing plain: %s", s)
	}
	if !strings.Contains(s, `tricky: "with: colon"`) {
		t.Errorf("expected quoted tricky value: %s", s)
	}
}

func TestRenderSingleKey(t *testing.T) {
	sec := makeSecret(t, map[string]interface{}{"k": "v", "n": 42})
	defer sec.Destroy()
	got, err := renderSingleKey(sec, "k")
	if err != nil {
		t.Fatalf("renderSingleKey k: %v", err)
	}
	if string(got) != "v" {
		t.Errorf("k = %q", got)
	}
	gotN, err := renderSingleKey(sec, "n")
	if err != nil {
		t.Fatalf("renderSingleKey n: %v", err)
	}
	if string(gotN) != "42" {
		t.Errorf("n = %q", gotN)
	}
	if _, err := renderSingleKey(sec, "missing"); err == nil {
		t.Errorf("expected error for missing key")
	}
}

func TestWritePrivateFilePerms(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "secret.txt")
	if err := writePrivateFile(p, []byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q", got)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Errorf("perm = %o, want 0600", st.Mode().Perm())
		}
	}
}
