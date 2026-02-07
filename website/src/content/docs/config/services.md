---
title: Services
description: Configuring services in lokl
---

Services are the core of your lokl configuration. Each service represents a process or container that makes up your development environment.

## Command-based Services

For local processes:

```yaml
services:
  frontend:
    command: pnpm dev
    path: apps/frontend
    port: 5173
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Shell command to run |
| `path` | string | Working directory (relative to config) |
| `port` | int | Port the service listens on |
| `env` | map | Environment variables |
| `env_file` | list | Paths to `.env` files to load |
| `depends_on` | list | Services to start first |
| `autostart` | bool | Start automatically (default: true) |
| `restart` | string | Restart policy: `no`, `always`, `on-failure` |

## Container-based Services

For Docker containers:

```yaml
services:
  db:
    image: postgres:16
    ports:
      - "5432:5432"
    port: 5432
    subdomain: db
    env:
      POSTGRES_PASSWORD: secret
    volumes:
      - ./data:/var/lib/postgresql/data
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Docker image |
| `ports` | list | Port mappings (`host:container`) |
| `port` | int | Host port for proxy routing and health checks |
| `env` | map | Environment variables |
| `env_file` | list | Paths to `.env` files to load |
| `volumes` | list | Volume mounts (`host:container`) |
| `subdomain` | string | Subdomain for proxy routing |
| `depends_on` | list | Services to start first |
| `health` | object | Health check configuration (see below) |
| `autostart` | bool | Start automatically (default: true) |
| `restart` | string | Restart policy: `no`, `always`, `on-failure` |

:::note
- When `port` is set with `ports`, the port value must appear as a host port in one of the mappings.
- Volume container paths must be absolute (e.g., `/var/lib/data`, not `data`).
- Docker must be running to use container-based services.
:::

## Dependencies

Control startup order with `depends_on`:

```yaml
services:
  api:
    command: pnpm dev
    depends_on:
      - db
      - redis

  db:
    image: postgres:16

  redis:
    image: redis:7
```

## Health Checks

Monitor service health:

```yaml
services:
  api:
    command: pnpm dev
    port: 3000
    health:
      path: /health
      interval: 10s
      timeout: 5s
      retries: 3
```

## Environment Variables

### Env Files

Load variables from `.env` files instead of hardcoding secrets:

```yaml
# Global — available to all services
env_file:
  - .env

services:
  api:
    command: pnpm dev
    env_file:
      - .env.local
```

Standard dotenv format: `KEY=VALUE`, `# comments`, blank lines, optional quotes. Paths are relative to `lokl.yaml`.

Inline `env` values take priority over `env_file` values.

### Variable Interpolation

Reference host environment variables with `${VAR}` or `$VAR`:

```yaml
services:
  api:
    command: pnpm dev
    env:
      DATABASE_URL: postgres://${DB_USER}:${DB_PASS}@localhost:5432/mydb
```

Variables are resolved against the host's environment at config load time. Missing variables expand to an empty string.
