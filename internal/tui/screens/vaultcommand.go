package screens

import (
	"fmt"
	"strings"

	"github.com/ned1313/vault-tui/internal/config"
)

// This file collects the VaultCommand() implementations for each
// screen so the equivalent CLI command overlay (alt+x) is centralized
// and easy to keep in sync with the underlying operations.

// vaultMountFlag returns the mount flag for v2 commands or the
// concatenated path for v1, where the mount and path are combined.
func vaultMountFlag(mount string, version int) (flag string, joinedPrefix string) {
	mount = strings.TrimSuffix(mount, "/")
	if version == 2 {
		return fmt.Sprintf("-mount=%s ", mount), ""
	}
	return "", mount + "/"
}

func (s *SecretListScreen) VaultCommand() string {
	flag, prefix := vaultMountFlag(s.mount.Path, s.version)
	p := strings.TrimPrefix(s.path, "/")
	target := strings.TrimSuffix(prefix+p, "/")
	if target == "" {
		target = strings.TrimSuffix(s.mount.Path, "/")
	}
	if flag != "" {
		return fmt.Sprintf("vault kv list %s%s", flag, p)
	}
	return fmt.Sprintf("vault kv list %s", target)
}

func (s *SecretViewScreen) VaultCommand() string {
	flag, prefix := vaultMountFlag(s.mount.Path, s.version)
	p := strings.TrimPrefix(s.path, "/")
	if flag != "" {
		return fmt.Sprintf("vault kv get %s%s", flag, p)
	}
	return fmt.Sprintf("vault kv get %s%s", prefix, p)
}

func (s *SecretEditScreen) VaultCommand() string {
	flag, prefix := vaultMountFlag(s.mount.Path, s.version)
	p := strings.TrimPrefix(strings.TrimSpace(s.pathInput.Value()), "/")
	verb := "put"
	if s.patchMode && !s.create && s.version == 2 {
		verb = "patch"
	}
	// Show key names only — never echo plaintext values into the
	// overlay so the toggle is safe to use while typing secrets.
	var keys []string
	for i := range s.pairs {
		k := strings.TrimSpace(s.pairs[i].key.Value())
		if k != "" {
			keys = append(keys, k+"=...")
		}
	}
	args := strings.Join(keys, " ")
	if args != "" {
		args = " " + args
	}
	if flag != "" {
		return fmt.Sprintf("vault kv %s %s%s%s", verb, flag, p, args)
	}
	return fmt.Sprintf("vault kv %s %s%s%s", verb, prefix, p, args)
}

func (s *SecretVersionsScreen) VaultCommand() string {
	mount := strings.TrimSuffix(s.mount.Path, "/")
	p := strings.TrimPrefix(s.path, "/")
	return fmt.Sprintf("vault kv metadata get -mount=%s %s", mount, p)
}

func (s *EngineListScreen) VaultCommand() string {
	return "vault secrets list"
}

func (s *DashboardScreen) VaultCommand() string {
	return "vault token lookup"
}

func (s *TokenDetailScreen) VaultCommand() string {
	return "vault token lookup"
}

func (s *SecretExportScreen) VaultCommand() string {
	flag, prefix := vaultMountFlag(s.mount.Path, s.version)
	p := strings.TrimPrefix(s.path, "/")
	if flag != "" {
		return fmt.Sprintf("vault kv get -format=json %s%s", flag, p)
	}
	return fmt.Sprintf("vault kv get -format=json %s%s", prefix, p)
}

func (s *AuthScreen) VaultCommand() string {
	if s.step != stepFillCredentials || s.credentialView == nil {
		return "vault login"
	}
	opt := authMethodOptions[s.selected]
	switch opt.methodID {
	case config.AuthMethodToken:
		return "vault login <token>"
	case config.AuthMethodUserPass:
		user := credentialFieldValue(s.credentialView, "username")
		if user == "" {
			user = "<username>"
		}
		return fmt.Sprintf("vault login -method=userpass username=%s", user)
	case config.AuthMethodLDAP:
		user := credentialFieldValue(s.credentialView, "username")
		if user == "" {
			user = "<username>"
		}
		return fmt.Sprintf("vault login -method=ldap username=%s", user)
	}
	return "vault login"
}

func credentialFieldValue(c *credentialForm, label string) string {
	for i, l := range c.labels {
		if l == label && i < len(c.inputs) {
			return strings.TrimSpace(c.inputs[i].Value())
		}
	}
	return ""
}
