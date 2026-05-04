# Vault TUI — Open Planning Questions

Answer inline under each question. Once answered, the answers will be folded back into `PLANNING.md` and the resolved item is marked `[RESOLVED]`.

---

## 1. Env-var export [WITHDRAWN]

**Resolution:** Feature dropped entirely. Secret output sinks are now limited to clipboard copy and local file export. PLANNING.md updated to remove `internal/secure/subshell.go`, the `e` action on the secret view screen, and all related context.

---

## 2. Multiple identities per server [RESOLVED]

**Resolution:** Tokens keyed by `address + auth method + username/identifier`. Folded into PLANNING.md (architecture decision 10, Phase 1c).

Follow-up question added below as Q11 (Token-auth identifier source).

---

## 3. Vault Enterprise namespaces [RESOLVED]

**Resolution:** In scope. Per server entry, surfaced in status bar, applied to all calls. Folded into PLANNING.md (architecture decision 8, `Client` interface, `ServerEntry`, status bar, dashboard).

Follow-up question added below as Q12 (mid-session namespace switching).

---

## 4. TLS toggle semantics [RESOLVED]

**Resolution:** No toggle — TLS is implied by URL scheme. Custom CA bundle path supported per server entry. Connecting to `http://` requires warning + confirmation. Folded into PLANNING.md (architecture decision 9, `Client` interface, Phase 1e).

Follow-up question added below as Q13 (CA bundle global default vs. per-entry).

---

## 5. Encrypted token file fallback — key source [RESOLVED]

**Resolution:** Layered fallback: keyring → OS-bound key → passphrase → no persistence. Folded into PLANNING.md (architecture decision 10, Phase 1c).

Follow-up questions added below as Q14 (OS-bound credential library) and Q15 (passphrase caching).

---

## 6. Re-auth flow and screen stack [RESOLVED]

**Resolution:** Push auth on stack, pop back on success, preserve in-progress edits. Folded into PLANNING.md (architecture decision 11, Phase 6 token expiry recovery).

---

## 7. Logging — location and redaction policy [RESOLVED]

**Resolution:** `~/.vault-tui/vault-tui.log`, no built-in rotation, baseline redaction rules (no secret values; secret keys debug-only; paths at info; tokens/accessors not above debug). Folded into PLANNING.md (project structure logger.go comment).

---

## 8. Go version and tooling baseline [RESOLVED]

**Resolution:** Go 1.23 minimum. `golangci-lint`, `staticcheck`, and `gosec` added to Makefile. Folded into PLANNING.md (Tech Stack section).

---

## 9. Default export format preference [RESOLVED]

**Resolution:** Yes — `DefaultExportFormat` added to config and Settings screen. Folded into PLANNING.md (Phase 1d, Phase 6).

---

## 10. Subshell host shell selection [WITHDRAWN]

**Resolution:** Superseded by the withdrawal of Q1. No subshell will be spawned, so shell-selection answers are no longer applicable.

Previous answer (kept for record):
- On Unix: launch `$SHELL`, fall back to `/bin/sh` if unset
- On Windows: Prefer PowerShell
- There should be a config option
- Brief confirmation
- Yes, add a marker env var

---

## 11. Token-auth `Identifier()` source [RESOLVED]

**Resolution:** Use `display_name` if set, fall back to the token's accessor. Folded into PLANNING.md (architecture decision 10, Phase 1b `auth.go` description).

Previous question text retained for context:

Tokens are now keyed by `address + auth_method + identifier`. UserPass and LDAP have a clear username. Token auth does not.
---

## 12. Mid-session namespace switching [RESOLVED]

**Resolution:** Hotkey `N` on dashboard / engine list switches the active namespace; change applies to subsequent calls only. Folded into PLANNING.md (architecture decision 8, Phase 2a, Phase 3a).

---

## 13. CA bundle scope [RESOLVED]

**Resolution:** Per-server only. No PLANNING.md change required (already current behavior).
---

## 14. OS-bound credential library [RESOLVED]

**Resolution:** Drop the OS-bound tier entirely. Token persistence is now keyring → passphrase-encrypted file → no persistence. Folded into PLANNING.md (architecture decision 10, Phase 1c, project structure crypto.go and keyring.go descriptions, `TokenPersistenceMode` config values).

---

## 15. Passphrase caching [RESOLVED]

**Resolution:** Prompt once at startup; cache the derived key in a memguard `Enclave` for the lifetime of the TUI process. Folded into PLANNING.md (architecture decision 10, Phase 1c keyring.go and crypto.go descriptions).
---

## 16. Subshell on no-TTY / Windows Terminal quirks [WITHDRAWN]

**Resolution:** Superseded by the withdrawal of Q1. No subshell will be spawned, so this question is no longer applicable.

Previous answer (kept for record):
- Suspend the TUI (`tea.Program.ReleaseTerminal`), run the subshell foreground, restore the TUI on subshell exit.