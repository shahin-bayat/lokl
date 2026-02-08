<div align="center">

# lokl

**One command to rule them all.**

Define your entire local dev environment in a single file. Start everything with `lokl up`.

[![Build](https://github.com/shahin-bayat/lokl/actions/workflows/ci.yml/badge.svg)](https://github.com/shahin-bayat/lokl/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/shahin-bayat/lokl/branch/main/graph/badge.svg)](https://codecov.io/gh/shahin-bayat/lokl)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docs](https://img.shields.io/badge/Docs-lokl-purple)](https://shahin-bayat.github.io/lokl/)

<img src="assets/demo.gif" alt="lokl demo" width="700">

</div>

---

## Why lokl?

New developer joins your team. Instead of spending a day setting up their environment:

```bash
lokl up
```

That's it. Frontend, backend, databases, HTTPS routing — all running.

## Features

🚀 **Single config file** — Define all services in `lokl.yaml`

🔐 **Automatic HTTPS** — Generated certificates for custom domains (`app.myproject.dev`)

🔄 **Process management** — Health checks, dependency ordering, auto-restart

🐳 **Docker support** — Run databases and caches as containers alongside your services

🔑 **Env files & interpolation** — Load secrets from `.env` files, reference with `${VAR}`

🖥️ **Interactive TUI** — Start/stop services, view logs, toggle proxy

🔍 **Project detection** — `lokl init` scans your project and generates config

## Installation

**Homebrew (macOS/Linux):**
```bash
brew install shahin-bayat/tap/lokl
```

**One-liner:**
```bash
curl -fsSL https://raw.githubusercontent.com/shahin-bayat/lokl/main/install.sh | bash
```

**Go install:**
```bash
go install github.com/shahin-bayat/lokl/cmd/lokl@latest
```

## Quick Start

```bash
# Initialize config from your project
lokl init

# Start your environment
lokl up
```

## Example Config

```yaml
name: my-project
version: "1"

proxy:
  domain: myproject.dev

services:
  frontend:
    command: pnpm dev
    path: apps/frontend
    port: 5173
    subdomain: app

  api:
    command: pnpm dev
    path: apps/api
    port: 3000
    subdomain: api
    env:
      DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/myproject
    depends_on:
      - db

  db:
    image: postgres:16
    ports:
      - "5432:5432"
    port: 5432
    env_file:
      - .env
    env:
      POSTGRES_DB: myproject
```

Then:
- `https://app.myproject.dev` → frontend (port 5173)
- `https://api.myproject.dev` → api (port 3000)

## Requirements

- macOS or Linux
- Go 1.25+ (for installation from source)
- Docker (for container-based services)

## License

[MIT](LICENSE)
