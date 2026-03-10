# Xiaohongshu CLI

A command-line tool for interacting with Xiaohongshu (小红书) API.

## Installation

```bash
go install github.com/saulyip/xiaohongshu-cli@latest
```

Or build from source:

```bash
git clone https://github.com/saulyip/xiaohongshu-cli.git
cd xiaohongshu-cli
go build -o xiaohongshu-cli ./cmd/xiaohongshu-cli
```

## Usage

### Authentication

Login via QR code:

```bash
xiaohongshu-cli auth login
```

Check authentication status:

```bash
xiaohongshu-cli auth status
```

Logout:

```bash
xiaohongshu-cli auth logout
```

### Feed

List homepage feed:

```bash
xiaohongshu-cli feed list
```

### Search

Search for posts:

```bash
xiaohongshu-cli search "keyword"
```

With filters:

```bash
# Filter by type: all, image, video, user
xiaohongshu-cli search "keyword" --filter image

# Sort by: general, hot, time
xiaohongshu-cli search "keyword" --sort hot
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--json` | Output JSON instead of human-readable text | `false` |
| `--store` | Store directory for session data | `~/.xiaohongshu-cli` |

## Development

```bash
# Build
go build -o xiaohongshu-cli ./cmd/xiaohongshu-cli

# Run
./xiaohongshu-cli --help
```

## License

MIT
