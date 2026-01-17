---
title: Installation
description: How to install lokl
---

## Requirements

- macOS or Linux
- Go 1.23+ (for installation from source)

## Install from Source

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
