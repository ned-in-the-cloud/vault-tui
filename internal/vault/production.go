package vault

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
)

// apiClient is the production *vaultapi.Client-backed implementation of
// Client. It is intentionally small: it stores the configured address,
// namespace, and CA bundle path on its own so they can be reused across
// reconfiguration calls without re-deriving from the underlying client.
type apiClient struct {
	cfg          *vaultapi.Config
	c            *vaultapi.Client
	addr         string
	namespace    string
	caBundlePath string
}

// NewClient constructs an apiClient with sensible defaults.
func NewClient() (Client, error) {
	cfg := vaultapi.DefaultConfig()
	if cfg.Error != nil {
		return nil, fmt.Errorf("vault: default config: %w", cfg.Error)
	}
	cfg.Timeout = 10 * time.Second
	c, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault: new client: %w", err)
	}
	return &apiClient{cfg: cfg, c: c, addr: c.Address()}, nil
}

func (a *apiClient) SetAddress(addr string) error {
	if addr == "" {
		return errors.New("vault: empty address")
	}
	u, err := url.Parse(addr)
	if err != nil {
		return fmt.Errorf("vault: parse address: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("vault: address must use http or https (got %q)", u.Scheme)
	}
	if err := a.c.SetAddress(addr); err != nil {
		return fmt.Errorf("vault: set address: %w", err)
	}
	a.addr = addr
	return nil
}

func (a *apiClient) Address() string { return a.addr }

func (a *apiClient) SetNamespace(ns string) {
	a.namespace = ns
	a.c.SetNamespace(ns)
}

func (a *apiClient) Namespace() string { return a.namespace }

func (a *apiClient) SetCABundlePath(path string) error {
	a.caBundlePath = path
	tlsCfg := &vaultapi.TLSConfig{}
	if path != "" {
		// Validate the file exists up front so we can surface a clear error.
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("vault: ca bundle: %w", err)
		}
		tlsCfg.CACert = path
	}
	if err := a.cfg.ConfigureTLS(tlsCfg); err != nil {
		return fmt.Errorf("vault: configure tls: %w", err)
	}
	return nil
}

func (a *apiClient) Health(ctx context.Context) (*HealthInfo, error) {
	resp, err := a.c.Sys().HealthWithContext(ctx)
	if err != nil {
		return nil, classifyError(err)
	}
	return &HealthInfo{
		Initialized: resp.Initialized,
		Sealed:      resp.Sealed,
		Standby:     resp.Standby,
		Version:     resp.Version,
		ClusterName: resp.ClusterName,
	}, nil
}

func (a *apiClient) Login(ctx context.Context, method AuthMethod) (*TokenInfo, error) {
	if method == nil {
		return nil, errors.New("vault: nil auth method")
	}
	token, _, err := method.Login(ctx, a.c)
	if err != nil {
		return nil, err
	}
	a.c.SetToken(token)
	return a.LookupToken(ctx)
}

func (a *apiClient) SetToken(token string) { a.c.SetToken(token) }
func (a *apiClient) Token() string         { return a.c.Token() }

func (a *apiClient) LookupToken(ctx context.Context) (*TokenInfo, error) {
	sec, err := a.c.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil {
		return nil, classifyError(err)
	}
	if sec == nil || sec.Data == nil {
		return nil, errors.New("vault: empty token lookup response")
	}
	return tokenInfoFromMap(a.c.Token(), sec.Data), nil
}

func (a *apiClient) RenewToken(ctx context.Context, increment int) (*TokenInfo, error) {
	sec, err := a.c.Auth().Token().RenewSelfWithContext(ctx, increment)
	if err != nil {
		return nil, classifyError(err)
	}
	if sec == nil || sec.Auth == nil {
		return nil, errors.New("vault: empty renew response")
	}
	// The renew response contains Auth, not Data; resolve full info via
	// a follow-up lookup so callers get a consistent shape.
	return a.LookupToken(ctx)
}

