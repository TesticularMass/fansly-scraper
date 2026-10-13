# Fansly Scraper

> [!WARNING]
> **‼️ Windows Defender False Positive** (literally no idea why or where this came from)
>
> The Windows executable of this program is frequently being flagged as **`Trojan:Script/Wacatac.B!ml`** by Windows Defender and other AV. This is a **false positive**.
>
> **Why is this happening?**
> The `!ml` in the detection name means it was flagged by a Machine Learning model, not a human. Because this is an unsigned, open-source scraper tool that makes many internet requests, the automated system incorrectly flags it as suspicious. This is a common issue for many Go and Rust-based command-line tools.
>
> **Recommended Solution - Install via Go (should hopefully bypass AV issues):**
> ```bash
> GOPROXY=direct go install github.com/TesticularMass/fansly-scraper/cmd/fansly-scraper@latest
> ```
>
> **What you can do:**
> 1.  **Trust the program:** You can temporarily disable Windows Defender to download and run the program, and then add it as an exception.
> 2.  **Build from source:** If you have Go installed, you can build it yourself. The program is not flagged when you compile it locally or install via go.
>
> The code is fully open-source for you to inspect. It is safe to use. but I understand if the AV warnings are concerning.

A simple all in one tool to download and monitor content from Fansly creators.

> **⚠️ Currently in development** - Some features may not work perfectly. See [known issues](#known-issues) below.


## Requirements

- **Optional but highly recommended**: [ffmpeg](https://ffmpeg.org/) for livestream recording and saving higher quality videos. Follow [this guide](https://phoenixnap.com/kb/ffmpeg-windows) for installing to system path on windows.
- **Optional**: [mt](https://github.com/mutschler/mt) for better contact sheets

## Quick Start

### 1. Download
- **Install via Go**:
```bash
GOPROXY=direct go install github.com/TesticularMass/fansly-scraper/cmd/fansly-scraper@latest
```

### 2. Run the Program
```bash
./fansly-scraper
```

On first run, the setup wizard helps you configure everything. Press 'a' to use auto login: it opens Fansly and provides a one‑line snippet to paste in DevTools Console. Your token and user‑agent are captured automatically and saved to the config.

## Basic Usage

### Interactive Mode (Recommended for beginners)
```bash
./fansly-scraper
```

From the main menu you can:
- Run setup wizard (choose save location, auto login)
- Reset configuration (restore defaults, re-run wizard)

### Command Line Mode
```bash
# Download all content from a creator
./fansly-scraper -u {creator-name}

# Download specific content types
./fansly-scraper -u {creator-name} -d [all|timeline|messages|stories]

# Download a specific Wall (timeline)
./fansly-scraper -u {creator-name} -w {wall-id}

# Download a single post by ID or URL
./fansly-scraper -p {post-id-or-url}

# Limit download to the X most recent items
./fansly-scraper -u {creator-name} -l 10

# Export text chat logs to JSON
./fansly-scraper -u {creator-name} --dump-chat-log

# Use multiple accounts / profiles
./fansly-scraper --profile account1 -u {creator-name}
./fansly-scraper --profile account2 -u {creator-name}

# Use a custom config file
./fansly-scraper --config /path/to/custom-config.toml -u {creator-name}

# Monitor for live streams
./fansly-scraper -m {creator-name}

# Start/stop monitoring
./fansly-scraper monitor [start|stop]

# Run diagnosis & environment checks
./fansly-scraper --diagnosis
```

**Note**: Live monitoring requires keeping your terminal session active. To run monitoring in the background, consider using terminal multiplexers like [tmux](https://github.com/tmux/tmux/wiki) or [zellij](https://github.com/zellij-org/zellij) on Linux/WSL. Starting from v0.6.3, you can monitor additional creators by running `-m creator` in separate terminal instances without restarting the existing monitor process.

### Update the Program
```bash
./fansly-scraper update
```

## Configuration & Storage

Detailed options and configuration parameters are documented in [config.md](./config.md).

### Configuration & Database Locations
- **Config file**: `~/.config/fansly-scraper/config.toml` (Windows: `%APPDATA%\fansly-scraper\config.toml`).
- **Database file**: `~/.config/fansly-scraper/downloads.db`. The database is stored next to the configuration to prevent SQLite advisory lock failures on network/SMB shares.
- **Profiles**: When running with `--profile <name>`, both configuration and database are stored inside `~/.config/fansly-scraper/profiles/<name>/`.

### Custom Post Date Filename Templates
Output file names can be customized in `config.toml` under `[options]`:
```toml
[options]
use_content_as_filename = true
date_format = "2006-01-02"
content_filename_template = "{date}_{model_name}_{postId}_{index}"
```
Available template placeholders: `{date}`, `{model_name}`, `{postId}`, `{mediaId}`, `{index}`, `{content}`.

## Build from source
```bash
git clone https://github.com/TesticularMass/fansly-scraper && cd fansly-scraper
go build -v -ldflags "-w -s" -o fansly-scraper ./cmd/fansly-scraper
```

## Contributing

Contributions are welcome and greatly appreciated! Whether it's reporting a bug, proposing a feature, or submitting a pull request, your help is valued.

![alt text](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)

![alt text](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat-square)

Please read our [Contributing Guidelines](./CONTRIBUTING.md) before you start.
Check the [Issues tab](https://github.com/TesticularMass/fansly-scraper/issues) to see if there's a task you'd like to take on.

## Known Issues

- **Redownloading**: As of v0.8.3, file naming convention was changed causing redownloading of media.
- **Chat recording**: May occasionally miss messages during reconnections
- **MacOS users**: May need to allow notifications in [system settings](https://github.com/gen2brain/beeep/issues/67#issuecomment-2646474049)
- **Live status**: Press `r` in TUI to refresh live status
- **Duplicate files**: Sometimes the same image may appear twice due to Fansly's API structure
- **Date formats for livestream filename**: In the event a stream gets interrupted and reattempts to record, if the date format isn't specific enough (ie, no timestamp), it may fail to save the stream after as both vods will be the same name, for now use one of these: `2006-01-02_15:04:05` or `20060102_150405`

---

> [!CAUTION]
> **Disclaimer**: This tool is not affiliated with Fansly or Select Media LLC. Use at your own risk. The developer of this script is not responsible for the end users' actions or any outcomes that may be taken upon the end users' account. Use at your own risk.
