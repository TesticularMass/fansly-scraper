# Fansly Scraper

A command-line and interactive terminal tool for downloading Fansly content and monitoring livestreams. This fork includes account profiles, downloads from multiple creators, chat exports, and configurable filenames.

## Requirements

- Go 1.25 or newer and Git to build from source.
- A Fansly account with access to the content you want to download.
- `ffmpeg` on your `PATH` for livestream recording and combining segmented videos; `ffprobe` for built-in contact sheets.
- Optional: `mt` for contact sheets when `use_mt_for_contact_sheet` is enabled.

## Quick Start

### 1. Build this fork

```bash
git clone https://github.com/TesticularMass/fansly-scraper.git
cd fansly-scraper
go build -v -ldflags "-w -s" -o fansly-scraper ./cmd/fansly-scraper
```

The module still uses the upstream path, `github.com/agnosto/fansly-scraper`. Build from the cloned repository to use this fork's code.

On Windows, use `-o fansly-scraper.exe` and run `.\fansly-scraper.exe` in the examples below.

### 2. Run the setup wizard

```bash
./fansly-scraper
```

On first run, the setup wizard helps configure your save location and authentication. Press `a` for auto login: it opens Fansly and provides a snippet to paste into the browser's DevTools Console. Your token and user-agent are captured and saved to the configuration.

## Usage

### Interactive mode

Run without arguments to open the terminal interface. Its menus let you:

- Download a creator's timeline, messages, stories, or individual posts.
- Download purchased content.
- Select creators for livestream monitoring.
- Like or unlike a creator's posts.
- Run the setup wizard, edit configuration, or reset it.

### Command-line downloads

Replace `creator`, IDs, and paths with your own values.

```bash
# Download all supported content types for a creator
./fansly-scraper -u creator

# Download multiple creators
./fansly-scraper -u creator1,creator2,creator3 -d all

# Download one content type: timeline, messages, or stories
./fansly-scraper -u creator -d messages

# Download a specific wall (defaults to timeline only)
./fansly-scraper -u creator -w 793928278367805440

# Include messages and stories alongside a specific wall
./fansly-scraper -u creator -w 793928278367805440 -d all

# Download a single post by ID or URL
./fansly-scraper -p 837364635748286464

# Override the configured post limit
./fansly-scraper -u creator -l 10

# Export text chat history to JSON
./fansly-scraper -u creator1,creator2 --dump-chat-log

# Use a named account profile or custom configuration
./fansly-scraper --profile account1 -u creator
./fansly-scraper --config /path/to/config.toml -u creator

# Show all options or run configuration/API diagnostics
./fansly-scraper --help
./fansly-scraper --diagnosis
```

`-d all` includes timeline, messages, and stories, plus profile images when enabled in configuration. Purchased content has a separate interactive menu option. Place flags before subcommands such as `monitor`.

### Livestream monitoring

```bash
# Toggle creators in the saved monitoring list
./fansly-scraper -m creator1,creator2

# Run the monitoring process in the current terminal
./fansly-scraper monitor start

# Stop it from another terminal
./fansly-scraper monitor stop
```

Repeating `-m creator` removes that creator from the monitoring list. Keep the monitoring process running, or use a persistent terminal session such as tmux or Zellij. You can toggle additional creators from another terminal while it runs. When using a profile, supply the same `--profile` before `monitor start`, `monitor stop`, and `-m`.

## Configuration & Storage

See [config.md](./config.md) for detailed options and [example-config.toml](./example-config.toml) for a configuration example.

Default configuration directories:

| Platform | Directory |
| --- | --- |
| Linux | `$XDG_CONFIG_HOME/fansly-scraper`, or `~/.config/fansly-scraper` when unset |
| macOS | `~/.config/fansly-scraper` |
| Windows | `%APPDATA%\fansly-scraper` |

- **Configuration:** `config.toml` in the configuration directory. A `config.toml` in the working directory takes precedence when neither `--config` nor `--profile` is supplied.
- **Database:** `downloads.db` normally lives in the configuration directory so media can be saved on a network share without placing SQLite there. An older database in the download location is imported when needed, with the original kept as `downloads.db.bak`. If migration fails or the configuration directory is not writable, the application can fall back to the download location.
- **Profiles:** `--profile account1` keeps configuration, database, and monitoring state under `profiles/account1/` in the default configuration directory.
- **Custom paths:** `--config /path/to/config.toml` takes precedence over `--profile` and uses the supplied file's directory for database and monitoring state. A working-directory config discovered automatically does not change the database directory.

### Post filename templates

```toml
[options]
use_content_as_filename = true
date_format = "2006-01-02"
content_filename_template = "{date}_{model_name}_{postId}_{index}"
```

Available placeholders: `{date}`, `{model_name}`, `{postId}`, `{mediaId}`, `{index}`, and `{content}`. Date formats use Go's reference date, as in the example above.

## Updating This Fork

From your clone, pull and rebuild:

```bash
git pull --ff-only
go build -v -ldflags "-w -s" -o fansly-scraper ./cmd/fansly-scraper
```

The built-in `update` command and update checks target `agnosto/fansly-scraper` releases. Running that command can replace this fork's executable with an upstream release. To disable those checks, set `check_updates = false` under `[options]` in your configuration.

## Development

Run the automated tests and static checks from the repository root:

```bash
go test ./...
go vet ./...
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) for contribution guidance. Report bugs and feature requests in this fork's [issue tracker](https://github.com/TesticularMass/fansly-scraper/issues).

## Known Limitations

- Changes to filename templates or naming conventions can cause existing content to download again.
- Chat recording may miss messages during reconnections.
- macOS notifications may require permission in system settings.
- Press `r` in the monitoring interface to refresh live status.
- Duplicate media can appear in API responses.
- Include a timestamp in livestream filenames, for example `2006-01-02_15-04-05`, to distinguish recordings after interruptions.

This project is not affiliated with Fansly or Select Media LLC. Use it at your own risk.
