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
    health:
      command: "pg_isready -U myproject"  # exec-based check (no HTTP endpoint)
      interval: 2s
      retries: 10
```

### Wildcard subdomains (multi-tenant apps)

```yaml
services:
  web:
    command: php artisan serve
    subdomain:
      - sellify.shop
      - "*.sellify.shop"
    port: 8000
```

`sudo lokl dns setup` writes `/etc/hosts` **and** `/etc/resolver/sellify.shop`; `lokl up` then runs an in-process DNS listener so every subdomain resolves locally. Cert SANs cover both apex and wildcard.

macOS only for now; Linux support (systemd-resolved) is coming in a follow-up release.

### Proxy-only services (one container, multiple HTTPS endpoints)

Some containers expose two HTTP servers on different ports — MinIO's S3 API on 9000 and its console on 9001, Postgres + a pgAdmin sidecar, Jaeger UI on its own port. Declare a second service entry with `proxy_only: true` to route another subdomain to the second port without starting a second container:

```yaml
services:
  minio:
    image: minio/minio:latest
    ports: ["9000:9000", "9001:9001"]
    port: 9000
    subdomain: s3

  minio-console:
    proxy_only: true
    subdomain: console         # → https://console.<domain> → 127.0.0.1:9001
    port: 9001
    depends_on: [minio]
```

A `proxy_only` service doesn't start a process or container — it only forwards. Same pattern works for host-native processes (e.g. a Go server you run manually with `go run`) that you want reachable via HTTPS.

Containers in the same lokl project share a bridge network (`lokl-{name}`).
They can reach each other by service name — no need to expose ports between containers:

```yaml
services:
  api:
    image: myapp:latest
    env:
      DB_HOST: db        # reaches the "db" container directly
      REDIS_HOST: cache  # reaches the "cache" container directly
  db:
    image: postgres:16
  cache:
    image: redis:7
```

### Override a container's command

```yaml
services:
  web:
    image: node:20
    command: "npm run dev"  # overrides image CMD, keeps ENTRYPOINT
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
