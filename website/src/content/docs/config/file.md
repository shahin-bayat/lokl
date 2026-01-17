---
title: Config File
description: lokl.yaml configuration reference
---

lokl uses a YAML configuration file, typically named `lokl.yaml`, to define your development environment.

## Basic Structure

```yaml
name: my-project
version: "1"

proxy:
  domain: myproject.dev

env:
  NODE_ENV: development

services:
  service-name:
    command: ...
    port: ...
```

## Top-level Fields

### `name`

**Required.** Project name used for identification and DNS markers.

```yaml
name: my-awesome-project
```

### `version`

Config file version. Currently `"1"`.

```yaml
version: "1"
```

### `proxy`

Proxy configuration for HTTPS routing.

```yaml
proxy:
  domain: myproject.dev
```

### `env`

Global environment variables inherited by all services.

```yaml
env:
  NODE_ENV: development
  DEBUG: "true"
```

### `services`

Map of service definitions. See [Services](/lokl/config/services/) for details.
