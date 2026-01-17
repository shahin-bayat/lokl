---
title: lokl dns
description: Manage DNS entries
---

Manage `/etc/hosts` entries for custom domains.

## Commands

### dns setup

Add DNS entries for your configured domains.

```bash
sudo lokl dns setup
```

This adds entries like:

```
# lokl:my-project
127.0.0.1 myproject.dev
127.0.0.1 app.myproject.dev
127.0.0.1 api.myproject.dev
# lokl:my-project:end
```

### dns remove

Remove DNS entries.

```bash
sudo lokl dns remove
```

After removal, flush your DNS cache:

**macOS:**
```bash
sudo dscacheutil -flushcache && sudo killall -HUP mDNSResponder
```

**Linux:**
```bash
sudo systemd-resolve --flush-caches
```

## Why sudo?

Modifying `/etc/hosts` requires root privileges. lokl only touches entries within its own markers (`# lokl:project-name`).
