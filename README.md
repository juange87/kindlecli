# kindlecli

A command-line tool to send documents directly to your Kindle devices using Amazon's Send-to-Kindle API. No email configuration needed.

## Features

- **Direct upload** — sends files through Amazon's internal Send-to-Kindle API (the same one used by the official desktop apps)
- **Multiple formats** — EPUB, PDF, MOBI, AZW, AZW3, DOC, DOCX, TXT, RTF, HTML
- **Batch send** — send multiple files in a single command
- **All devices** — automatically sends to every Kindle device on your account
- **Session recovery** — check saved credentials and recover rejected sessions
- **Agent-friendly login** — two-step PKCE login with JSON output
- **Single binary** — no language runtime needed; optional clipboard mode uses OS utilities

## Installation

Build with a current stable Go release (minimum: Go 1.25.5). When using the
locally built binary, replace `kindlecli` with `./kindlecli` in the examples.

### From source

```bash
go install github.com/juange87/kindlecli@latest
```

### Build manually

```bash
git clone https://github.com/juange87/kindlecli.git
cd kindlecli
go build -o kindlecli .
```

## Usage

### 1. Log in to your Amazon account

```bash
kindlecli login
```

If your saved session is valid, this reuses it without opening a browser. Otherwise, it opens your browser for Amazon OAuth2 login. After signing in, you'll be redirected to the Send to Kindle page. Paste the full URL from the address bar into the terminal prompt. Use `kindlecli login --clipboard` for the clipboard workflow.

Your session is saved locally at `~/.config/kindlecli/auth.json`.

### 2. List your Kindle devices

```bash
kindlecli devices
```

```
Found 3 device(s):
  1. Kindle Paperwhite (G0123456789)
  2. Kindle Scribe (G9876543210)
  3. Kindle for Mac (ABCDEF123456)
```

### 3. Send files to your Kindle

```bash
# Send a single file
kindlecli send book.epub

# Send multiple files
kindlecli send book1.epub book2.pdf document.docx

# Specify title and author
kindlecli send --title "Foundation" --author "Isaac Asimov" foundation.epub
```

Files are sent to **all** Kindle devices on your account.

### 4. Log out

```bash
kindlecli logout
```

This attempts to deregister the device from Amazon, then deletes the local
session and pending login. A warning indicates when remote deregistration could
not be confirmed. You do not need to log out before recovering a rejected session.

## Supported Formats

| Format | Extensions |
|--------|-----------|
| EPUB | `.epub` |
| PDF | `.pdf` |
| Kindle | `.mobi`, `.azw`, `.azw3` |
| Documents | `.doc`, `.docx`, `.txt`, `.rtf` |
| Web | `.htm`, `.html` |

## Commands

| Command | Description |
|---------|-------------|
| `login` | Reuse a valid session or authenticate with Amazon via browser OAuth2 |
| `auth status [--json]` | Verify the saved session with Amazon; exit 0 only when valid |
| `send` | Upload and send files to all Kindle devices |
| `devices` | List all Kindle devices on your account |
| `logout` | Attempt remote deregistration and delete local credentials and pending login |

### Global Flags

| Flag | Description |
|------|-------------|
| `--config <dir>` | Config directory (default: `~/.config/kindlecli`) |
| `--verbose` | Enable verbose output for debugging |

### Login Flags

| Flag | Description |
|------|-------------|
| `--start` | Save a pending login and print its URL without waiting or opening a browser |
| `--finish` | Complete the pending login using the redirect URL from stdin |
| `--json` | Structured output with `--start` or `--finish` |
| `--force` | Start a new login without checking the existing session |
| `--fresh` | Ask Amazon for fresh browser authentication; combine with `--force` if the CLI session is valid |
| `--clipboard` | Read the redirect URL from the clipboard instead of pasting it into stdin |
| `--no-browser` | Print the URL without opening a browser during interactive login |

`--start` and `--finish` are mutually exclusive. `--finish` cannot be combined
with `--force`, `--fresh`, or `--no-browser`; `--start` cannot use `--clipboard`.

### Send Flags

| Flag | Description |
|------|-------------|
| `--title <string>` | Document title (default: filename without extension) |
| `--author <string>` | Document author (default: "Unknown") |
| `--upload-timeout <duration>` | Maximum time per file upload (default: `10m`) |

## Session recovery and automation

`kindlecli auth status` checks the saved credentials with Amazon without sending a
file. `--json` returns `state` and `message`. States are `valid`, `missing`,
`invalid`, `rejected`, and `unavailable`; only `valid` exits with code 0.
A network or service error is `unavailable`, not proof that you need to log in.

`kindlecli login` reuses a valid session and replaces rejected credentials only
when authentication succeeds. Use `--force` to replace a session explicitly.
Browser authentication is reused when Amazon permits it; `--fresh` asks Amazon
for a fresh sign-in. Amazon can still require MFA or a password.

For interactive login, paste the full redirect URL at the prompt. Use
`--clipboard` to retain the clipboard workflow, or `--no-browser` to open the
printed URL yourself. Linux clipboard support includes Wayland and X11.

For a skill or another agent, use two invocations with the same config directory:

