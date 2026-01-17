---
title: Introduction
description: What is lokl and why use it
---

lokl is a CLI/TUI tool that allows developers to define their entire local development environment in a single configuration file.

## The Problem

Setting up a local development environment is painful:

- **Time-consuming** — New developers spend hours/days configuring their setup
- **Inconsistent** — "Works on my machine" syndrome
- **Complex HTTPS** — OAuth, cookies, CORS often require HTTPS with custom domains
- **Scattered config** — Docker Compose, npm scripts, env files, nginx configs everywhere

## The Solution

lokl provides:

1. **Single config file** — All services defined in `lokl.yaml`
2. **Automatic HTTPS** — Certificates generated and trusted for custom domains
3. **Process orchestration** — Health checks, dependencies, restart policies
4. **Interactive TUI** — Visual management of all services

## Example

```yaml
name: my-project

proxy:
  domain: myproject.dev

services:
  frontend:
    command: pnpm dev
    port: 5173
    subdomain: app

  api:
    command: pnpm dev
    port: 3000
    subdomain: api
```

Then run:

```bash
lokl up
```

Access your services at:
- `https://app.myproject.dev` → frontend
- `https://api.myproject.dev` → api
