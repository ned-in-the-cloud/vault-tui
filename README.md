# vault-tui

A terminal UI for HashiCorp Vault that lets you browse secrets safely. Secret values are **never shown in plain text** — they can only flow to two deliberate output sinks: the system clipboard (with auto-clear) or a local file written through an explicit export flow.

## Features

- **Connect** to any Vault server with optional namespace, TLS CA bundle, and an `http://` warning guard
- **Authenticate** via token, userpass, or LDAP
- **Token management** — live TTL countdown, manual renewal, watcher-driven auto-renew, and token detail view
- **KV v1 & v2** — browse engine mounts, navigate secret paths with breadcrumbs, and view masked secret keys
- **Version workflows** — list all versions with creation/deletion timestamps, rollback, soft-delete, undelete, destroy, delete-all, and destroy-all
- **Secret edit** — create and update KV secrets; edit state survives re-auth when a token expires mid-edit
- **Secure clipboard** — copy any secret value with automatic clipboard clear after a configurable timeout
- **Secure file export** — export secrets to `.json`, `.env`, or `.yaml` with private file permissions
- **Direct-path recovery** — enter a known path directly when permission-denied errors block normal browsing
- **OS keyring integration** — tokens are stored in the system keyring (Keychain / Credential Manager / Secret Service) with a passphrase-encrypted file fallback
- **Encrypted token storage** — AES-256-GCM at rest, key derived via Argon2id, cached in a memguard enclave for the process lifetime
- **Memory security** — all secret values live in [memguard](https://github.com/awnumar/memguard) enclaves and are zeroed before GC
- **Logging** — structured `slog` output to `~/.vault-tui/vault-tui.log`; secret values are never logged

## Installation

### Pre-built binaries

Download the latest release for your platform from the [releases page](https://github.com/ned-in-the-cloud/vault-tui/releases).

### Build from source

Requires Go 1.23 or later.

```bash
git clone https://github.com/ned-in-the-cloud/vault-tui.git
cd vault-tui
make build          # produces bin/vault-tui
```

## Usage

```
vault-tui
```

On first launch you will be prompted for a Vault server address. Subsequent connections are saved to `~/.vault-tui/config.json`.

### Key bindings

| Key | Action |
|-----|--------|
| `?` | Toggle context-sensitive help |
| `Alt+X` | Show equivalent Vault CLI command for the current view |
| `Ctrl+Q` / `Ctrl+C` | Quit |
| `Esc` | Go back / close overlay |
| `r` | Refresh current screen |
| `n` | Switch namespace |
| `p` | Enter direct path (secret list, when browsing is blocked) |

## Configuration

Settings are stored in `~/.vault-tui/config.json`. The file is created automatically on first run. Available preferences include:

- **Token persistence mode** — keyring, encrypted file, or none
- **Clipboard clear timeout**
- **Export format defaults**
- **Per-action confirmation suppression**

## Development

### Requirements

- Go 1.23+
- [`golangci-lint`](https://golangci-lint.run/usage/install/)
- [`gosec`](https://github.com/securego/gosec)
- [Vault CLI](https://developer.hashicorp.com/vault/install) (for integration tests and dev seeding)

### Common Makefile targets

```bash
make build    # compile ./bin/vault-tui
make test     # run unit tests
make vet      # go vet
make lint     # golangci-lint
make sec      # gosec security scan
make all      # vet + test + build
```

### Running integration tests

The integration tests target a real Vault server. The bootstrap script starts a temporary dev server on `127.0.0.1:8210` and seeds it with mounts, auth methods, users, policies, and test data.

```powershell
# Start and seed (Windows / PowerShell)
.\scripts\seed-dev.ps1

# Run the integration suite
go test -tags=integration ./internal/vault/...

# Stop the temporary server when done
.\scripts\seed-dev.ps1 -Stop
```

Environment variables used by the integration tests:

| Variable | Default |
|----------|---------|
| `VAULT_TUI_TEST_ADDR` | `http://127.0.0.1:8210` |
| `VAULT_TUI_TEST_TOKEN` | `root-test-token` |

Pre-seeded userpass logins: `alice / hunter2` and `bob / tacos`.

## Architecture

```
cmd/vault-tui/       — entry point (memguard init, config, logging, tea.Program)
internal/
  vault/             — Vault client abstraction, KV engines, auth, token watcher
  secure/            — SecureString (memguard), keyring, clipboard, file export, crypto
  config/            — user preferences, connection history
  tui/               — Bubble Tea root model, screen stack, theme, key bindings
    screens/         — one file per screen/flow
    components/      — reusable UI widgets (status bar, breadcrumb, confirm dialog)
  logging/           — slog wrapper with secret-value redaction rules
```

The TUI follows the Elm architecture via Bubble Tea v2. Screens communicate through typed messages (`PushScreenMsg`, `PopScreenMsg`, `ReplaceScreenMsg`, `TokenInfoMsg`, etc.) rather than shared mutable state.

## Security considerations

- Secret values never appear as plain text in any terminal output or log
- The only outputs are clipboard (with auto-clear) and local files (with `0600` permissions)
- Tokens are stored encrypted at rest using AES-256-GCM with an Argon2id-derived key
- All in-memory secret values are held in memguard enclaves and zeroed on release
- `gosec` and `golangci-lint` run in CI on every pull request

## License

MIT
