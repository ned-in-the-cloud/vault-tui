//go:build integration

package vault_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ned1313/vault-tui/internal/vault"
)

func TestIntegrationKVv2VersionLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := authedClient(t, ctx)

	mount := "kv-v2"
	mounts, err := c.ListMounts(ctx)
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if _, ok := mounts[mount+"/"]; !ok {
		t.Skipf("mount %q not present; run scripts/seed-dev.ps1 first", mount)
	}

	e := c.KVv2(mount)
	path := "vault-tui-itest/version-lifecycle"
	defer e.DeleteAllVersions(ctx, path)

	if err := e.Put(ctx, path, map[string]interface{}{"k": "v1"}); err != nil {
		t.Fatalf("Put v1: %v", err)
	}
	if err := e.Put(ctx, path, map[string]interface{}{"k": "v2"}); err != nil {
		t.Fatalf("Put v2: %v", err)
	}
	if err := e.Put(ctx, path, map[string]interface{}{"k": "v3"}); err != nil {
		t.Fatalf("Put v3: %v", err)
	}

	if err := e.DeleteVersions(ctx, path, []int{2}); err != nil {
		t.Fatalf("DeleteVersions v2: %v", err)
	}
	if err := e.DestroyVersions(ctx, path, []int{1}); err != nil {
		t.Fatalf("DestroyVersions v1: %v", err)
	}

	versions, err := e.GetVersionsList(ctx, path)
	if err != nil {
		t.Fatalf("GetVersionsList: %v", err)
	}
	if len(versions) < 3 {
		t.Fatalf("expected at least 3 versions, got %d", len(versions))
	}
	byVersion := map[int]vault.KVVersionMeta{}
	for _, v := range versions {
		byVersion[v.Version] = v
	}

	if byVersion[1].Version == 0 || !byVersion[1].Destroyed {
		t.Fatalf("version 1 expected destroyed, got %+v", byVersion[1])
	}
	if byVersion[2].Version == 0 || byVersion[2].DeletionTime.IsZero() || byVersion[2].Destroyed {
		t.Fatalf("version 2 expected soft-deleted, got %+v", byVersion[2])
	}
	if byVersion[3].Version == 0 || byVersion[3].Destroyed || !byVersion[3].DeletionTime.IsZero() {
		t.Fatalf("version 3 expected active, got %+v", byVersion[3])
	}

	if err := e.UndeleteVersions(ctx, path, []int{2}); err != nil {
		t.Fatalf("UndeleteVersions v2: %v", err)
	}
	versionsAfterUndelete, err := e.GetVersionsList(ctx, path)
	if err != nil {
		t.Fatalf("GetVersionsList after undelete: %v", err)
	}
	var v2 vault.KVVersionMeta
	for _, v := range versionsAfterUndelete {
		if v.Version == 2 {
			v2 = v
			break
		}
	}
	if v2.Version == 0 || !v2.DeletionTime.IsZero() {
		t.Fatalf("version 2 expected active after undelete, got %+v", v2)
	}

	// Destroyed versions should remain destroyed even if undelete is attempted.
	_ = e.UndeleteVersions(ctx, path, []int{1})
	versionsAfterUndeleteDestroyed, err := e.GetVersionsList(ctx, path)
	if err != nil {
		t.Fatalf("GetVersionsList after undelete destroyed: %v", err)
	}
	var v1 vault.KVVersionMeta
	for _, v := range versionsAfterUndeleteDestroyed {
		if v.Version == 1 {
			v1 = v
			break
		}
	}
	if v1.Version == 0 || !v1.Destroyed {
		t.Fatalf("version 1 should remain destroyed, got %+v", v1)
	}
}

func TestIntegrationKVv2RollbackCreatesNewCurrentVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := authedClient(t, ctx)

	mount := "kv-v2"
	mounts, err := c.ListMounts(ctx)
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if _, ok := mounts[mount+"/"]; !ok {
		t.Skipf("mount %q not present; run scripts/seed-dev.ps1 first", mount)
	}

	e := c.KVv2(mount)
	path := "vault-tui-itest/rollback"
	defer e.DeleteAllVersions(ctx, path)

	if err := e.Put(ctx, path, map[string]interface{}{"k": "v1"}); err != nil {
		t.Fatalf("Put v1: %v", err)
	}
	if err := e.Put(ctx, path, map[string]interface{}{"k": "v2"}); err != nil {
		t.Fatalf("Put v2: %v", err)
	}

	mdBefore, err := e.GetMetadata(ctx, path)
	if err != nil {
		t.Fatalf("GetMetadata before rollback: %v", err)
	}
	if mdBefore.CurrentVersion != 2 {
		t.Fatalf("current before rollback = %d, want 2", mdBefore.CurrentVersion)
	}

	v1, err := e.GetVersion(ctx, path, 1)
	if err != nil {
		t.Fatalf("GetVersion(1): %v", err)
	}
	defer v1.Destroy()
	if err := vault.PutFromSecureV2(ctx, e, path, v1.Data, nil); err != nil {
		t.Fatalf("rollback write: %v", err)
	}

	mdAfter, err := e.GetMetadata(ctx, path)
	if err != nil {
		t.Fatalf("GetMetadata after rollback: %v", err)
	}
	if mdAfter.CurrentVersion <= mdBefore.CurrentVersion {
		t.Fatalf("expected new current version > %d, got %d", mdBefore.CurrentVersion, mdAfter.CurrentVersion)
	}

	got, err := e.Get(ctx, path)
	if err != nil {
		t.Fatalf("Get latest after rollback: %v", err)
	}
	defer got.Destroy()
	if v := decodeString(t, got.Data["k"]); v != "v1" {
		t.Fatalf("latest value after rollback = %q, want v1", v)
	}
}

func TestIntegrationKVv2DeleteAllRemovesMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := authedClient(t, ctx)

	mount := "kv-v2"
	mounts, err := c.ListMounts(ctx)
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if _, ok := mounts[mount+"/"]; !ok {
		t.Skipf("mount %q not present; run scripts/seed-dev.ps1 first", mount)
	}

	e := c.KVv2(mount)
	path := "vault-tui-itest/delete-all"
	defer e.DeleteAllVersions(ctx, path)

	if err := e.Put(ctx, path, map[string]interface{}{"k": "v1"}); err != nil {
		t.Fatalf("Put v1: %v", err)
	}
	if err := e.Put(ctx, path, map[string]interface{}{"k": "v2"}); err != nil {
		t.Fatalf("Put v2: %v", err)
	}

	if err := e.DeleteAllVersions(ctx, path); err != nil {
		t.Fatalf("DeleteAllVersions: %v", err)
	}

	_, err = e.GetMetadata(ctx, path)
	if err == nil {
		t.Fatalf("expected metadata to be gone after delete-all")
	}
	if !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete-all, got %v", err)
	}
}
