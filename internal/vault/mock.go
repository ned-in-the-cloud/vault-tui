package vault

import (
	"context"
	"errors"
)

// MockClient is a configurable in-memory Client implementation used by
// tests. Each method delegates to a function field; unset fields cause
// the method to return errMockNotConfigured. State for SetAddress,
// SetNamespace, SetCABundlePath, SetToken is captured directly.
type MockClient struct {
	addr         string
	namespace    string
	caBundlePath string
	token        string

	HealthFn      func(ctx context.Context) (*HealthInfo, error)
	LoginFn       func(ctx context.Context, method AuthMethod) (*TokenInfo, error)
	LookupTokenFn func(ctx context.Context) (*TokenInfo, error)
	RenewTokenFn  func(ctx context.Context, increment int) (*TokenInfo, error)
	WatcherFn     func(increment int) (*TokenWatcher, error)
	ListMountsFn  func(ctx context.Context) (map[string]*MountInfo, error)
	ListKVFn      func(ctx context.Context, mount string, version int, key string) ([]string, error)
	KVv1Fn        func(mount string) any
	KVv2Fn        func(mount string) any
}

var errMockNotConfigured = errors.New("mock: function not configured")

func (m *MockClient) SetAddress(addr string) error   { m.addr = addr; return nil }
func (m *MockClient) Address() string                { return m.addr }
func (m *MockClient) SetNamespace(ns string)         { m.namespace = ns }
func (m *MockClient) Namespace() string              { return m.namespace }
func (m *MockClient) SetCABundlePath(p string) error { m.caBundlePath = p; return nil }
func (m *MockClient) SetToken(t string)              { m.token = t }
func (m *MockClient) Token() string                  { return m.token }

func (m *MockClient) Health(ctx context.Context) (*HealthInfo, error) {
	if m.HealthFn == nil {
		return nil, errMockNotConfigured
	}
	return m.HealthFn(ctx)
}

func (m *MockClient) Login(ctx context.Context, method AuthMethod) (*TokenInfo, error) {
	if m.LoginFn == nil {
		return nil, errMockNotConfigured
	}
	return m.LoginFn(ctx, method)
}

func (m *MockClient) LookupToken(ctx context.Context) (*TokenInfo, error) {
	if m.LookupTokenFn == nil {
		return nil, errMockNotConfigured
	}
	return m.LookupTokenFn(ctx)
}

func (m *MockClient) RenewToken(ctx context.Context, increment int) (*TokenInfo, error) {
	if m.RenewTokenFn == nil {
		return nil, errMockNotConfigured
	}
	return m.RenewTokenFn(ctx, increment)
}

func (m *MockClient) NewTokenWatcher(increment int) (*TokenWatcher, error) {
	if m.WatcherFn == nil {
		return nil, errMockNotConfigured
	}
	return m.WatcherFn(increment)
}

func (m *MockClient) ListMounts(ctx context.Context) (map[string]*MountInfo, error) {
	if m.ListMountsFn == nil {
		return nil, errMockNotConfigured
	}
	return m.ListMountsFn(ctx)
}

func (m *MockClient) ListKV(ctx context.Context, mount string, version int, key string) ([]string, error) {
	if m.ListKVFn == nil {
		return nil, errMockNotConfigured
	}
	return m.ListKVFn(ctx, mount, version, key)
}

func (m *MockClient) KVv1(mount string) any {
	if m.KVv1Fn == nil {
		return nil
	}
	return m.KVv1Fn(mount)
}

func (m *MockClient) KVv2(mount string) any {
	if m.KVv2Fn == nil {
		return nil
	}
	return m.KVv2Fn(mount)
}

// Compile-time interface check.
var _ Client = (*MockClient)(nil)
