---
title: Proxy & HTTPS
description: Configure HTTPS routing with custom domains
---

lokl includes a built-in reverse proxy that provides automatic HTTPS for your services.

## Basic Setup

```yaml
proxy:
  domain: myproject.dev

services:
  frontend:
    command: pnpm dev
    port: 5173
    subdomain: app
```

This makes the frontend available at `https://app.myproject.dev`.

## How It Works

1. **Certificate Generation** — lokl generates self-signed certificates for your domain
2. **Trust Store** — Certificates are added to your system trust store
3. **DNS** — Entries added to `/etc/hosts` for local resolution
4. **Routing** — Requests are proxied to the appropriate service based on subdomain

## Subdomains

Assign subdomains to services:

```yaml
services:
  frontend:
    port: 5173
    subdomain: app      # → https://app.myproject.dev

  api:
    port: 3000
    subdomain: api      # → https://api.myproject.dev

  admin:
    port: 4000
    subdomain: admin    # → https://admin.myproject.dev
```

## Root Domain

A service without a subdomain gets the root domain:

```yaml
services:
  main:
    port: 3000
    # No subdomain → https://myproject.dev
```

## Path Rewriting

For SPA routing or API prefixes:

```yaml
services:
  api:
    port: 3000
    subdomain: api
    rewrite:
      strip_prefix: /v1
      fallback: /index.html
```

## DNS Management

Setup DNS entries:

```bash
sudo lokl dns setup
```

Remove DNS entries:

```bash
sudo lokl dns remove
```

## Wildcard subdomains

Multi-tenant apps (Laravel tenancy, Rails tenancy, per-customer previews) assign subdomains at runtime. `subdomain` accepts a list so one service can answer every tenant:

```yaml
services:
  web:
    command: php artisan serve
    subdomain:
      - sellify.shop
      - "*.sellify.shop"
    port: 8000
```

### How it works

- `sudo lokl dns setup` writes `/etc/hosts` plus `/etc/resolver/sellify.shop` so macOS forwards wildcard lookups to lokl.
- `lokl up` starts an in-process DNS listener on `127.0.0.1:5454` that answers every subdomain with `127.0.0.1`.
- The proxy cert's SAN list covers both the apex (`sellify.shop`) and the wildcard (`*.sellify.shop`) — TLS works for every tenant.

### Limits

- `*` must be the leftmost label: `"*.x.y"` is valid; `"a.*.y"` and `"*"` are not.
- Reserved parents (`*.com`, `*.org`, `*.net`, `*.local`, `*.test`, `*.localhost`) are rejected.
- macOS only for now. Linux support (systemd-resolved) lands in a follow-up release.

## Toggle Proxy

In the TUI, press `p` to toggle between:
- **Local** — Direct connection to service
- **Remote** — Through HTTPS proxy
