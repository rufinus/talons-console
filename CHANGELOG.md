# Changelog

All notable changes to talons-console will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-02-26

### Added

- **Slash commands** — ten interactive commands available during a chat session:
  `/help`, `/agent`, `/session`, `/model`, `/thinking`, `/timeout`,
  `/status`, `/clear`, `/reconnect`, and `/exit` (alias: `/quit`)
- **Tab autocomplete** for slash command names — press `Tab` after `/` to cycle through options
- **Command history navigation** — use the up/down arrow keys to recall previous inputs
- **First-launch hint** — new users see "Type `/help` for available commands" on startup
- **`/status` session info box** — shows connection state, active agent, session key,
  model override, thinking level, message count, and uptime at a glance
- **`/history` reserved name** — placeholder registered with description
  "Browse message history (coming in v0.3)" so tab autocomplete surfaces it early

### Fixed

- Config validation no longer runs gateway checks for the `config-test` and
  `update-check` subcommands, which do not require a live gateway connection
- `ws://` URLs with explicit ports are now parsed correctly; previously a
  missing trailing slash could cause the host to be misidentified

### Changed

- Configuration loading is now split into two stages: `loadConfig` (reads and
  validates the config file) and `requireGatewayConfig` (asserts that gateway
  credentials are present). Commands opt in to the second stage only when needed.
- Session state is managed through a dedicated state object rather than scattered
  individual variables, making agent/session/model changes consistent across the UI.

## [0.1.0] - 2026-01-15

### Added

- Initial release: WebSocket chat client for OpenClaw Gateway
- Token and password authentication
- Full streaming support
- Session and agent selection via CLI flags
- Configuration file support (`~/.config/talons/config.yaml`)
- Cross-platform builds (Linux, macOS, Windows)
