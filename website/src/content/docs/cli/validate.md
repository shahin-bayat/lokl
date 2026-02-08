---
title: lokl validate
description: Validate the configuration file
---

Check your config file for errors without starting any services.

## Usage

```bash
lokl validate [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `-c, --config` | Config file path (default: `lokl.yaml`) |

## Examples

Validate default config:

```bash
lokl validate
# ✓ lokl.yaml is valid
```

Validate custom config:

```bash
lokl validate -c staging.yaml
```

## What it checks

- YAML syntax
- Environment file resolution and variable interpolation
- Required fields (service name, command or image)
- Port conflicts (duplicate ports across services)
- Dependency resolution (unknown or circular dependencies)
- Port mapping format (for Docker services)

## CI usage

Add to your CI pipeline to catch config errors early:

```yaml
- name: Validate lokl config
  run: lokl validate
```
