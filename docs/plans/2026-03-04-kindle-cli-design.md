# kindlecli - Design Document

**Date**: 2026-03-04
**Status**: Approved

## Overview

`kindlecli` is a Go CLI tool for sending EPUB and PDF files to your Kindle devices using Amazon's internal Send-to-Kindle API (same API used by Amazon's official desktop apps). No email setup required — authenticate via OAuth2 in your browser.

## Scope

### In scope (v1)
- OAuth2 login via browser (PKCE flow)
- Send EPUB and PDF files to all Kindle devices
- List registered Kindle devices
- Persistent session storage
- Logout

### Out of scope (future)
- Sending URLs/webpages (requires HTML-to-EPUB conversion)
- Email-based fallback
- Device selection (send to specific device)

## CLI Commands

```
kindlecli login          # Open browser for Amazon OAuth2, save session
kindlecli send <file>    # Send epub/pdf to all Kindle devices
kindlecli send *.epub    # Send multiple files (glob expansion)
kindlecli devices        # List your Kindle devices
kindlecli logout         # Logout and delete session
```

### Global flags
- `--config <path>` — alternate config directory (default: `~/.config/kindlecli/`)
- `--verbose` — detailed output

### `send` flags
- `--title <string>` — document title (default: filename without extension)
- `--author <string>` — author (default: empty)

## Architecture

### Project structure

```
kindlecli/
├── cmd/                    # CLI commands (cobra)
│   ├── root.go            # Root command, global flags
│   ├── login.go           # OAuth2 login flow
│   ├── send.go            # File upload + send
│   ├── devices.go         # List devices
│   └── logout.go          # Logout
├── internal/
│   ├── amazon/            # Amazon API client
│   │   ├── auth.go        # OAuth2 + token exchange + device registration
│   │   ├── signer.go      # RSA request signing (X-ADP-Request-Digest)
│   │   ├── stk.go         # SendToKindle API (GetUploadUrl, upload, send, devices)
│   │   └── models.go      # Data structures (DeviceInfo, OwnedDevice, etc.)
│   └── config/
│       └── config.go      # Session persistence (~/.config/kindlecli/)
├── go.mod
├── go.sum
└── main.go
```

### Dependencies
- `github.com/spf13/cobra` — CLI framework
- Go standard library: `crypto/rsa`, `crypto/sha256`, `net/http`, `encoding/xml`, `encoding/json`

### Amazon API Flow

Based on reverse-engineering by [stkclient](https://github.com/maxdjohnson/stkclient).

#### 1. OAuth2 Login (PKCE)
- Generate random verifier + SHA256 challenge
- Build sign-in URL with OpenID Connect params
- User opens URL in browser, logs into Amazon
- Amazon redirects to `amazon.com/gp/sendtokindle?openid.oa2.authorization_code=XXX`
- User pastes redirect URL back into CLI

#### 2. Token Exchange
- POST to `https://api.amazon.com/auth/token`
- Body: authorization_code + code_verifier
- Response: access_token

#### 3. Device Registration
- POST XML to `https://firs-ta-g7g.amazon.com/FirsProxy/registerDeviceWithToken`
- Registers as device type `A1K6D1WRW0MALS` (Send to Kindle desktop app)
- Response: RSA private key (PEM), adp_token, device info

#### 4. Request Signing
All subsequent API requests to `stkservice.amazon.com` are signed:
- Sign data: `method + \n + path + \n + date + \n + body + \n + adp_token`
- SHA256 hash → RSA PKCS1 v1.5 sign with device private key
- Headers: `X-ADP-Request-Digest: <base64_signature>:<date>`, `X-ADP-Authentication-Token: <adp_token>`

#### 5. File Send Flow
1. `POST /GetUploadUrl` → get S3 presigned URL + stk_token
2. `PUT` file to upload URL
3. `POST /GetListOfOwnedDevices` → get all device serial numbers
4. `POST /SendToKindle` → route file to devices with metadata

#### 6. Logout
- `GET /FirsProxy/disownFiona?contentDeleted=false` (signed)
- Delete local auth.json

### API Endpoints
| Endpoint | Purpose |
|----------|---------|
| `https://www.amazon.com/ap/signin` | OAuth2 sign-in page |
| `https://api.amazon.com/auth/token` | Token exchange |
| `https://firs-ta-g7g.amazon.com/FirsProxy/registerDeviceWithToken` | Device registration |
| `https://stkservice.amazon.com/GetListOfOwnedDevices` | List Kindle devices |
| `https://stkservice.amazon.com/GetUploadUrl` | Get S3 upload URL |
| `https://stkservice.amazon.com/SendToKindle` | Send file to devices |
| `https://firs-ta-g7g.amazon.com/FirsProxy/disownFiona` | Logout |

### Client Info (sent with every STK request)
```json
{
    "appName": "ShellExtension",
    "appVersion": "1.1.1.253",
    "os": "MacOSX_10.14.6_x64",
    "osArchitecture": "x64"
}
```

## Security

- RSA private key + adp_token stored in `~/.config/kindlecli/auth.json`
- File permissions: `0600` (owner read/write only)
- `kindlecli logout` deletes local auth and calls remote logout API

## User Experience

### First-time login
```
$ kindlecli login
Opening browser for Amazon login...
Paste the redirect URL here: https://www.amazon.com/gp/sendtokindle?openid...
✓ Logged in successfully. Session saved.
```

### Sending a file
```
$ kindlecli send my-book.epub
Sending my-book.epub (2.3 MB)...
✓ Sent to 2 devices.
```

### Listing devices
```
$ kindlecli devices
1. Juan's Kindle Paperwhite (G000XX123456)
2. Kindle App on iPhone (G000XX789012)
```

## References

- [stkclient](https://github.com/maxdjohnson/stkclient) — Python implementation of the same API
- [kindle-send](https://github.com/nikhil1raghav/kindle-send) — Go CLI using email approach (inspiration for UX)
- [Amazon Send to Kindle](https://www.amazon.com/sendtokindle) — Official web interface