```bash
kindlecli login --start --json
# Open the returned URL in the user's browser.
# After Amazon redirects, write the full redirect URL to the next command's stdin:
kindlecli login --finish --json
kindlecli auth status --json
```

`--start` returns either `valid` (nothing to do) or `pending` with `url` and
`expires_at`. A pending login expires after 10 minutes. Starting again replaces
the pending attempt, so only the most recent URL can be used. `--finish --clipboard` can read the copied URL instead of stdin. Never put redirect URLs in
command-line arguments, shell history, logs, or chat messages: they contain a
one-time authorization code. Keep browser navigation and URL handling inside the
agent's local tools. Handle password/MFA prompts in the browser yourself.

Pending PKCE credentials are stored privately in `pending-login.json` and deleted
after a successful login. A failed authentication attempt preserves the previous `auth.json`; Amazon still
controls whether an older device registration remains valid.
Login commands are serialized using `.login-lock` in the config directory. If a
process is killed and leaves that directory behind, first ensure no login is
running, then remove the empty `.login-lock` directory and retry.

This flow automates the handoff to the browser; it does not implement refresh
of revoked device tokens. Silent token renewal still needs verification against
Amazon's undocumented device API.

## Batch results

`send` checks input files before contacting Amazon and returns a nonzero exit code
if any file is invalid or fails to upload. Successful files in a partially failed
batch are not rolled back; retry only failed files to avoid duplicates. A rejected
session stops the remaining batch. “Accepted by Amazon” confirms API acceptance,
not that the document has already arrived on a Kindle.

Uploads have a separate 10-minute timeout; change it with
`kindlecli send --upload-timeout 20m large.pdf`. API calls retain a 30-second
limit. Uploads and deliveries are not automatically retried, since a delivery
may already have been accepted when a connection fails.

`logout` always attempts local cleanup, including pending logins. If Amazon's
remote deregistration fails, a warning is printed; deleting local credentials
alone does not prove that Amazon deregistered the virtual device.

## How It Works

kindlecli uses Amazon's internal Send-to-Kindle API, the same undocumented API that powers Amazon's official desktop applications (Send to Kindle for Mac/PC). The authentication flow works as follows:

1. **OAuth2 PKCE login** — opens a browser for standard Amazon sign-in with a PKCE code challenge
2. **Token exchange** — exchanges the authorization code for an access token
3. **Device registration** — registers a virtual device with Amazon, receiving an RSA private key and ADP authentication token
4. **Signed API requests** — all subsequent API calls are signed with the RSA key using a custom PKCS#1 v1.5 padding scheme (SHA-256 hash, no DigestInfo prefix)

The file upload process:

1. Request a pre-signed upload URL from `stkservice.amazon.com`
2. Upload the file via HTTP PUT to the pre-signed URL
3. Send a delivery request to push the file to your Kindle devices

## Project Structure

| Path | Purpose |
|------|---------|
| `main.go`, `cmd/root.go` | CLI entry point, global flags and error exit handling |
| `cmd/auth.go`, `cmd/login.go`, `cmd/logout.go` | Session diagnosis, interactive/two-step login and cleanup |
| `cmd/send.go`, `cmd/devices.go` | Batch validation, delivery and device listing |
| `internal/amazon/auth.go` | PKCE, token exchange and device registration |
| `internal/amazon/stk.go`, `errors.go`, `models.go` | HTTP client, error classification and response validation |
| `internal/amazon/signer.go` | Amazon's custom RSA request signing |
| `internal/config/` | Atomic credential storage and its tests |
| `cmd/*_test.go`, `internal/amazon/*_test.go` | CLI, authentication, HTTP and signing tests |
| `.github/` | CI and dependency updates |
| `docs/authentication-testing.md` | Live login verification and silent-renewal research |
| `AGENTS.md` | Repository instructions for coding agents |

## Security

- Session credentials are stored at `~/.config/kindlecli/auth.json` with `0600` permissions (owner read/write only)
- The saved RSA private key is used locally to sign requests; subsequent API calls do not include it
- Credentials are replaced atomically; pending PKCE credentials also use private files
- `kindlecli logout` attempts remote deregistration and deletes local credentials and pending login
- No credentials are logged or transmitted to third parties

## Development

Use a current stable Go release to build the binary (the module's minimum remains
Go 1.25.5 for compatibility). CI tests that minimum and the current stable release,
including Linux, macOS and Windows, and checks formatting, `go vet`, module
consistency and known reachable vulnerabilities. Dependabot proposes module and
GitHub Actions updates weekly.

```bash
go test -race ./...
go vet ./...
go build ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

Tests use synthetic credentials and local HTTP servers, never your Amazon account.

For the live authentication test and the remaining silent-renewal investigation,
see [the testing guide](docs/authentication-testing.md). `login --verbose` reports
only whether Amazon returned a refresh token; it never prints or stores that token.

## Credits

Inspired by [kindle-send](https://github.com/nikhil1raghav/kindle-send) and based on the Send-to-Kindle API reverse-engineered by [stkclient](https://github.com/maxdjohnson/stkclient).

## Disclaimer

This tool uses an undocumented Amazon API. It is not affiliated with or endorsed by Amazon. The API may change or break at any time.

## License

MIT
