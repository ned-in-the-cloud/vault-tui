# Vault TUI Implementation Checklist

This checklist reflects the current state of the phased plan in `PLANNING.md` plus the additional items captured below.

Legend:

- `[x]` implemented
- `[ ]` still to do

## Prioritized Execution Order

- [x] Finish namespace switching so changing the namespace immediately refreshes the current screen state.
- [x] Implement expired-token recovery and stack-preserving re-auth.
- [x] Add the settings/help/error-display polish layer from Phase 6.
- [x] Implement rollback, delete-all, and destroy-all version workflows.
- [ ] Replace the current lightweight renewal loop with the full watcher-driven message flow described in the plan.

## Completed In This Commit Batch

- [x] Add global equivalent Vault CLI command overlay support (`alt+x`) with per-screen command rendering.
- [x] Add global quit hotkey support for `Ctrl+Q` alongside `Ctrl+C`.
- [x] Improve auth form Enter behavior by jumping to the first required empty field before submit.
- [x] Add capability probing to gate secret creation actions in the path browser.
- [x] Refresh token status-bar info alongside screen reload flows.
- [x] Update KV v2 patch behavior to use read-write merge semantics for broader policy compatibility.
- [x] Add rollback plus delete-all and destroy-all version workflows to the KV v2 versions screen.

## Phase 1: Scaffolding, Connection, and Authentication

- [x] Initialize memguard, load config, initialize logging, and start the Bubble Tea program.
- [x] Implement the root TUI model with a screen stack and global key handling.
- [x] Implement screen stack navigation with push, pop, replace, and current screen helpers.
- [x] Define global key bindings including `Ctrl+C`, `Ctrl+Q`, `Esc`, `?`, and command toggle.
- [x] Build the Vault client abstraction with address, namespace, auth, token, mounts, and KV accessors.
- [x] Classify Vault/API failures into sentinel errors such as permission denied, not found, sealed, connection failure, and invalid token.
- [x] Implement auth methods for token, userpass, and LDAP.
- [x] Implement secure in-memory secret handling with `SecureString`.
- [x] Implement token persistence with keyring and fallback modes.
- [x] Implement encrypted token file support for passphrase-backed persistence.
- [x] Implement clipboard copy with auto-clear support.
- [x] Implement secure file export primitives with private file permissions.
- [x] Implement config load/save with defaults, confirmation prefs, history, export prefs, and token persistence mode.
- [x] Implement connect screen with address, namespace, CA bundle, and `http://` warning confirmation.
- [x] Implement auth screen with dynamic credential inputs and post-login navigation to the dashboard.

## Phase 2: Dashboard and Token Management

- [x] Implement dashboard with server info, token info, and navigation menu.
- [x] Implement persistent status bar with server, namespace, TTL, policies, and help hint.
- [x] Implement TTL countdown refresh behavior.
- [x] Implement manual token renewal.
- [x] Implement auto-renew toggle and renewal behavior.
- [x] Implement token detail screen with expanded token metadata.
- [ ] Replace the current lightweight renewal loop with the full watcher-driven message flow described in the plan.

## Phase 3: Secrets Engine Listing and KV Path Browsing

- [x] Implement secrets engine list with KV mounts prioritized.
- [x] Show non-KV mounts as not yet supported.
- [x] Implement secret path browser for folders and leaf secrets.
- [x] Implement breadcrumb component for KV navigation.
- [x] Implement create-secret entry from the current path when the token has create/update capability.
- [x] Implement refresh behavior for mount and secret lists.
- [ ] Implement the planned direct-path recovery flow for permission-denied browsing errors.
- [x] Complete namespace-switch reload behavior after the prompt closes.

## Phase 4: KV Engine Interface Layer

- [x] Implement shared KV engine interfaces for v1 and v2.
- [x] Implement KV v1 list, get, put, and delete.
- [x] Implement KV v2 list, get, get version, metadata, subkeys, put, patch, delete, undelete, destroy, and delete-all back-end operations.
- [x] Wrap secret values in `SecureString` when reading from Vault.
- [x] Provide secure helper flows for put/update from `SecureString` values.
- [x] Cover KV behavior with unit tests and integration tests.

## Phase 5: KV Secret Operations

- [x] Implement secret view with masked on-screen values.
- [x] Use KV v2 subkeys for key-name-only loading when permitted.
- [x] Fall back to reading the full secret for KV v2 when subkeys access is denied.
- [x] Implement copy-to-clipboard for selected keys.
- [x] Implement export flow for JSON, YAML, dotenv, and single-key raw output.
- [x] Enforce private file permissions on export output.
- [x] Implement version history screen for KV v2.
- [x] Implement undelete, delete-version, and destroy-version actions for KV v2 versions.
- [x] Implement create and update flows through the shared secret edit screen.
- [x] Implement v2 patch vs overwrite behavior in the shared secret edit screen.
- [x] Implement confirmation dialogs with suppression flags for non-destructive categories.
- [x] Require typing `DESTROY` for destructive confirmation flows.
- [x] Implement v1 permanent delete confirmation.
- [x] Implement v2 current-version delete confirmation.
- [x] Implement rollback action from the versions screen.
- [x] Implement delete-all-versions and destroy-all UI flows.
- [ ] Implement CAS controls in the edit UI when mounts require check-and-set.
- [ ] Add explicit tests for no-plaintext screen rendering and broader end-to-end TUI secret workflows.

## Phase 6: Polish and Extensibility

- [x] Implement context-sensitive help component.
- [x] Implement settings screen for masking, clipboard, auto-renew, export defaults, persistence mode, confirmation reset, and history clearing.
- [x] Implement token-expiry recovery that pushes auth on `ErrInvalidToken` and returns to the interrupted screen after re-auth.
- [x] Implement a dedicated error overlay/toast with reconnect and re-auth actions.
- [ ] Preserve in-progress edit state across the planned re-auth flow.

## Improvements

- [x] Show the equivalent Vault CLI command from screens that support it, using a dedicated toggle key.
- [x] When the auth form has required empty fields, pressing Enter jumps to the first empty field instead of immediately submitting.
- [x] Allow quitting from anywhere with `Ctrl+Q` in addition to `Ctrl+C`.

## Next Recommended Slice

- [ ] Replace the current lightweight renewal loop with the full watcher-driven message flow described in the plan.
- [ ] Preserve in-progress edit state across the planned re-auth flow.
