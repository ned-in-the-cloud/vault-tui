package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/ned1313/vault-tui/internal/secure"
)

// KVEntry is one item returned by an engine List.
type KVEntry struct {
	Name     string // entry name without trailing slash
	IsFolder bool   // true if Vault returned the entry with a trailing "/"
}

// KVVersionMeta describes a single version of a KV-v2 secret.
type KVVersionMeta struct {
	Version      int
	CreatedTime  time.Time
	DeletionTime time.Time
	Destroyed    bool
}

// KVMetadata describes the metadata for a KV-v2 secret.
type KVMetadata struct {
	CASRequired        bool
	CreatedTime        time.Time
	CurrentVersion     int
	CustomMetadata     map[string]string
	DeleteVersionAfter time.Duration
	MaxVersions        int
	OldestVersion      int
	UpdatedTime        time.Time
	Versions           map[int]KVVersionMeta
}

// KVSecret carries the data and metadata for a single KV secret.
//
// Each value in Data is the JSON-encoded representation of the underlying
// Vault value, wrapped in a SecureString so the plaintext lives in a
// memguard enclave rather than the regular Go heap. Callers must Destroy
// every SecureString once they are done; KVSecret.Destroy() does that for
// the whole map at once.
type KVSecret struct {
	Data            map[string]*secure.SecureString
	VersionMetadata *KVVersionMeta
	CustomMetadata  map[string]string
}

// Destroy zeros every SecureString owned by the secret. Safe to call on a
// nil receiver and idempotent.
func (s *KVSecret) Destroy() {
	if s == nil {
		return
	}
	for k, v := range s.Data {
		v.Destroy()
		delete(s.Data, k)
	}
}

// KVEngine is the lowest common denominator across KV v1 and v2.
type KVEngine interface {
	Version() int
	Mount() string
	List(ctx context.Context, path string) ([]KVEntry, error)
	Get(ctx context.Context, path string) (*KVSecret, error)
	Put(ctx context.Context, path string, data map[string]interface{}) error
	Delete(ctx context.Context, path string) error
}

// KVv2Engine extends KVEngine with the version- and metadata-aware
// operations available on KV v2 mounts.
type KVv2Engine interface {
	KVEngine
	GetVersion(ctx context.Context, path string, version int) (*KVSecret, error)
	GetVersionsList(ctx context.Context, path string) ([]KVVersionMeta, error)
	GetMetadata(ctx context.Context, path string) (*KVMetadata, error)
	GetSubkeys(ctx context.Context, path string, version int, depth int) ([]string, error)
	PutCAS(ctx context.Context, path string, data map[string]interface{}, cas *int) error
	Patch(ctx context.Context, path string, data map[string]interface{}, cas *int) error
	DeleteVersions(ctx context.Context, path string, versions []int) error
	UndeleteVersions(ctx context.Context, path string, versions []int) error
	DestroyVersions(ctx context.Context, path string, versions []int) error
	DeleteAllVersions(ctx context.Context, path string) error
}

// PutFromSecure opens the SecureString values, JSON-decodes each one into
// an interface, calls e.Put with the assembled map, and destroys every
// SecureString it touched before returning. The map itself is not held
// after the call.
func PutFromSecure(ctx context.Context, e KVEngine, path string, data map[string]*secure.SecureString) error {
	plain, err := decodeSecureMap(data)
	if err != nil {
		return err
	}
	defer destroySecureMap(data)
	return e.Put(ctx, path, plain)
}

// PutFromSecureV2 is the KV v2 variant that supports an optional CAS.
func PutFromSecureV2(ctx context.Context, e KVv2Engine, path string, data map[string]*secure.SecureString, cas *int) error {
	plain, err := decodeSecureMap(data)
	if err != nil {
		return err
	}
	defer destroySecureMap(data)
	return e.PutCAS(ctx, path, plain, cas)
}

