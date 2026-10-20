# mail-reader

A CLI for authenticating with Outlook.com via Microsoft Graph API and downloading emails as JSON for offline processing.

## How it works

Auth uses OAuth 2.0 Authorization Code flow with PKCE against Microsoft's v2.0 endpoint. Access tokens are refreshed automatically using a refresh token saved to `~/.the-mail-reader/config.json`. Emails (including HTML converted to Markdown) are exported as JSON.

## Prerequisites

- Go 1.21+ (this repo is `go 1.26`)
- A Microsoft developer account (free): https://developer.microsoft.com/en-us/microsoft-365/dev-program
- Your Outlook.com / work email account

## One-time Azure setup

1. Go to https://entra.microsoft.com → **App registrations** → **New registration**
   - Name: `mail-reader`
   - **Supported account types**: "Personal Microsoft accounts only" (this is what this tool targets). The tool uses the `consumers` tenant by default to match.
   - Skip redirect URI for now.

2. In your app registration:
   - **Authentication** → **Add platform** → **Mobile and desktop applications**
     - Redirect URI: `http://localhost` (this enables loopback for public clients)
   - Also add a **Web** platform with redirect URI: `http://localhost:8080/callback`
   - Under **Advanced settings**, enable **"Allow public client flows"** if you'd rather not use a secret (recommended for personal use).

3. **API permissions** → **Add a permission** → **Microsoft Graph** → **Delegated permissions** → add:
   - `Mail.Read`
   - `User.Read`

4. Copy the **Application (client) ID** from **Overview**.

Optional (only if you disabled public client flows/set up a Web app): create a **Client secret** under **Certificates & secrets**.

## Build

```
go build -o mailreader .
```

## Usage

```
./mailreader auth    # Authorize (you'll be prompted for your client ID)
./mailreader folders # List your mail folders
./mailreader fetch   # Download all messages from your inbox
```

### Fetch options

```
./mailreader fetch                          # inbox
./mailreader fetch -f sent                  # sent folder
./mailreader fetch -f all                   # every folder
./mailreader fetch -f inbox -s 2026-01-01   # only messages from 2026 onward
./mailreader fetch -f inbox -u 2025-12-31   # only messages up to end of 2025
./mailreader fetch -f "<specific folder name>" -o ./out
./mailreader fetch -m 200                   # page size (default 500)
```

### Output layout

```
emails/
└── individual/              # one JSON file per email — scales to large mailboxes
    ├── <message-id>.json
    └── ...
```

No monolithic combined file — for large inboxes that would be unwieldy. If you want all messages in one streamable file, concatenate them or process the `individual/` directory directly.

Each message looks like:

```json
{
  "id": "AQMkADAwAT...",
  "subject": "Coffee on Thursday?",
  "from": "alice@example.com",
  "to": ["you@outlook.com"],
  "received_at": "2026-09-02T14:11:00Z",
  "body_text": "",
  "body_markdown": "Hey! **Are you free** Thursday? ...",
  "has_attachment": false,
  "attachments": [],
  "folder": "Inbox",
  "read": true,
  "importance": "normal",
  "categories": []
}
```

Note: attachment *metadata* is included; attachment binary content is not downloaded.

## Security notes

- Tokens are stored in `~/.the-mail-reader/config.json` with `0600` permissions.
- The client secret (if any) is stored in the same file — keep it out of version control.
- The tool never uploads your email data anywhere; all Graph requests go straight from your machine to Microsoft.

## Config env vars

| Variable | Default | Purpose |
|---|---|---|
| `MAILREADER_TENANT` | `consumers` | Microsoft tenant for the login endpoint. Use `common` or a tenant ID if your app's audience is "Accounts in any organizational directory" (or + personal accounts). |
| `MAILREADER_REDIRECT_URI` | `http://localhost:8080/callback` | Must exactly match a redirect URI registered in Azure. For a "Mobile and desktop" public client, use just `http://localhost:8080`. |
| `MAILREADER_TIMEOUT` | `300s` | Per-request timeout (e.g. `30m`); full email bodies make pages slow on large mailboxes. |