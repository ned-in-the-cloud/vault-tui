package vault

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ned1313/vault-tui/internal/secure"
)

// fakeKVEngine implements KVv2Engine for unit tests. Only the methods the
// tests touch are wired up.
type fakeKVEngine struct {
	mount      string
	version    int
	putCalled  bool
	gotPath    string
	gotData    map[string]interface{}
	casSeen    *int
	putErr     error
	getResult  *KVSecret
	getErr     error
	listResult []KVEntry
	listErr    error
}

func (f *fakeKVEngine) Version() int  { return f.version }
func (f *fakeKVEngine) Mount() string { return f.mount }
func (f *fakeKVEngine) List(_ context.Context, _ string) ([]KVEntry, error) {
	return f.listResult, f.listErr
}
func (f *fakeKVEngine) Get(_ context.Context, _ string) (*KVSecret, error) {
	return f.getResult, f.getErr
}
func (f *fakeKVEngine) Put(_ context.Context, p string, d map[string]interface{}) error {
	f.putCalled = true
	f.gotPath = p
	f.gotData = d
	return f.putErr
}
func (f *fakeKVEngine) Delete(_ context.Context, _ string) error { return nil }

// v2 extras (no-op for the tests we need)
func (f *fakeKVEngine) GetVersion(_ context.Context, _ string, _ int) (*KVSecret, error) {
	return nil, nil
}
func (f *fakeKVEngine) GetVersionsList(_ context.Context, _ string) ([]KVVersionMeta, error) {
	return nil, nil
}
func (f *fakeKVEngine) GetMetadata(_ context.Context, _ string) (*KVMetadata, error) {
	return nil, nil
}
func (f *fakeKVEngine) GetSubkeys(_ context.Context, _ string, _ int, _ int) ([]string, error) {
	return nil, nil
}
func (f *fakeKVEngine) PutCAS(_ context.Context, p string, d map[string]interface{}, cas *int) error {
	f.putCalled = true
	f.gotPath = p
	f.gotData = d
	f.casSeen = cas
	return f.putErr
}
func (f *fakeKVEngine) Patch(_ context.Context, _ string, _ map[string]interface{}, _ *int) error {
	return nil
}
func (f *fakeKVEngine) DeleteVersions(_ context.Context, _ string, _ []int) error   { return nil }
func (f *fakeKVEngine) UndeleteVersions(_ context.Context, _ string, _ []int) error { return nil }
func (f *fakeKVEngine) DestroyVersions(_ context.Context, _ string, _ []int) error  { return nil }
func (f *fakeKVEngine) DeleteAllVersions(_ context.Context, _ string) error         { return nil }

func TestSecureSecretFromV1Data(t *testing.T) {
	src := map[string]interface{}{
		"username": "alice",
		"port":     5432.0,
		"tls":      true,
	}
	s, err := secureSecretFromV1Data(src)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if len(s.Data) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(s.Data))
	}
	// Source values nilled out.
	for k, v := range src {
		if v != nil {
			t.Errorf("src[%q] not nilled: %v", k, v)
		}
	}
	// Round-trip: opening and decoding should yield original values.
	var got string
	if err := s.Data["username"].WithBytes(func(b []byte) error {
		return json.Unmarshal(b, &got)
	}); err != nil {
		t.Fatalf("decode username: %v", err)
	}
	if got != "alice" {
		t.Errorf("username = %q, want alice", got)
	}
	s.Destroy()
	if len(s.Data) != 0 {
		t.Errorf("Destroy should clear map, got %d entries", len(s.Data))
	}
}

func TestPutFromSecureRoundTrip(t *testing.T) {
	in := map[string]*secure.SecureString{
		"username": secure.NewSecureString([]byte(`"alice"`)),
		"port":     secure.NewSecureString([]byte(`5432`)),
	}
	fake := &fakeKVEngine{}
	if err := PutFromSecure(context.Background(), fake, "creds/db", in); err != nil {
		t.Fatalf("PutFromSecure: %v", err)
	}
	if !fake.putCalled {
		t.Fatal("Put was not called")
	}
	if fake.gotPath != "creds/db" {
		t.Errorf("path = %q", fake.gotPath)
	}
	if fake.gotData["username"] != "alice" {
		t.Errorf("username = %v", fake.gotData["username"])
	}
	if fake.gotData["port"].(float64) != 5432 {
		t.Errorf("port = %v", fake.gotData["port"])
	}
	// SecureStrings must have been destroyed.
	for k, ss := range in {
		if !ss.IsEmpty() {
			t.Errorf("source %q not destroyed", k)
		}
	}
}

func TestPutFromSecureV2WithCAS(t *testing.T) {
	in := map[string]*secure.SecureString{
		"k": secure.NewSecureString([]byte(`"v"`)),
	}
	fake := &fakeKVEngine{}
	cas := 7
	if err := PutFromSecureV2(context.Background(), fake, "p", in, &cas); err != nil {
		t.Fatalf("PutFromSecureV2: %v", err)
	}
	if fake.casSeen == nil || *fake.casSeen != 7 {
		t.Errorf("cas = %v", fake.casSeen)
	}
}

func TestPutFromSecurePropagatesError(t *testing.T) {
	in := map[string]*secure.SecureString{
		"k": secure.NewSecureString([]byte(`"v"`)),
	}
	fake := &fakeKVEngine{putErr: errors.New("boom")}
	if err := PutFromSecure(context.Background(), fake, "p", in); err == nil {
		t.Fatal("expected error")
	}
	if !in["k"].IsEmpty() {
		t.Error("source not destroyed on error")
	}
}

func TestListEntriesFromKeysSorting(t *testing.T) {
	got := listEntriesFromKeys([]string{"zebra", "alpha/", "bravo", "alpha2/"})
	want := []KVEntry{
		{Name: "alpha", IsFolder: true},
		{Name: "alpha2", IsFolder: true},
		{Name: "bravo"},
		{Name: "zebra"},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d", len(got))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestFlattenSubkeys(t *testing.T) {
	tree := map[string]interface{}{
		"top":    nil,
		"nested": map[string]interface{}{"a": nil, "b": map[string]interface{}{"c": nil}},
	}
	var out []string
	flattenSubkeys("", tree, &out)
	// order isn't deterministic; check membership
	seen := map[string]bool{}
	for _, s := range out {
		seen[s] = true
	}
	for _, want := range []string{"top", "nested.a", "nested.b.c"} {
		if !seen[want] {
			t.Errorf("missing %q in %v", want, out)
		}
	}
}