func (a *apiClient) ListMounts(ctx context.Context) (map[string]*MountInfo, error) {
	// Try the privileged sys/mounts endpoint first; fall back to
	// sys/internal/ui/mounts for tokens that lack `read` on
	// sys/mounts (the same fallback the Vault CLI uses to render
	// `vault secrets list` for non-root users).
	if mounts, err := a.c.Sys().ListMountsWithContext(ctx); err == nil {
		return convertMounts(mounts), nil
	}
	return a.listUIMounts(ctx)
}

// listUIMounts queries sys/internal/ui/mounts which returns only the
// mounts visible to the current token. The response shape is:
//
//	{ "data": { "secret": { "mount-path/": { "type": "kv", "options": {...}, ... } } } }
func (a *apiClient) listUIMounts(ctx context.Context) (map[string]*MountInfo, error) {
	resp, err := a.c.Logical().ReadWithContext(ctx, "sys/internal/ui/mounts")
	if err != nil {
		return nil, classifyError(err)
	}
	if resp == nil || resp.Data == nil {
		return map[string]*MountInfo{}, nil
	}
	secretSection, _ := resp.Data["secret"].(map[string]interface{})
	out := make(map[string]*MountInfo, len(secretSection))
	for path, raw := range secretSection {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		mi := &MountInfo{Path: path}
		if v, ok := entry["type"].(string); ok {
			mi.Type = v
		}
		if v, ok := entry["description"].(string); ok {
			mi.Description = v
		}
		if opts, ok := entry["options"].(map[string]interface{}); ok && len(opts) > 0 {
			mi.Options = make(map[string]string, len(opts))
			for k, v := range opts {
				if s, ok := v.(string); ok {
					mi.Options[k] = s
				}
			}
		}
		out[path] = mi
	}
	return out, nil
}

// convertMounts adapts the privileged Sys().ListMounts response.
func convertMounts(mounts map[string]*vaultapi.MountOutput) map[string]*MountInfo {
	out := make(map[string]*MountInfo, len(mounts))
	for path, m := range mounts {
		mi := &MountInfo{
			Path:        path,
			Type:        m.Type,
			Description: m.Description,
		}
		if m.Options != nil {
			mi.Options = make(map[string]string, len(m.Options))
			for k, v := range m.Options {
				mi.Options[k] = v
			}
		}
		out[path] = mi
	}
	return out
}

func (a *apiClient) KVv1(mount string) KVEngine   { return newKVv1Engine(a.c, mount) }
func (a *apiClient) KVv2(mount string) KVv2Engine { return newKVv2Engine(a.c, mount) }

// tokenInfoFromMap parses a Vault token lookup response payload.
func tokenInfoFromMap(rawToken string, data map[string]interface{}) *TokenInfo {
	ti := &TokenInfo{ID: rawToken}
	if v, ok := data["accessor"].(string); ok {
		ti.Accessor = v
	}
	if v, ok := data["display_name"].(string); ok {
		ti.DisplayName = v
	}
	if v, ok := data["entity_id"].(string); ok {
		ti.EntityID = v
	}
	if v, ok := data["policies"].([]interface{}); ok {
		for _, p := range v {
			if s, ok := p.(string); ok {
				ti.Policies = append(ti.Policies, s)
			}
		}
	}
	ti.TTL = readDurationSeconds(data, "ttl")
	ti.CreationTTL = readDurationSeconds(data, "creation_ttl")
	if v, ok := data["renewable"].(bool); ok {
		ti.Renewable = v
	}
	if v, ok := data["orphan"].(bool); ok {
		ti.Orphan = v
	}
	if v, ok := data["num_uses"].(int); ok {
		ti.NumUses = v
	}
	if v, ok := data["issue_time"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			ti.IssueTime = t
		}
	}
	if v, ok := data["expire_time"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			ti.ExpireTime = t
		}
	}
	return ti
}

func readDurationSeconds(data map[string]interface{}, key string) time.Duration {
	v, ok := data[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return time.Duration(n) * time.Second
	case int64:
		return time.Duration(n) * time.Second
	case float64:
		return time.Duration(n) * time.Second
	}
	// json.Number is the most common shape coming out of the api package.
	type number interface{ Int64() (int64, error) }
	if jn, ok := v.(number); ok {
		if i, err := jn.Int64(); err == nil {
			return time.Duration(i) * time.Second
		}
	}
	return 0
}