// decodeSecureMap opens each enclave, json-decodes the bytes, and returns
// the plaintext as a regular map. Callers are responsible for destroying
// the source SecureStrings; the returned map only lives long enough to be
// handed to Vault.
func decodeSecureMap(data map[string]*secure.SecureString) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(data))
	for k, ss := range data {
		if ss == nil || ss.IsEmpty() {
			out[k] = ""
			continue
		}
		var decoded interface{}
		if err := ss.WithBytes(func(b []byte) error {
			return json.Unmarshal(b, &decoded)
		}); err != nil {
			return nil, fmt.Errorf("vault: decode secure value %q: %w", k, err)
		}
		out[k] = decoded
	}
	return out, nil
}

func destroySecureMap(data map[string]*secure.SecureString) {
	for _, ss := range data {
		ss.Destroy()
	}
}

// secureSecretFromAPI converts a Vault SDK *KVSecret into our SecureString-
// backed form. Each value is JSON-encoded, wrapped, and then nilled in the
// source map so the plaintext only survives in the enclaves.
func secureSecretFromAPI(in *vaultapi.KVSecret) (*KVSecret, error) {
	if in == nil {
		return nil, nil
	}
	out := &KVSecret{Data: make(map[string]*secure.SecureString, len(in.Data))}
	for k, v := range in.Data {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("vault: encode secret value %q: %w", k, err)
		}
		out.Data[k] = secure.NewSecureString(b)
		in.Data[k] = nil
	}
	if in.VersionMetadata != nil {
		out.VersionMetadata = &KVVersionMeta{
			Version:      in.VersionMetadata.Version,
			CreatedTime:  in.VersionMetadata.CreatedTime,
			DeletionTime: in.VersionMetadata.DeletionTime,
			Destroyed:    in.VersionMetadata.Destroyed,
		}
	}
	if len(in.CustomMetadata) > 0 {
		out.CustomMetadata = make(map[string]string, len(in.CustomMetadata))
		for k, v := range in.CustomMetadata {
			if s, ok := v.(string); ok {
				out.CustomMetadata[k] = s
			} else {
				out.CustomMetadata[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	return out, nil
}

// secureSecretFromV1Data wraps a raw v1 data map. The source map's values
// are nilled after wrapping.
func secureSecretFromV1Data(data map[string]interface{}) (*KVSecret, error) {
	out := &KVSecret{Data: make(map[string]*secure.SecureString, len(data))}
	for k, v := range data {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("vault: encode v1 value %q: %w", k, err)
		}
		out.Data[k] = secure.NewSecureString(b)
		data[k] = nil
	}
	return out, nil
}

// listEntriesFromKeys converts the "keys" list returned by the LIST
// endpoint into a sorted slice of KVEntry.
func listEntriesFromKeys(raw []string) []KVEntry {
	out := make([]KVEntry, 0, len(raw))
	for _, k := range raw {
		if strings.HasSuffix(k, "/") {
			out = append(out, KVEntry{Name: strings.TrimSuffix(k, "/"), IsFolder: true})
		} else {
			out = append(out, KVEntry{Name: k, IsFolder: false})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsFolder != out[j].IsFolder {
			return out[i].IsFolder
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ---- KV v1 implementation -------------------------------------------------

type kvV1Engine struct {
	c     *vaultapi.Client
	mount string
}

func newKVv1Engine(c *vaultapi.Client, mount string) *kvV1Engine {
	return &kvV1Engine{c: c, mount: strings.Trim(mount, "/")}
}

func (e *kvV1Engine) Version() int  { return 1 }
func (e *kvV1Engine) Mount() string { return e.mount }
func (e *kvV1Engine) listPath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return e.mount + "/"
	}
	return e.mount + "/" + p + "/"
}
func (e *kvV1Engine) dataPath(p string) string {
	return e.mount + "/" + strings.Trim(p, "/")
}

func (e *kvV1Engine) List(ctx context.Context, path string) ([]KVEntry, error) {
	resp, err := e.c.Logical().ListWithContext(ctx, e.listPath(path))
	if err != nil {
		return nil, classifyError(err)
	}
	if resp == nil || resp.Data == nil {
		return []KVEntry{}, nil
	}
	rawKeys, _ := resp.Data["keys"].([]interface{})
	keys := make([]string, 0, len(rawKeys))
	for _, k := range rawKeys {
		if s, ok := k.(string); ok {
			keys = append(keys, s)
		}
	}
	return listEntriesFromKeys(keys), nil
}

func (e *kvV1Engine) Get(ctx context.Context, path string) (*KVSecret, error) {
	resp, err := e.c.Logical().ReadWithContext(ctx, e.dataPath(path))
	if err != nil {
		return nil, classifyError(err)
	}
	if resp == nil || resp.Data == nil {
		return nil, ErrNotFound
	}
	return secureSecretFromV1Data(resp.Data)
}

func (e *kvV1Engine) Put(ctx context.Context, path string, data map[string]interface{}) error {
	_, err := e.c.Logical().WriteWithContext(ctx, e.dataPath(path), data)
	if err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV1Engine) Delete(ctx context.Context, path string) error {
	_, err := e.c.Logical().DeleteWithContext(ctx, e.dataPath(path))
	if err != nil {
		return classifyError(err)
	}
	return nil
}

// ---- KV v2 implementation -------------------------------------------------

type kvV2Engine struct {
	c      *vaultapi.Client
	mount  string
	helper *vaultapi.KVv2
}

func newKVv2Engine(c *vaultapi.Client, mount string) *kvV2Engine {
	m := strings.Trim(mount, "/")
	return &kvV2Engine{c: c, mount: m, helper: c.KVv2(m)}
}

func (e *kvV2Engine) Version() int  { return 2 }
func (e *kvV2Engine) Mount() string { return e.mount }

func (e *kvV2Engine) metadataListPath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return e.mount + "/metadata/"
	}
	return e.mount + "/metadata/" + p + "/"
}

func (e *kvV2Engine) subkeysPath(p string) string {
	return e.mount + "/subkeys/" + strings.Trim(p, "/")
}

func (e *kvV2Engine) List(ctx context.Context, path string) ([]KVEntry, error) {
	resp, err := e.c.Logical().ListWithContext(ctx, e.metadataListPath(path))
	if err != nil {
		return nil, classifyError(err)
	}
	if resp == nil || resp.Data == nil {
		return []KVEntry{}, nil
	}
	rawKeys, _ := resp.Data["keys"].([]interface{})
	keys := make([]string, 0, len(rawKeys))
	for _, k := range rawKeys {
		if s, ok := k.(string); ok {
			keys = append(keys, s)
		}
	}
	return listEntriesFromKeys(keys), nil
}

func (e *kvV2Engine) Get(ctx context.Context, path string) (*KVSecret, error) {
	s, err := e.helper.Get(ctx, strings.Trim(path, "/"))
	if err != nil {
		return nil, classifyError(err)
	}
	return secureSecretFromAPI(s)
}

func (e *kvV2Engine) GetVersion(ctx context.Context, path string, version int) (*KVSecret, error) {
	s, err := e.helper.GetVersion(ctx, strings.Trim(path, "/"), version)
	if err != nil {
		return nil, classifyError(err)
	}
	return secureSecretFromAPI(s)
}

func (e *kvV2Engine) GetVersionsList(ctx context.Context, path string) ([]KVVersionMeta, error) {
	vs, err := e.helper.GetVersionsAsList(ctx, strings.Trim(path, "/"))
	if err != nil {
		return nil, classifyError(err)
	}
	out := make([]KVVersionMeta, 0, len(vs))
	for _, v := range vs {
		out = append(out, KVVersionMeta{
			Version:      v.Version,
			CreatedTime:  v.CreatedTime,
			DeletionTime: v.DeletionTime,
			Destroyed:    v.Destroyed,
		})
	}
	return out, nil
}

func (e *kvV2Engine) GetMetadata(ctx context.Context, path string) (*KVMetadata, error) {
	m, err := e.helper.GetMetadata(ctx, strings.Trim(path, "/"))
	if err != nil {
		return nil, classifyError(err)
	}
	if m == nil {
		return nil, ErrNotFound
	}
	out := &KVMetadata{
		CASRequired:        m.CASRequired,
		CreatedTime:        m.CreatedTime,
		CurrentVersion:     m.CurrentVersion,
		DeleteVersionAfter: m.DeleteVersionAfter,
		MaxVersions:        m.MaxVersions,
		OldestVersion:      m.OldestVersion,
		UpdatedTime:        m.UpdatedTime,
		Versions:           make(map[int]KVVersionMeta, len(m.Versions)),
	}
	if len(m.CustomMetadata) > 0 {
		out.CustomMetadata = make(map[string]string, len(m.CustomMetadata))
		for k, v := range m.CustomMetadata {
			if s, ok := v.(string); ok {
				out.CustomMetadata[k] = s
			} else {
				out.CustomMetadata[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	for k, v := range m.Versions {
		// Vault returns version keys as strings in JSON; the SDK already
		// decoded them into the typed Versions map keyed by string.
		var n int
		if _, err := fmt.Sscanf(k, "%d", &n); err == nil {
			out.Versions[n] = KVVersionMeta{
				Version:      v.Version,
				CreatedTime:  v.CreatedTime,
				DeletionTime: v.DeletionTime,
				Destroyed:    v.Destroyed,
			}
		}
	}
	return out, nil
}

func (e *kvV2Engine) GetSubkeys(ctx context.Context, path string, version int, depth int) ([]string, error) {
	params := map[string][]string{}
	if version > 0 {
		params["version"] = []string{fmt.Sprintf("%d", version)}
	}
	if depth > 0 {
		params["depth"] = []string{fmt.Sprintf("%d", depth)}
	}
	resp, err := e.c.Logical().ReadWithDataWithContext(ctx, e.subkeysPath(path), params)
	if err != nil {
		return nil, classifyError(err)
	}
	if resp == nil || resp.Data == nil {
		return nil, ErrNotFound
	}
	tree, _ := resp.Data["subkeys"].(map[string]interface{})
	out := make([]string, 0, len(tree))
	flattenSubkeys("", tree, &out)
	sort.Strings(out)
	return out, nil
}

func flattenSubkeys(prefix string, tree map[string]interface{}, out *[]string) {
	for k, v := range tree {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		if child, ok := v.(map[string]interface{}); ok && len(child) > 0 {
			flattenSubkeys(full, child, out)
			continue
		}
		*out = append(*out, full)
	}
}

func (e *kvV2Engine) Put(ctx context.Context, path string, data map[string]interface{}) error {
	_, err := e.helper.Put(ctx, strings.Trim(path, "/"), data)
	if err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV2Engine) PutCAS(ctx context.Context, path string, data map[string]interface{}, cas *int) error {
	var opts []vaultapi.KVOption
	if cas != nil {
		opts = append(opts, vaultapi.WithCheckAndSet(*cas))
	}
	_, err := e.helper.Put(ctx, strings.Trim(path, "/"), data, opts...)
	if err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV2Engine) Patch(ctx context.Context, path string, data map[string]interface{}, cas *int) error {
	// Use the read-write merge strategy so callers only need
	// read+update on data/<path> rather than the separate "patch"
	// capability (which the HTTP PATCH method requires). This matches
	// what most operators grant in practice.
	opts := []vaultapi.KVOption{vaultapi.WithMergeMethod(vaultapi.KVMergeMethodReadWrite)}
	if cas != nil {
		opts = append(opts, vaultapi.WithCheckAndSet(*cas))
	}
	_, err := e.helper.Patch(ctx, strings.Trim(path, "/"), data, opts...)
	if err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV2Engine) Delete(ctx context.Context, path string) error {
	if err := e.helper.Delete(ctx, strings.Trim(path, "/")); err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV2Engine) DeleteVersions(ctx context.Context, path string, versions []int) error {
	if err := e.helper.DeleteVersions(ctx, strings.Trim(path, "/"), versions); err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV2Engine) UndeleteVersions(ctx context.Context, path string, versions []int) error {
	if err := e.helper.Undelete(ctx, strings.Trim(path, "/"), versions); err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV2Engine) DestroyVersions(ctx context.Context, path string, versions []int) error {
	if err := e.helper.Destroy(ctx, strings.Trim(path, "/"), versions); err != nil {
		return classifyError(err)
	}
	return nil
}

func (e *kvV2Engine) DeleteAllVersions(ctx context.Context, path string) error {
	if err := e.helper.DeleteMetadata(ctx, strings.Trim(path, "/")); err != nil {
		return classifyError(err)
	}
	return nil
}
