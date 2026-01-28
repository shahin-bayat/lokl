---
title: Installation
description: How to install lokl
---

## Requirements

- macOS or Linux (Windows not supported)

## Homebrew (Recommended)

```bash
brew install shahin-bayat/tap/lokl
```

## One-liner Script

```bash
curl -fsSL https://raw.githubusercontent.com/shahin-bayat/lokl/main/install.sh | bash
```

This downloads the latest release and installs to `/usr/local/bin` (or `~/.local/bin` if no write access).

To install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/shahin-bayat/lokl/main/install.sh | bash -s -- v0.1.0
```

## Install from Source

Requires Go 1.23+:

```bash
go install github.com/shahin-bayat/lokl/cmd/lokl@latest
```

Make sure `$GOPATH/bin` is in your `PATH`.

## Verify Installation

```bash
lokl --version
```

## Shell Completions

### Bash

```bash
lokl completion bash > /etc/bash_completion.d/lokl
```

### Zsh

```bash
lokl completion zsh > "${fpath[1]}/_lokl"
```

### Fish

```bash
lokl completion fish > ~/.config/fish/completions/lokl.fish
```
