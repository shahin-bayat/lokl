---
title: Quick Start
description: Get up and running in minutes
---

## Auto-detect your project

The easiest way to get started is to let lokl detect your project structure:

```bash
cd your-project
lokl init
```

This scans your project, detects services, and generates a `lokl.yaml` config file.

## Or create manually

Create a `lokl.yaml` in your project root:

```yaml
name: my-project
version: "1"

proxy:
  domain: myproject.dev

services:
  app:
    command: pnpm dev
    port: 3000
```

## Start your environment

```bash
lokl up
```

This will:
1. Start all services
2. Generate HTTPS certificates
3. Configure local DNS
4. Open the interactive TUI

## DNS Setup

For custom domains to work, run once:

```bash
sudo lokl dns setup
```

This adds entries to `/etc/hosts`.

## Access your services

With the config above, your app is available at:

```
https://myproject.dev
```

## TUI Controls

| Key | Action |
|-----|--------|
| `j/k` | Navigate services |
| `s` | Start service |
| `x` | Stop service |
| `r` | Restart service |
| `l` | Toggle logs |
| `p` | Toggle proxy |
| `q` | Quit |
