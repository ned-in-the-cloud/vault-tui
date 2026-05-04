# Vault TUI — Implementation Plan

## Context

Build a terminal UI for HashiCorp Vault in Go that provides safe, interactive access to Vault secrets. The core problem: the Vault CLI is not intuitive for inexperienced users and can potentially leak secrets to the terminal. This TUI helps users navigate the Vault server while ensuring secret values are **never displayed in plaintext on-screen** — they can only be sent to explicit output sinks: the clipboard (with auto-clear) or local files written through a deliberate export flow. The initial scope covers connection/auth (with Vault Enterprise namespace support), dashboard, token management, and full KV v1/v2 secrets engine support, including file export for KV secrets. The architecture is designed for future extension to dynamic secrets engines and operator functions.

**Go module**: `github.com/ned1313/vault-tui`
**Minimum Go version**: 1.23

## Technology Stack

| Component | Library | Purpose |
|-----------|---------|---------|
| TUI framework | `charmbracelet/bubbletea` v2 | Elm-architecture TUI |
| UI components | `charmbracelet/bubbles` v2 | Lists, tables, text inputs |
| Styling | `charmbracelet/lipgloss` v2 | Terminal styling/layout |
| Forms | `charmbracelet/huh` v2 | Structured input forms |
| Vault client | `hashicorp/vault/api` | Official Vault Go client |
| Token storage | `zalando/go-keyring` | OS keyring (Keychain/Credential Manager/Secret Service) |
| Memory security | `awnumar/memguard` | Encrypted memory for secrets |
| Clipboard | `golang.design/x/clipboard` | Cross-platform clipboard with auto-clear |

**Build / quality tooling**: `go vet`, `staticcheck`, `golangci-lint`, `gosec`. All wired into `Makefile` targets and run in CI before tests.

## Project Structure

```
vault-tui/
├── cmd/vault-tui/
│   └── main.go                        # Entry point, memguard init, run tea.Program
├── internal/
│   ├── vault/                         # Vault client abstraction (no TUI imports)
│   │   ├── client.go                  # Client interface + production impl
│   │   ├── auth.go                    # Auth method types + login logic
│   │   ├── kv.go                      # KVEngine / KVv2Engine interfaces + impls
│   │   ├── token.go                   # Token lookup, renew, LifetimeWatcher
│   │   ├── sys.go                     # ListMounts, Health, SealStatus
│   │   ├── errors.go                  # Sentinel errors from HTTP status codes
│   │   └── mock.go                    # Mock Client for tests
│   ├── secure/                        # Security primitives
│   │   ├── memory.go                  # SecureString type (memguard Enclave wrapper)
│   │   ├── keyring.go                 # Token store: keyring primary, passphrase-encrypted file fallback, no-persistence final fallback
│   │   ├── clipboard.go              # Copy + auto-clear goroutine
│   │   ├── fileexport.go             # Export secrets to local files with secure permissions
│   │   └── crypto.go                  # AES-256-GCM at-rest encryption for tokens.enc; key derived from a startup passphrase via Argon2id and cached in a memguard Enclave for the process lifetime
│   ├── config/                        # User preferences
│   │   ├── config.go                  # Config struct, load/save (~/.vault-tui/config.json)
│   │   ├── preferences.go            # Confirmation suppression per category
│   │   └── history.go                 # Server connection history
│   ├── tui/                           # Bubble Tea presentation layer
│   │   ├── app.go                     # Root model: screen stack, global keys, layout
│   │   ├── nav.go                     # ScreenStack: push/pop/replace
│   │   ├── theme.go                   # Lip Gloss style definitions
│   │   ├── keys.go                    # Global key bindings
│   │   ├── components/
│   │   │   ├── statusbar.go           # Bottom bar: server, TTL countdown, policies
│   │   │   ├── breadcrumb.go          # Path breadcrumb for KV navigation
│   │   │   ├── confirm.go            # Confirmation dialog + "don't ask again"
│   │   │   ├── errordisplay.go        # Error overlay/toast
│   │   │   ├── secretfield.go         # Masked input (stars or blank)
│   │   │   └── help.go                # Context-sensitive help panel
│   │   └── screens/
│   │       ├── connect.go             # Server address + TLS options
│   │       ├── auth.go                # Auth method selection + credential forms
│   │       ├── dashboard.go           # Post-auth home screen
│   │       ├── tokendetail.go         # Token info + renew
│   │       ├── enginelist.go          # Secrets engine mount browser
│   │       ├── secretlist.go          # Path browser within a KV mount
│   │       ├── secretview.go          # Secret keys (masked values) + copy/export
│   │       ├── secretexport.go        # Export secret to file (.json/.env/.yaml) or single-key file
│   │       ├── secretcreate.go        # New secret form
│   │       ├── secretupdate.go        # Patch vs overwrite (v2)
│   │       ├── secretversions.go      # Version list (v2)
│   │       └── secretdelete.go        # Delete/destroy flow
│   └── logging/
│       └── logger.go                  # slog-based, file-only output to ~/.vault-tui/vault-tui.log; redaction rules: never log secret values, log secret keys only at debug, log paths at info, never echo response bodies, never log tokens or token accessors above debug. No built-in rotation (use OS log rotation tools).
├── go.mod
├── go.sum
├── Makefile
└── CLAUDE.md
```

