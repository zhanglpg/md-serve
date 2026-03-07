# MD Serve

A lightweight HTTP server that renders Markdown files as a styled website. Supports Obsidian-compatible syntax including wikilinks, callouts, highlights, and comments. Serve one directory or multiple vaults side by side.

## Features

- Serves `.md` and `.markdown` files as styled HTML pages
- **Multi-vault support** — serve multiple directories as named vaults with a landing page
- Obsidian syntax support: `[[wikilinks]]`, `==highlights==`, `%%comments%%`, callouts
- Wiki links resolve case-insensitively with space/hyphen interoperability (`[[My Page]]` finds `my-page.md`)
- Callout types: note, tip, info, warning, danger, example, success, failure, bug, abstract, question, quote
- Table of Contents sidebar generated from headings
- Full-text search across all markdown files (filterable by vault)
- Directory browsing with breadcrumb navigation
- Automatic `index.md` / `README.md` serving for directories
- Syntax highlighting for code blocks (Dracula theme)
- GitHub Flavored Markdown tables and footnotes
- KaTeX math and Mermaid diagram rendering
- Static file serving (images, PDFs, etc.) with proper MIME types
- Dark mode support (auto-detects system preference)
- Responsive design

## Installation

```bash
go build -o md-serve .
```

## Usage

```bash
# Serve current directory on port 8080
./md-serve

# Serve a specific directory with custom port and title
./md-serve -dir /path/to/docs -port 3000 -title "My Wiki"

# Serve multiple vaults (each gets its own URL namespace and landing-page card)
./md-serve -dir notes=/path/to/notes -dir wiki=/path/to/wiki
```

### Command-line Flags

| Flag     | Default    | Description                              |
|----------|------------|------------------------------------------|
| `-dir`   | `.`        | Directory to serve. Repeatable. Use `name=path` for named vaults. |
| `-port`  | `8080`     | Port to listen on                        |
| `-title` | `MD Serve` | Site title displayed in the navigation   |

### Multi-Vault Mode

Pass multiple `-dir` flags to serve several directories as independent vaults:

```bash
./md-serve -dir docs=/srv/docs -dir blog=/srv/blog -title "My Site"
```

- The root URL (`/`) shows a landing page with a card for each vault.
- Each vault is accessible under its name prefix (e.g. `/docs/`, `/blog/`).
- Search spans all vaults by default; add `&vault=docs` to restrict results.

## Running as a Service at Startup

### systemd (Linux)

Create a service unit file at `/etc/systemd/system/md-serve.service`:

```ini
[Unit]
Description=MD Serve - Markdown file server
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/path/to/your/docs
ExecStart=/usr/local/bin/md-serve -dir /path/to/your/docs -port 8080 -title "My Docs"
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then enable and start the service:

```bash
# Copy the binary to a system-wide location
sudo cp md-serve /usr/local/bin/

# Reload systemd, enable on boot, and start
sudo systemctl daemon-reload
sudo systemctl enable md-serve
sudo systemctl start md-serve

# Check status
sudo systemctl status md-serve

# View logs
sudo journalctl -u md-serve -f
```

### macOS (launchd)

Create a plist file at `~/Library/LaunchAgents/com.md-serve.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.md-serve</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/md-serve</string>
        <string>-dir</string>
        <string>/path/to/your/docs</string>
        <string>-port</string>
        <string>8080</string>
        <string>-title</string>
        <string>My Docs</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/md-serve.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/md-serve.err</string>
</dict>
</plist>
```

Then load the service:

```bash
launchctl load ~/Library/LaunchAgents/com.md-serve.plist

# To stop and unload
launchctl unload ~/Library/LaunchAgents/com.md-serve.plist
```

### Windows (Task Scheduler)

1. Open **Task Scheduler** (`taskschd.msc`)
2. Click **Create Basic Task**
3. Set the trigger to **When the computer starts**
4. Set the action to **Start a program** and configure:
   - **Program:** `C:\path\to\md-serve.exe`
   - **Arguments:** `-dir C:\path\to\docs -port 8080 -title "My Docs"`
   - **Start in:** `C:\path\to\docs`
5. In the task properties, check **Run whether user is logged on or not**

Alternatively, use PowerShell:

```powershell
$action = New-ScheduledTaskAction -Execute "C:\path\to\md-serve.exe" `
  -Argument "-dir C:\path\to\docs -port 8080 -title `"My Docs`"" `
  -WorkingDirectory "C:\path\to\docs"
$trigger = New-ScheduledTaskTrigger -AtStartup
Register-ScheduledTask -TaskName "MD Serve" -Action $action -Trigger $trigger `
  -RunLevel Highest -User "SYSTEM"
```

### Docker

```dockerfile
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /md-serve .

FROM alpine:latest
COPY --from=build /md-serve /usr/local/bin/md-serve
EXPOSE 8080
ENTRYPOINT ["md-serve"]
CMD ["-dir", "/docs", "-port", "8080"]
```

Run with:

```bash
docker build -t md-serve .
docker run -d -p 8080:8080 -v /path/to/docs:/docs --restart unless-stopped md-serve
```

## Running Tests

```bash
go test ./...
```

## License

MIT
