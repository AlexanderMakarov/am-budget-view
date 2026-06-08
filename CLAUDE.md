# AM-Budget-View — agent notes

## Configuration files

| File | Purpose |
|------|---------|
| `config.yaml` (repo root) | Embedded default copied on first app run. May contain the developer's local settings — **do not use in tests** and **do not overwrite with test data**. |
| `testdata/config.yaml` | Stable fixture for automated tests. Prefer this path or inline temp YAML in `_test.go` files. |

When writing or running tests that need a config file, use `testdata/config.yaml` or create a temporary file — never read or modify the repo-root `config.yaml`.

## Security model — local-only execution

**This app is designed to be run locally by a single user on their own machine. It is not hardened for multi-user or networked deployment.** Keep this in mind when changing the web layer:

- The web UI listens on `:<UIPort>` (all interfaces) and has **no authentication, no CSRF protection, and no Host-header validation**. Anyone able to reach the port can drive every endpoint.
- The bank-download endpoints (`/bank-downloads/run`, `/bank-downloads/config`) accept bank session credentials (Bearer token / Cookie), trigger authenticated calls to bank APIs, and write statement files to the working directory. These are powerful actions that assume a trusted local caller.
- Mitigations already in place: session secrets are **ephemeral** (never persisted — see `StripBankDownloadSecrets`, `writeBankDownloadsConfig`), HTTP logs redact `Authorization`/`Cookie`/`Set-Cookie` (`http_log.go`), and statement output is confined to the working directory (`resolveOutputDir`).
- **Before exposing this app beyond `localhost`** (e.g. binding a public interface, running behind a shared proxy, or packaging as a service), add: bind to `127.0.0.1` by default, an auth layer, CSRF tokens on state-changing POSTs, and a Host-header allowlist to defend against DNS-rebinding. Until then, treat "runs only on the user's own machine" as a hard assumption.

## Running tests

```bash
go test ./...
```

## Docs

Per-bank instructions: `docs/en/banks/` and `docs/ru/banks/`. Served in-app at `/docs/bank/{sourceId}`.