## Key Architecture Decisions

1. **Screen stack navigation** — Screens are `tea.Model` instances on a `[]tea.Model` stack. Push/pop maps naturally to hierarchical KV browsing. Screens return `NavigateMsg` commands; only the root model manipulates the stack.

2. **Interface-based Vault client** — All Vault calls go through a `Client` interface. Production wraps `*api.Client`; tests use `MockClient`. No screen imports `vault/api` directly.

3. **`AppContext` dependency injection** — A shared struct carrying the Vault client, secure storage manager, config, and logger. Passed to every screen constructor.

4. **`SecureString` everywhere** — All secret values are `memguard.Enclave` wrappers. Opened to `LockedBuffer` only for the minimum duration needed (clipboard write, API call), then destroyed.

5. **KV v2 `subkeys` for display** — For KV v2, the secret view screen loads key names via the `subkeys` endpoint (no values returned). Secret values are only fetched when the user explicitly copies, exports, or saves to file. KV v1 has no metadata-only equivalent, so opening a v1 secret unavoidably fetches values; the implementation must wrap them in `SecureString` immediately and never render plaintext on-screen.

6. **Explicit output sinks** — Secrets are never rendered in plaintext in the TUI. Sensitive values may only leave secure memory through two explicit user actions: clipboard copy or local file export. File export is treated as higher risk than clipboard and uses an explicit export flow with format selection, path selection, overwrite confirmation, and private file permissions.

7. **All Vault I/O is asynchronous** — Vault API calls never run inline in `Update`. They are wrapped in `tea.Cmd` closures that return result messages (success/error). The UI shows a loading indicator while waiting. This applies to login, lookup, list, get, put, patch, delete, and metadata calls.

8. **Vault Enterprise namespaces are first-class** — Each server history entry stores an optional namespace. The active namespace is applied to every Vault call via `X-Vault-Namespace` and is surfaced in the status bar alongside the server address. The active namespace can be changed mid-session via a hotkey (`N`) on the dashboard and engine list; the change applies only to subsequent calls.

9. **TLS is determined by the server URL scheme** — `https://` enables TLS; `http://` does not. There is no separate TLS toggle. Connecting to an `http://` address requires explicit warning + confirmation. Users may provide a custom CA bundle path per server entry; if absent, the system trust store is used.

10. **Token persistence is a layered fallback** — (1) OS keyring via `go-keyring`, (2) AES-256-GCM-encrypted file at `~/.vault-tui/tokens.enc` with key derived from a user passphrase via Argon2id, (3) no persistence, login each session. The passphrase is prompted once at startup and the derived key is cached in a memguard `Enclave` for the lifetime of the TUI process. Tokens are keyed by `address + auth_method + identifier` so multiple identities per server do not overwrite each other. For Token auth, `identifier` is the token's `display_name` if set, falling back to its accessor.

