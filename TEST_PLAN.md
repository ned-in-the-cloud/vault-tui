# Vault TUI Test Plan

This document is a comprehensive manual and automation-oriented test matrix for Vault TUI.

## Automation Status

The matrix below maps each scenario group to automated coverage in this repository.

Legend:

- `Automated`: implemented and runnable with current test harness
- `Partial`: partly automated; additional harness work needed
- `Planned`: not yet automated

| Group | Status | Automated By | Run Command |
| --- | --- | --- | --- |
| A. Version view correctness | Automated | `internal/tui/screens/secretversions_test.go` + `internal/vault/kv_version_integration_test.go` | `go test ./internal/tui/screens/...` and `go test -tags=integration ./internal/vault/...` |
| B. Delete/undelete/destroy specific version | Automated | `internal/vault/kv_version_integration_test.go` | `go test -tags=integration ./internal/vault/...` |
| C. Rollback semantics | Automated | `internal/tui/screens/secretversions_test.go` + `internal/vault/kv_version_integration_test.go` | `go test ./internal/tui/screens/...` and `go test -tags=integration ./internal/vault/...` |
| D. Delete-all and destroy-all flows | Partial | `internal/tui/screens/secretversions_test.go` + `internal/vault/kv_version_integration_test.go` | `go test ./internal/tui/screens/...` and `go test -tags=integration ./internal/vault/...` |
| E. Permission-path behavior | Partial | screen/unit coverage for error handling; integration policy-token suite pending | `go test ./internal/tui/screens/...` |
| F. Browsing/direct-path recovery | Automated | `internal/tui/screens/secretlist_test.go` | `go test ./internal/tui/screens/...` |
| G. Re-auth and in-progress state | Automated | `internal/tui/screens/secretedit_test.go` + `internal/tui/screens/auth_reauth_test.go` | `go test ./internal/tui/screens/...` |
| H. Footer/token behavior | Partial | `internal/tui/statusbar_test.go` + `internal/tui/app_test.go` | `go test ./internal/tui/...` |
| I. Security/no-plaintext expectations | Partial | export and masking related unit tests in screens/secure/config packages | `go test ./...` |

## Fully Automated Test Commands

Run all fast unit/screen tests:

```bash
go test ./...
```

Run integration flows (requires test Vault server and seed data):

```bash
go test -tags=integration ./internal/vault/...
```

Run everything together:

```bash
go test ./...; go test -tags=integration ./internal/vault/...
```

## Scope

- KV v1 and KV v2 browsing and secret operations
- Versioned workflows (delete, undelete, destroy, rollback, delete-all)
- Auth/session resilience (invalid token, re-auth, auto-renew)
- Policy/permission behavior and UX for denied actions
- Security expectations (no plaintext leakage paths)

## Test Environment

- Vault dev or test server with seeded data and both mounts:
  - `kv-v1/`
  - `kv-v2/`
- Multiple policies/tokens to validate permission outcomes (see policy matrix below)
- `vault-tui` built from current branch
- Optional helper:
  - `vault kv metadata get kv-v2/<path>` for version state verification
  - `vault kv get -version=<n> kv-v2/<path>` for rollback verification

## Policy Matrix (KV v2)

Use these policy profiles to validate behavior across capability sets.

### P0: Full KV v2 admin

Expected capabilities on:

- `kv-v2/data/*`: create, read, update, delete
- `kv-v2/metadata/*`: read, list, delete
- `kv-v2/delete/*`: update
- `kv-v2/undelete/*`: update
- `kv-v2/destroy/*`: update
- `sys/capabilities-self`: update

### P1: Standard editor (no version-admin)

Expected capabilities on:

- `kv-v2/data/*`: create, read, update, delete
- `kv-v2/metadata/*`: read, list
- no access to `delete/`, `undelete/`, `destroy/`, `metadata delete`

### P2: Version-admin only

Expected capabilities on:

- `kv-v2/data/*`: read
- `kv-v2/metadata/*`: read, list
- `kv-v2/delete/*`: update
- `kv-v2/undelete/*`: update
- `kv-v2/destroy/*`: update

### P3: Browsing-limited (direct-path recovery)

Expected capabilities pattern example:

- list denied on broad parent path (for browsing)
- direct read allowed for specific known path(s)

## Seed Data

Recommended KV v2 test target:

- path: `bob/api/keys`
- create at least 3 versions with distinct content
- soft-delete one version and destroy another for visibility checks

Recommended KV v1 target:

- path: `alice/service/creds`

## Manual Test Cases

## A. Version view correctness (KV v2)

