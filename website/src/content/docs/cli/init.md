---
title: lokl init
description: Initialize a new lokl.yaml from project detection
---

Scan your project and generate a `lokl.yaml` configuration file.

## Usage

```bash
lokl init
```

## What It Does

1. **Scans** your project directory for services
2. **Detects** package manager (pnpm, yarn, npm, bun)
3. **Finds** runnable scripts in package.json files
4. **Infers** ports from scripts or known tool defaults
5. **Prompts** for script selection and configuration
6. **Generates** a clean `lokl.yaml` file

## Supported Detection

Currently supports Node.js projects:

- Monorepos with `apps/` and `packages/` directories
- Single-app projects
- Package managers: pnpm, yarn, npm, bun

## Example Session

```
$ lokl init
Scanning project...

Detected 2 service(s):
  - api (apps/api)
  - frontend (apps/frontend)

─── api ───
  Available scripts:
    [1] dev
    [2] start
    [3] test
    [0] Skip this service
  Select script [1]: 1
  Port (required): 3000

─── frontend ───
  Available scripts:
    [1] dev
    [2] build
    [0] Skip this service
  Select script [1]: 1
  Port [5173]: 

Base domain [my-project.dev]: 

✓ Created lokl.yaml
✓ Run 'lokl up' to start your environment
```

## Port Detection

Ports are inferred from:

1. **Script flags** — `--port 3000`, `-p 3000`, `PORT=3000`
2. **Tool defaults** — vite (5173), next (3000), etc.
3. **Manual input** — Prompted if not detected