11. **Re-auth preserves the screen stack** — On `ErrInvalidToken`, the auth screen is pushed on top of the current stack. On successful re-auth, it pops back to the originating screen and the failed operation may be retried. In-progress edit forms are preserved.

12. **Confirmation categories** — Four independent suppression flags in config (delete version, destroy version, delete all, destroy all). Destroy operations require typing "DESTROY" regardless of suppression. The file-export overwrite prompt is independent and never suppressed.

---

## Phase 1: Scaffolding, Connection, and Authentication

**Goal**: Launch the TUI, connect to a Vault server, authenticate, arrive at a placeholder dashboard with a stored token.

### 1a. Core Infrastructure
- `cmd/vault-tui/main.go` — Init memguard (`CatchInterrupt`, `defer Purge`), load config, create root model, run `tea.NewProgram`
- `internal/tui/app.go` — Root `tea.Model` owning `ScreenStack` + `AppContext`. Routes global keys (quit, back, help), delegates to `stack.Current().Update(msg)`
- `internal/tui/nav.go` — `ScreenStack` type with `Push`, `Pop`, `Replace`, `Current`
- `internal/tui/theme.go` — Lip Gloss style constants with adaptive light/dark
- `internal/tui/keys.go` — Global bindings: `ctrl+c` quit, `esc` back, `?` help

### 1b. Vault Client Abstraction
- `internal/vault/client.go` — `Client` interface:
  ```go
  type Client interface {
      SetAddress(addr string) error
      Address() string
      SetNamespace(ns string)         // empty string clears
      Namespace() string
      SetCABundlePath(path string) error // empty path uses system trust store
      Health(ctx context.Context) (*HealthInfo, error)
      Login(ctx context.Context, method AuthMethod) (*TokenInfo, error)
      SetToken(token string)
      LookupToken(ctx context.Context) (*TokenInfo, error)
      RenewToken(ctx context.Context, increment int) (*TokenInfo, error)
      ListMounts(ctx context.Context) (map[string]*MountInfo, error)
      KVv1(mount string) KVv1Engine
      KVv2(mount string) KVv2Engine
  }
  ```
- `internal/vault/errors.go` — Sentinel errors mapped from HTTP status codes: `ErrPermissionDenied` (403), `ErrNotFound` (404), `ErrServerSealed` (503), `ErrConnectionFailed`, `ErrInvalidToken`
- `internal/vault/auth.go` — Auth method types (Token, UserPass, LDAP) with login logic per method. Each `AuthMethod` exposes an `Identifier()` string used to key stored tokens: UserPass and LDAP use the username; Token auth uses the token's `display_name` if set, falling back to its accessor. Only human auth methods for now; OIDC and machine auth (AppRole) will be added later.
- `internal/vault/mock.go` — `MockClient` with configurable function fields per method

### 1c. Secure Storage
- `internal/secure/memory.go` — `SecureString` wrapping `memguard.Enclave`. Methods: `NewSecureString([]byte)`, `Open() (*LockedBuffer, error)`, `Destroy()`
- `internal/secure/keyring.go` — Layered token store. Tokens are keyed by `address + auth_method + identifier`. Resolution order on read/write:
  1. OS keyring via `go-keyring` (Keychain / Credential Manager / Secret Service)
  2. `tokens.enc` encrypted with a user passphrase (Argon2id key derivation, prompted once at startup; derived key held in a memguard `Enclave` for the process lifetime)
  3. No persistence; user must re-auth each session.
  The active layer is detected at startup based on platform support and user config.
- `internal/secure/crypto.go` — AES-256-GCM at-rest encryption for `~/.vault-tui/tokens.enc`. Key is derived from the startup passphrase via Argon2id, cached in a memguard `Enclave`, and never written to disk alongside the ciphertext.
- `internal/secure/clipboard.go` — `CopyToClipboard(data, clearAfter)` with auto-clear goroutine
- `internal/secure/fileexport.go` — Writes exported secrets to user-selected paths. Defaults to `~/.vault-tui/exports`, prompts before overwrite, ensures private file permissions, and is structured to support optional encrypted exports later.