- [ ] A-01 Open versions view for `kv-v2/bob/api/keys`.
  - Expected: all versions are listed, not only current.
  - Expected columns: version, created, deleted, status.
- [ ] A-02 Verify status mapping:
  - Active non-current shows `Active`
  - Current active shows `Current`
  - Soft-deleted shows `Deleted` and deleted timestamp
  - Destroyed shows `Destroyed`
- [ ] A-03 Refresh versions (`R`) after external Vault CLI action.
  - Expected: list and status reflect latest server state.

## B. Delete/undelete/destroy specific version (KV v2)

- [ ] B-01 Delete a specific non-current version (`d`).
  - Expected: success message and post-refresh status `Deleted`.
  - Verify with `vault kv metadata get kv-v2/bob/api/keys`.
- [ ] B-02 Undelete a soft-deleted version (`u`).
  - Expected: status returns to `Active`.
- [ ] B-03 Destroy a specific version (`D`).
  - Expected: status `Destroyed` (irreversible).
- [ ] B-04 Attempt undelete on destroyed version.
  - Expected: operation fails with clear error.

## C. Rollback semantics (KV v2)

- [ ] C-01 Roll back from version N (`r`).
  - Expected: new latest version is created with content from N.
  - Verify data equality via `vault kv get`.
- [ ] C-02 Attempt rollback from destroyed version.
  - Expected: blocked with explanatory status message.

## D. Delete-all and destroy-all flows

- [ ] D-01 Delete all (`a`) with warning confirmation.
  - Expected: metadata/data removed; path no longer version-browsable.
- [ ] D-02 Destroy all (`A`) with destructive confirmation.
  - Expected: all versions destroyed where applicable.
- [ ] D-03 Confirmation preference behavior:
  - non-destructive skips can bypass dialog when enabled
  - destructive always requires explicit DESTROY gate

## E. Permission-path behavior (key issue you reported)

These cases are intended to isolate policy path mismatches for version ops.

- [ ] E-01 Under P1 (no `delete/*`), try deleting version `2` in Versions view.
  - Expected: permission denied.
  - Expected UX: clear error shown and no false success state.
- [ ] E-02 Under same token, delete latest version using the Vault CLI
  - Expected: may succeed if policy allows `data/* delete` but not `delete/* update`.
  - Confirms endpoint capability mismatch rather than app transport bug.
- [ ] E-03 Under P0 or P2, retry version delete/undelete/destroy.
  - Expected: operations succeed.

## F. Browsing and direct-path recovery

- [ ] F-01 Deny list on parent path; allow read on known direct path.
- [ ] F-02 In Secret List, trigger permission denied while browsing.
  - Expected: recovery action offers direct path option.
- [ ] F-03 Enter direct path and navigate.
  - Expected: prompt closes and opens target path screen.

## G. Re-auth and in-progress state

- [ ] G-01 Force `ErrInvalidToken` during secret edit save.
  - Expected: re-auth flow opens.
  - Expected: typed edit values still present after re-auth return.
- [ ] G-02 Repeat for patch mode and overwrite mode.

## H. Footer/token behavior

- [ ] H-01 Footer TTL decrements each second while idle on multiple screens.
- [ ] H-02 Toggle auto-renew on/off and confirm watcher behavior.
- [ ] H-03 Verify footer status after token renewal and after manual renew.

## I. Security/no-plaintext expectations

- [ ] I-01 Secret values are masked in all views.
- [ ] I-02 Copy/export paths work and clipboard auto-clear occurs.
- [ ] I-03 No plaintext values appear in status bar, errors, or help overlays.
- [ ] I-04 Exported file permission behavior is private as designed.

## Suggested Automated Tests to Add

## Screen-level tests

- [ ] Versions view renders deleted timestamp and status from server-provided versions list.
- [ ] Permission denied on version operations produces actionable error state.
- [ ] Direct-path recovery prompt navigation and cancel flows.

## Vault integration tests

- [ ] Policy-specific version action tests for `data/*` vs `delete/*/undelete/*/destroy/*` capability combinations.
- [ ] Rollback correctness test (content equality between source version and new latest).
- [ ] Delete-all and destroy-all validation against metadata endpoint.

## End-to-end smoke tests

- [ ] Auth -> engine list -> path browse -> version delete/undelete/destroy -> verify statuses.
- [ ] Invalid token during edit -> re-auth -> resume state.

## Test Run Notes

Record each run:

- Build commit
- Vault server version
- Policy profile used
- Mount and path tested
- Action
- Expected result
- Actual result
- CLI verification output (if applicable)
