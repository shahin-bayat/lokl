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
| `env` | map | Environment variables |
| `volumes` | list | Volume mounts |

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
