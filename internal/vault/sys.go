package vault

import (
	"context"
	"path"
	"strings"
)

// KVVersion returns 1 or 2 for KV-type mounts, 0 otherwise. Vault
// reports version via the mount's options map; "kv" with no version
// option is treated as v1, matching server defaults.
func KVVersion(mi *MountInfo) int {
	if mi == nil {
		return 0
	}
	t := strings.ToLower(mi.Type)
	if t != "kv" && t != "generic" {
		return 0
	}
	if mi.Options != nil {
		if v, ok := mi.Options["version"]; ok {
			switch v {
			case "2":
				return 2
			case "1":
				return 1
			}
		}
	}
	return 1
}

// IsKV reports whether the mount is a KV engine (v1 or v2).
func IsKV(mi *MountInfo) bool { return KVVersion(mi) > 0 }

// ListKV lists keys at the given logical path beneath the mount.
// version selects the KV API surface (1 or 2). The result is the raw
// key listing as returned by Vault: folders end in "/", secrets do not.
//
// path may be empty (root of the mount) and is normalized — leading and
// trailing slashes are stripped before the request is built so callers
// can pass either form.
func (a *apiClient) ListKV(ctx context.Context, mount string, version int, key string) ([]string, error) {
	mount = strings.Trim(mount, "/")
	key = strings.Trim(key, "/")
	var listPath string
	switch version {
	case 2:
		listPath = path.Join(mount, "metadata", key)
	default:
		listPath = path.Join(mount, key)
	}
	secret, err := a.c.Logical().ListWithContext(ctx, listPath)
	if err != nil {
		return nil, classifyError(err)
	}
	if secret == nil || secret.Data == nil {
		return []string{}, nil
	}
	rawKeys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return []string{}, nil
	}
	out := make([]string, 0, len(rawKeys))
	for _, k := range rawKeys {
		if s, ok := k.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}
