---
title: lokl up
description: Start the development environment
---

Start all services defined in your config file.

## Usage

```bash
lokl up [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `-c, --config` | Config file path (default: `lokl.yaml`) |
| `-d, --detach` | Run without TUI (background mode) |

## Examples

Start with TUI:

```bash
lokl up
```

Start in background:

```bash
lokl up --detach
```

Use custom config:

```bash
lokl up -c custom.yaml
```

## Log colors

lokl preserves ANSI colors from your tools (vite, nx, etc.) in the TUI log
viewer. Because service output is captured through a pipe, tools would
normally detect a non-TTY and disable color — lokl sets `FORCE_COLOR=1` in
each process service's environment to keep it on.

Cursor- and screen-control escapes (hard resets, screen clears) are still
stripped so misbehaving tools can't corrupt the TUI.

To opt out, set `NO_COLOR` in your shell or in a service's `env:` block —
lokl then skips `FORCE_COLOR` injection entirely. A service-level
`FORCE_COLOR` in `env:` always overrides lokl's value.
