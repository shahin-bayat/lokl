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
