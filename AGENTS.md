# Repository guidance

kindlecli is a Go CLI that sends files through Amazon's undocumented
Send-to-Kindle API. Read [README.md](README.md) for commands and
[docs/authentication-testing.md](docs/authentication-testing.md) for live testing.

## Code navigation

<!-- CODEGRAPH_START -->
In repositories indexed by CodeGraph (a `.codegraph/` directory exists at the
repository root), use CodeGraph before text search or reading source to understand
or locate code. Prefer `codegraph_explore` when available, or run
`codegraph explore "<symbol names or question>"`. If there is no `.codegraph/`
directory, skip CodeGraph; do not create an index without the user's request.
<!-- CODEGRAPH_END -->

- `cmd/` contains Cobra commands, session state handling and batch behavior.
- `internal/amazon/` contains authentication, request signing, HTTP operations and
  response validation. This is a device-registration protocol, not generic LWA.
- `internal/config/` contains atomic JSON persistence. Reuse `SaveJSON` for private
  state instead of overwriting credentials in place.
- Keep tests beside the code. Prefer the standard library and existing dependencies.

## Authentication invariants

- Reuse saved credentials while Amazon accepts them. File existence and elapsed
  time alone do not establish session validity or expiry.
- `auth status --json` returns `valid`, `missing`, `invalid`, `rejected`, or
  `unavailable`; only `valid` exits with code 0. Do not equate network errors or
  generic HTTP 403 responses with rejected credentials.
- Recover through `login`, not a preliminary `logout`. Preserve the saved session
  until replacement credentials have been received, validated and saved.
- `login --start --json` returns `valid` or `pending`. For `pending`, the browser
  handoff ends with the redirect URL passed to stdin of `login --finish --json`.
  Both commands must use the same config directory. A new start replaces the
  pending attempt; its PKCE verifier expires after 10 minutes.
- Browser automation belongs to the calling skill or agent. Keep password/MFA
  interaction in the user's browser and handle redirect URLs locally, without
  shell interpolation or exposing them in chat, logs or command-line arguments.
- `.login-lock` serializes login/logout. Remove a stale lock only after confirming
  no authentication process is running. Do not delete unrelated config files.
- Silent token renewal is not implemented or proven. Verbose output reports only
  whether a refresh token was returned. Do not infer support from generic Login
  with Amazon documentation or add an unverified refresh/localhost callback flow.
- The legacy registration serial is fixed. Different `--config` directories do
  not guarantee independent device registrations on Amazon.

## Delivery and HTTP behavior

- All discovered devices are targets. Do not send a real document just to test
  authentication; `auth status` and `devices` suffice.
- Invalid inputs and partial/all-file failures must produce a nonzero exit code.
  Preserve successes in the summary and stop the remaining batch on session
  rejection. Never retry an entire partially successful batch automatically.
- API acceptance does not prove delivery to a Kindle. Connection failures during
  delivery can leave acceptance uncertain, so avoid automatic delivery retries.
- Check both HTTP status and the service's JSON status/required fields. Keep
  credential rejection separate from upload URL errors and service failures.
- API calls use a 30-second timeout; uploads default to 10 minutes and accept
  `--upload-timeout`. Leave compression negotiation to Go's HTTP transport.
- Do not follow redirects with signed requests or expose response bodies, keys,
  tokens or signed upload URLs in errors. Preserve useful error classification.

## Validation and documentation

For code changes, run the relevant regression tests, then the checks appropriate
to the change:

```bash
gofmt -w <changed-go-files>
go test -race ./...
go vet ./...
go build ./...
```

For dependency/security changes, also run:

```bash
go mod tidy
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

Use a current stable Go toolchain while preserving the minimum declared in
`go.mod` unless a change requires raising it. CI also tests the minimum version
and stable Go on Linux, macOS and Windows. Tests must use synthetic credentials
and mocked HTTP/local servers, never the user's account. Live verification is a
separate step; report clearly when it still needs user interaction.

Documentation-only changes need consistency/link checks, not new tests. Update
README command/flag tables and the authentication guide when changing their
contracts. Keep generated binaries and all credential/pending files out of Git.
Follow the user's requested commit and publication workflow.

## Local installation for Hermes

Hermes uses `kindlecli` from PATH, not the repository's `./kindlecli` binary.
When the task includes updating the CLI used on this machine, run `go install .`
after validation, inspect `command -v kindlecli` and verify the installed command.
Use `go version -m` to check its build revision. Do not treat `go build` alone as
installation. Check for PATH shadowing before creating another binary location.
An existing Hermes process must resolve the same installation; do not assume its
PATH if it reports a different version. `@latest` follows release tags and can
lag behind main, so install the validated checkout for local development.
