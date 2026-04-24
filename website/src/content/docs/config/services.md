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
| `command` | string or list | Shell command (string → `sh -c`) or exec args (list). Allowed with `image` to override the image's default command. |
| `path` | string | Working directory (relative to config) |
| `port` | int | Port the service listens on |
| `env` | map | Environment variables |
| `env_file` | list | Paths to `.env` files to load |
| `depends_on` | list | Services to start first |
| `autostart` | bool | Start automatically (default: true) |
| `restart` | string | Restart policy: `no`, `always`, `on-failure` |
| `proxy_only` | bool | Register the subdomain route without starting a process or container. Requires `port` and `subdomain`. See [Proxy-only services](#proxy-only-services). |

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
| `command` | string or list | Override the image's default `CMD`. String → `sh -c` inside container; list → direct exec. |
| `ports` | list | Port mappings (`host:container`) |
| `port` | int | Host port for proxy routing and health checks |
| `env` | map | Environment variables |
| `env_file` | list | Paths to `.env` files to load |
| `volumes` | list | Volume mounts. `host:container` for bind mounts, `name:container` for named volumes, bare `/container/path` for anonymous volumes (mask a dir from an outer bind mount). |
| `subdomain` | string or list of strings | Subdomain for proxy routing. A list may mix exact names and wildcards like `"*.sellify.shop"` for multi-tenant apps (macOS only for now). |
| `depends_on` | list | Services to start first |
| `health` | object | Health check configuration (see below) |
| `autostart` | bool | Start automatically (default: true) |
| `restart` | string | Restart policy: `no`, `always`, `on-failure` |
| `proxy_only` | bool | Register the subdomain route without starting a process or container. Requires `port` and `subdomain`. See [Proxy-only services](#proxy-only-services). |

:::note
- When `port` is set with `ports`, the port value must appear as a host port in one of the mappings.
- Volume container paths must be absolute (e.g., `/var/lib/data`, not `data`).
- "Mask" volumes (bare container path, e.g. `- /var/www/html/vendor`) hide a directory from an outer bind mount — useful for `vendor/` or `node_modules/` inside a source bind. lokl backs these with deterministically-named Docker volumes — `lokl-<project>-mask-<service>-<escaped-path>` when a project network is in use, falling back to `lokl-mask-<service>-<escaped-path>` otherwise — so the contents survive container recreation. Remove manually via `docker volume rm` when you want to reset.
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

### HTTP check (`health.path`)

Monitor HTTP services:

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

### Overriding a container's command

`command` is allowed alongside `image`. It overrides the image's default `CMD` while preserving `ENTRYPOINT` — matching `docker run image cmd...` and Docker Compose's `command:` field.

```yaml
services:
  api:
    image: node:20
    command: "npm run dev"          # string → sh -c inside the container
  worker:
    image: node:20
    command: ["node", "worker.js"]  # list → direct exec, no shell
```

**Caveat:** images with an `ENTRYPOINT` (e.g. `python`, `node`, `postgres`) prepend it to the command. Shell-form `command: "script.py"` on a `python` image becomes `python /bin/sh -c "script.py"` — use the list form or a custom image when this matters.

**Requirement for shell form:** the image must contain `/bin/sh` (most do; `scratch` and some distroless images do not).

**Signal handling:** with shell form, `/bin/sh` is PID 1. `docker stop` sends `SIGTERM` to `sh`, which may not forward it to the child — the container waits out the stop grace period before `SIGKILL`. Use the list form for fast graceful shutdown on a single binary.

### Exec check (`health.command`)

For data services that have no HTTP endpoint (databases, caches):

```yaml
services:
  db:
    image: postgres:16
    health:
      command: "pg_isready -U myapp"   # string → sh -c wrap
      interval: 2s
      timeout: 1s
      retries: 10

  cache:
    image: redis:7
    health:
      command: ["redis-cli", "ping"]   # array → exec directly (no shell)
      interval: 1s
      retries: 5
```

`path` and `command` are mutually exclusive — use one or the other.

## Container Networking

Containers in the same lokl project share a bridge network (`lokl-{name}`).
They can reach each other by service name as hostname — no need to expose ports between containers:

```yaml
services:
  api:
    image: myapp:latest
    env:
      DB_HOST: db        # reaches the "db" container directly
      REDIS_HOST: cache  # reaches the "cache" container directly
    depends_on:
      - db
      - cache
  db:
    image: postgres:16
    health:
      command: "pg_isready -U myapp"
      retries: 10
  cache:
    image: redis:7
    health:
      command: ["redis-cli", "ping"]
      retries: 5
```

The network is created on `lokl up` and removed on `lokl down`.

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

## Proxy-only services

A service entry with `proxy_only: true` registers a subdomain route without starting a process or container. Useful for:

- **One container, multiple HTTPS endpoints** — MinIO API + console, Postgres + pgAdmin, Jaeger UI on a second port.
- **Host-native processes** — a Go server or Python script you run manually; lokl gives it a trusted HTTPS URL without managing its lifecycle.

```yaml
services:
  minio:
    image: minio/minio:latest
    ports: ["9000:9000", "9001:9001"]
    port: 9000
    subdomain: s3

  minio-console:
    proxy_only: true
    subdomain: console
    port: 9001
    depends_on: [minio]
```

### Required fields

- `port` — the target port lokl forwards to (always `127.0.0.1:<port>` over HTTP).
- `subdomain` — scalar or list; wildcards like `"*.x.test"` work the same as for regular services.

### Allowed optional fields

- `depends_on` — wait for another service to be healthy before probing.
- `rewrite.strip_prefix` / `rewrite.fallback` — path aliasing, same as normal services.
- `health.path` — HTTP readiness probe instead of the default TCP probe.

### Disallowed fields

`command`, `image`, `ports`, `volumes`, `env`, `env_file`, `autostart`, `restart`, `ready_timeout`, `limits`, `health.command`. Validation rejects these with a clear error naming the field.

### Notes

- The target port can belong to a lokl-managed container (via its `ports:` publish) or to a host-native process the user runs themselves — lokl just dials `127.0.0.1:<port>`.
- The duplicate-port validator ignores proxy-only services — they forward, not bind.
- In the TUI, proxy-only services appear in the service list with a TCP-probe-backed status dot. The restart key is a no-op (no process to restart). The log panel shows a static banner.
