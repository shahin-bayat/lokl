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

## Toggle Proxy

In the TUI, press `p` to toggle between:
- **Local** — Direct connection to service
- **Remote** — Through HTTPS proxy
