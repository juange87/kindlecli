# kindlecli

A command-line tool to send documents directly to your Kindle devices using Amazon's Send-to-Kindle API. No email configuration needed.

## Features

- **Direct upload** — sends files through Amazon's internal Send-to-Kindle API (the same one used by the official desktop apps)
- **Multiple formats** — EPUB, PDF, MOBI, AZW, AZW3, DOC, DOCX, TXT, RTF, HTML
- **Batch send** — send multiple files in a single command
- **All devices** — automatically sends to every Kindle device on your account
- **Single binary** — no runtime dependencies, just one Go binary

## Installation

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

This opens your browser for Amazon OAuth2 login. After signing in, you'll be redirected to the Send to Kindle page. Copy the full URL from the address bar, then press Enter in the terminal to read it from your clipboard.

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

This deregisters the device from your Amazon account and deletes the local session.

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
| `login` | Authenticate with Amazon via browser OAuth2 |
| `send` | Upload and send files to all Kindle devices |
| `devices` | List all Kindle devices on your account |
| `logout` | Deregister device and delete local session |

### Global Flags

| Flag | Description |
|------|-------------|
| `--config <dir>` | Config directory (default: `~/.config/kindlecli`) |
| `--verbose` | Enable verbose output for debugging |

### Send Flags

| Flag | Description |
|------|-------------|
| `--title <string>` | Document title (default: filename without extension) |
| `--author <string>` | Document author (default: "Unknown") |

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

```
kindlecli/
├── main.go                          # Entry point
├── cmd/
│   ├── root.go                      # Cobra root command, global flags
│   ├── login.go                     # OAuth2 browser login flow
│   ├── send.go                      # File upload and delivery
│   ├── devices.go                   # List Kindle devices
│   └── logout.go                    # Deregister and cleanup
└── internal/
    ├── amazon/
    │   ├── models.go                # Data types (DeviceInfo, OwnedDevice, etc.)
    │   ├── models_test.go           # XML/JSON parsing tests
    │   ├── auth.go                  # OAuth2 PKCE, token exchange, device registration
    │   ├── signer.go                # RSA request signing (custom PKCS#1 v1.5)
    │   ├── signer_test.go           # Signing tests
    │   └── stk.go                   # Send-to-Kindle API client
    └── config/
        ├── config.go                # Session persistence (~/.config/kindlecli/)
        └── config_test.go           # Config save/load tests
```

## Security

- Session credentials are stored at `~/.config/kindlecli/auth.json` with `0600` permissions (owner read/write only)
- The RSA private key never leaves your machine
- `kindlecli logout` deregisters the device remotely and deletes local credentials
- No credentials are logged or transmitted to third parties

## Credits

Inspired by [kindle-send](https://github.com/nikhil1raghav/kindle-send) and based on the Send-to-Kindle API reverse-engineered by [stkclient](https://github.com/maxdjohnson/stkclient).

## Disclaimer

This tool uses an undocumented Amazon API. It is not affiliated with or endorsed by Amazon. The API may change or break at any time.

## License

MIT