### 1d. Config System
- `internal/config/config.go` — Load/save `~/.vault-tui/config.json`. Fields: `MaskingStyle` (Stars/Blank), `ClipboardClearSecs` (default 15), `ConfirmationPrefs`, `ServerHistory`, `DefaultAuthMethod`, `AutoRenewToken`, `DefaultExportDir`, `DefaultExportFormat` (JSON / YAML / .env / single-key), `PreferEncryptedFileExport` (future), `TokenPersistenceMode` (auto / keyring / passphrase / none)
- `internal/config/preferences.go` — `ConfirmationPrefs` with four boolean suppress fields
- `internal/config/history.go` — `ServerEntry{Address, Namespace, CABundlePath, LastUsed, AuthMethod, AuthPath, LastIdentifier}`

### 1e. Connect and Auth Screens
- `internal/tui/screens/connect.go` — Server address input (with history list), optional namespace field, optional CA bundle path. TLS is implied by the URL scheme (`https://` enables TLS, `http://` disables it). Connecting to an `http://` address shows a danger-level warning and requires explicit confirmation. Calls `Health()` to verify, then navigates to auth screen.
- `internal/tui/screens/auth.go` — Two-step: select auth method (Token, UserPass, LDAP) + mount path, then dynamic credential fields per method. All secret inputs use configured masking. On success: store token in the active persistence layer keyed by `address + auth_method + identifier`, navigate to dashboard.

### Testing
- Unit: error classification, mock client interface satisfaction, config round-trip, stack push/pop, keyring with mock, crypto encrypt/decrypt round-trip
- Integration: connect + token auth against `vault server -dev`

---

## Phase 2: Dashboard and Token Management

**Goal**: Post-auth home screen with live token info, server details, and token renewal.

### 2a. Dashboard
- `internal/tui/screens/dashboard.go` — Three panels: Server Info (address, namespace, seal status, version), Token Info (policies, TTL countdown, accessor, identity), Navigation Menu (Secrets Engines, Token Management, Settings). Hotkey `N` opens a namespace-switch prompt; the new namespace applies to subsequent calls only.
- `internal/tui/components/statusbar.go` — Persistent bottom bar with server address, namespace (when set), TTL countdown (via `tea.Tick`), policy summary. Rendered by root model below active screen.
- TTL countdown via `tea.Tick(time.Second)`. If `AutoRenewToken` enabled, start `LifetimeWatcher` in `Init()`.

### 2b. Token Management
- `internal/tui/screens/tokendetail.go` — Full token details table (accessor, creation time, TTL, policies, renewable, num uses). Actions: `r` renew, `a` toggle auto-renewal.
- `internal/vault/token.go` — `TokenInfo` struct, `LookupToken`, `RenewToken`, `LifetimeWatcher` integration producing `TokenRenewedMsg`/`TokenRenewalFailedMsg`

### Testing
- Unit: token info parsing, TTL countdown math, watcher message production
- Manual: dev Vault with short-TTL token (`vault token create -ttl=2m -renewable`)

---

## Phase 3: Secrets Engine Listing and KV Path Browsing

**Goal**: Browse available secrets engines, select a KV mount, navigate the secret path hierarchy.

### 3a. Engine List
- `internal/tui/screens/enginelist.go` — Calls `ListMounts()`, filters to KV type, shows mount path + version + description. Selecting navigates to secret list. Hotkey `N` opens the same namespace-switch prompt as the dashboard; on switch the mount list is reloaded.
- `internal/vault/sys.go` — `MountInfo{Path, Type, Description, Options}`, `ListMounts` implementation

### 3b. Secret Path Browser
- `internal/tui/screens/secretlist.go` — Lists keys at current path. Folders (trailing `/`) vs secrets styled differently. Breadcrumb above list. Navigation: `Enter` drills into folder or opens secret, `Backspace`/`esc` goes up, `n` creates new secret, `/` activates filter.
- `internal/tui/components/breadcrumb.go` — Renders `mount/ > path/ > subpath/` with styled separators
- Permission denied handling: show error overlay with option to type a direct path

