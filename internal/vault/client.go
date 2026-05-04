// Package vault provides an abstraction over the official HashiCorp Vault
// Go client. All TUI screens interact with Vault exclusively through the
// Client interface defined here so that production and test code can be
// swapped freely.
package vault

import (
	"context"
	"time"
)

// HealthInfo summarizes the health status of the Vault server.
type HealthInfo struct {
	Initialized bool
	Sealed      bool
	Standby     bool
	Version     string
	ClusterName string
}

// MountInfo describes a single secrets engine mount.
type MountInfo struct {
	Path        string
	Type        string
	Description string
	Options     map[string]string
}

// TokenInfo summarizes the metadata for the active Vault token.
type TokenInfo struct {
	ID          string // raw token (sensitive; do not log)
	Accessor    string
	DisplayName string
	EntityID    string
	Policies    []string
	TTL         time.Duration
	CreationTTL time.Duration
	Renewable   bool
	Orphan      bool
	NumUses     int
	IssueTime   time.Time
	ExpireTime  time.Time
}

// Client is the production-and-mock-friendly Vault interface used by the
// TUI. Implementations must be safe for use from a single goroutine; the
// TUI serializes calls through tea commands.
type Client interface {
	SetAddress(addr string) error
	Address() string

	SetNamespace(ns string)
	Namespace() string

	SetCABundlePath(path string) error

	Health(ctx context.Context) (*HealthInfo, error)

	Login(ctx context.Context, method AuthMethod) (*TokenInfo, error)
	SetToken(token string)
	Token() string
	LookupToken(ctx context.Context) (*TokenInfo, error)
	RenewToken(ctx context.Context, increment int) (*TokenInfo, error)
	NewTokenWatcher(increment int) (*TokenWatcher, error)

	ListMounts(ctx context.Context) (map[string]*MountInfo, error)
	ListKV(ctx context.Context, mount string, version int, key string) ([]string, error)

	// KVv1 / KVv2 are placeholders for Phase 4 and return implementations
	// that satisfy the engine interfaces in kv.go (added later).
	KVv1(mount string) any
	KVv2(mount string) any
}
