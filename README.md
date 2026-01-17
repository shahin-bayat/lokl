<div align="center">

# lokl

**One command to rule them all.**

Define your entire local dev environment in a single file. Start everything with `lokl up`.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docs](https://img.shields.io/badge/Docs-lokl-purple)](https://shahin-bayat.github.io/lokl/)

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

🖥️ **Interactive TUI** — Start/stop services, view logs, toggle proxy

🔍 **Project detection** — `lokl init` scans your project and generates config

## Quick Start

```bash
# Install (macOS/Linux)
go install github.com/shahin-bayat/lokl/cmd/lokl@latest

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
    depends_on:
      - db

  db:
    image: postgres:16
    ports:
      - "5432:5432"
    env:
      POSTGRES_PASSWORD: secret
```

Then:
- `https://app.myproject.dev` → frontend (port 5173)
- `https://api.myproject.dev` → api (port 3000)

## Requirements

- macOS or Linux
- Go 1.23+ (for installation from source)

## License

[MIT](LICENSE)