### Testing
- Unit: mount list parsing, key classification (folder vs secret), breadcrumb rendering
- Integration: browsing pre-seeded secrets on dev Vault

---

## Phase 4: KV Engine Interface Layer

**Goal**: Complete Vault KV abstraction for both v1 and v2. Pure backend — no TUI code. Can be developed in parallel with Phases 2-3.

- `internal/vault/kv.go`:
  ```go
  type KVEngine interface {
      Version() int
      Mount() string
      List(ctx, path) ([]KVEntry, error)
      Get(ctx, path) (*KVSecret, error)
      Put(ctx, path, data) error
      Delete(ctx, path) error
  }

  type KVv2Engine interface {
      KVEngine
      GetVersion(ctx, path, version) (*KVSecret, error)
      GetVersionsList(ctx, path) ([]KVVersionMeta, error)
      GetMetadata(ctx, path) (*KVMetadata, error)
      GetSubkeys(ctx, path, version, depth) ([]string, error)
      Put(ctx, path, data, cas *int) error // overrides KVEngine.Put with optional CAS
      Patch(ctx, path, data, cas *int) error
      DeleteVersions(ctx, path, versions) error
      UndeleteVersions(ctx, path, versions) error
      DestroyVersions(ctx, path, versions) error
      DeleteAllVersions(ctx, path) error
  }
  ```
- `KVSecret.Data` is `map[string]*secure.SecureString` — values wrapped in memguard on read, source zeroed
- `PutFromSecure` helper opens enclaves, calls Put, destroys buffers in a single method
- `ExportFromSecure` helper opens enclaves only for serialization/writing, writes the selected format, then destroys temporary buffers immediately after the file write completes
- Two implementations: `kvV1Engine` wrapping `*api.KVv1`, `kvV2Engine` wrapping `*api.KVv2`

### Testing
- Unit: every method against MockClient, verify SecureString wrapping/cleanup
- Integration: full CRUD cycle on dev Vault with both KV v1 and v2 mounts

---

## Phase 5: KV Secret Operations (Core Feature)

**Goal**: Complete KV secrets management — view, create, update, delete, destroy.

### 5a. Secret View
- `internal/tui/screens/secretview.go` — Shows key names with masked values (`********`). **Key design**: for KV v2, loads only key names via `GetSubkeys` (no secret data in memory); for KV v1, calls `Get` and wraps immediately.
- Actions: `c` copy value to clipboard (opens SecureString only then), `f` export to local file, `v` view versions, `u` update, `d` delete, `n` create new

### 5b. Secret File Export
- `internal/tui/screens/secretexport.go` — Explicit export flow for local files. User selects export mode, output path, and confirms write if the target already exists.
- Export modes:
  - Entire secret as JSON
  - Entire secret as YAML
  - Entire secret as `.env`
  - Single selected key written to a file in raw plaintext
- Default path is under `~/.vault-tui/exports`, but the user may choose any destination path. The export form pre-populates a safe default filename based on mount + path + selected format.
- Exported files persist until manually deleted. There is no auto-delete in the initial scope.
- Export writes plaintext in Phase 5. The implementation should be structured so encrypted file export can be added later without redesigning the screen or secure-writing API.
- All export writes must enforce private file permissions (`0600`-equivalent on Unix; restricted ACL on Windows).
- Overwrite behavior is always prompt-before-write. This prompt is separate from delete/destroy confirmation preferences and is never suppressed.

### 5c. Secret Versions (KV v2)
- `internal/tui/screens/secretversions.go` — Table: version, created time, status (Current/Active/Deleted/Destroyed). Actions: `Enter` view version, `u` undelete, `d` delete version, `D` destroy version, `r` rollback

### 5d. Secret Create
- `internal/tui/screens/secretcreate.go` — Path input (pre-filled with current browsing path) + dynamic key-value pair editor. Key inputs are plaintext, value inputs are masked per config. `ctrl+a` adds pair, `ctrl+d` removes pair. On submit: values wrapped to SecureString, textinput values cleared/zeroed, `PutFromSecure` called.

