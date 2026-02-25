# talons-console

A lightweight, fast console client for OpenClaw Gateway. Written in Go — single binary, no dependencies. The slim alternative to the full Node.js TUI for remote OpenClaw access. 🦉

## Overview

**talons** is a community-built console client for the [OpenClaw](https://openclaw.ai) Gateway. It provides a fast, lightweight alternative to the official Node.js TUI, designed for users who want:

- **Single binary distribution** — No Node.js, no npm, no dependencies
- **Fast startup** — Written in Go for instant launch
- **Small footprint** — ~5-10MB vs ~200MB+ for the full TUI
- **Easy cross-platform builds** — Linux, macOS, Windows from one codebase
- **Full protocol compatibility** — Supports all OpenClaw Gateway features

## Features

### Core
- [x] WebSocket connection to OpenClaw Gateway
- [x] Token and password authentication
- [x] Full streaming support (no polling)
- [x] Session management (switch agents, sessions)
- [x] History persistence

### UI (Planned)
- [ ] Rich terminal UI with Bubble Tea
- [ ] Collapsible tool output cards
- [ ] Model picker
- [ ] Agent/session switcher
- [ ] Slash command support
- [ ] File drop/attachment support

## Installation

### From Source
```bash
go install github.com/rufinus/talons-console@latest
```

### Pre-built Binaries
Download from [Releases](https://github.com/rufinus/talons-console/releases)

### Homebrew (Future)
```bash
brew install rufinus/tap/talons
```

## Usage

### Connect to local Gateway
```bash
talons
```

### Connect to remote Gateway
```bash
talons --url ws://192.168.9.125:8080 --token <your-token>
```

### With password auth
```bash
talons --url ws://gateway.local:8080 --password <your-password>
```

### Specify session
```bash
talons --session main --agent main
```

## Configuration

talons looks for configuration in:
- `~/.config/talons/config.yaml`
- Environment variables (e.g., `TALONS_URL`, `TALONS_TOKEN`)
- Command-line flags

Example config:
```yaml
gateway:
  url: ws://192.168.9.125:8080
  token: your-token-here
  
session:
  agent: main
  key: main
  
defaults:
  deliver: true
  thinking: medium
```

## Development

### Prerequisites
- Go 1.21+
- OpenClaw Gateway (for testing)

### Build
```bash
go build -o talons ./cmd/talons
```

### Test
```bash
go test ./...
```

### Cross-compile
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o talons-linux ./cmd/talons

# macOS
GOOS=darwin GOARCH=amd64 go build -o talons-darwin ./cmd/talons

# Windows
GOOS=windows GOARCH=amd64 go build -o talons.exe ./cmd/talons
```

## Architecture

talons is built with:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Styling
- [gorilla/websocket](https://github.com/gorilla/websocket) — WebSocket client
- [cobra](https://github.com/spf13/cobra) — CLI framework

## Protocol

talons speaks the OpenClaw Gateway WebSocket protocol directly:
- Connects to `ws://<host>:<port>/` (or `wss://` for TLS)
- Authenticates via token or password
- Sends/receives JSON messages
- Handles event streaming for real-time updates

See [PROTOCOL.md](docs/PROTOCOL.md) for detailed protocol documentation.

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License — see [LICENSE](LICENSE) for details.

## Acknowledgments

- [OpenClaw](https://openclaw.ai) — The awesome AI assistant framework this client connects to
- [Charm](https://charm.sh) — Beautiful terminal UI libraries
- The OpenClaw community for making this possible

## Community

- GitHub Discussions: https://github.com/rufinus/talons-console/discussions
- Issues: https://github.com/rufinus/talons-console/issues

---

Built with 🦉 by the OpenClaw community.