### 5e. Secret Update (KV v2)
- `internal/tui/screens/secretupdate.go` — Mode selector: Overwrite (replace all keys) or Patch (merge changes). Overwrite loads key names via `GetSubkeys` (user re-enters all values). Patch shows existing keys, user enters only changed/new values. Supports CAS (check-and-set) if mount requires it.
- KV v1: overwrite only, no mode selector

### 5f. Delete/Destroy
- `internal/tui/screens/secretdelete.go` — Uses confirmation dialog component
- `internal/tui/components/confirm.go` — Reusable dialog with danger levels:
  - **Normal** (yellow): delete version
  - **Warning** (orange): delete all versions
  - **Destructive** (red, requires typing "DESTROY"): destroy operations
- Checks `ConfirmationPrefs` — if suppressed for the category, skips dialog
- "Don't ask again for this category" checkbox, persisted to config

### Operations Matrix (KV v2)

| Operation | Confirmation Category | Danger Level |
|-----------|----------------------|--------------|
| Delete current version | DeleteSecretVersion | Normal |
| Delete specific versions | DeleteSecretVersion | Normal |
| Destroy specific versions | DestroySecretVersion | Destructive |
| Delete all versions + metadata | DeleteAllVersions | Warning |

KV v1 delete is permanent → uses Destructive danger level.

### Testing
- Unit: every KV operation path with MockClient, SecureString lifecycle, CAS conflict handling, confirmation dialog suppression logic, key-value editor add/remove/focus, export serialization (`json`/`yaml`/`.env`), overwrite prompt flow, and private-permission file creation
- Integration: full CRUD cycle on dev Vault (both v1 and v2) — create, read, copy, export to file, patch, version history, delete, undelete, destroy
- Security: verify no plaintext secrets in screen output, exported files are created with private permissions, and temporary buffers are zeroed after clipboard/env/file export actions complete

---

## Phase 6: Polish and Extensibility

**Goal**: Production-quality UX, help system, settings, error recovery.

- `internal/tui/components/help.go` — Context-sensitive help via `bubbles/help`, toggled with `?`
- `internal/tui/screens/settings.go` — Configure masking style, clipboard clear duration, auto-renew, default export directory, default export format, custom CA bundle path defaults, token persistence mode, reset confirmation prefs, clear server history, and later opt into encrypted file export when implemented
- Token expiry recovery: any `ErrInvalidToken` pushes the auth screen on top of the current stack (try stored token first, then prompt). On success, the stack pops back to the screen that issued the failed call; in-progress edit forms are preserved.
- `internal/tui/components/errordisplay.go` — Color-coded error overlay, auto-dismiss, action buttons for reconnect/re-auth
- Extensibility: `enginelist.go` shows all mount types, "Not yet supported" for non-KV, making future additions incremental

---

## Phase Dependency Graph

```
Phase 1 (Scaffolding + Connect + Auth)
   │
   ├──→ Phase 2 (Dashboard + Token)
   │       │
   │       └──→ Phase 3 (Engine List + Path Browser)
   │               │
   │               └──→ Phase 5 (KV Operations) ──→ Phase 6 (Polish + Extensibility)
   │                       ▲
   └──→ Phase 4 (KV Interface Layer) ──┘    (Phase 4 can run in parallel with 2 and 3)
```

## Verification Plan

1. **Dev Vault server**: `vault server -dev` for all integration testing
2. **Per-phase manual testing**: Each phase should produce a runnable binary demonstrating that phase's features
3. **Security verification**: After each phase, confirm no plaintext secrets visible in screen output, clipboard clears correctly, exported files use private permissions, and memguard Enclaves are destroyed
4. **Auth testing**: Test with Token, UserPass, and LDAP methods against dev server, including multiple stored identities per server and namespace scoping (Enterprise dev server)
5. **KV testing**: Seed both v1 and v2 mounts, test full CRUD + version operations + file export flows (JSON, YAML, `.env`, single-key raw file)
6. **Error path testing**: Test with restricted policies to verify permission denied handling, test with expired tokens to verify re-auth stack-preservation flow, test connecting to an `http://` address triggers warning + confirmation, test invalid CA bundle path produces a clear error
