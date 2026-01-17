# Contributing to lokl

Thanks for your interest in contributing to lokl!

## Development Setup

```bash
# Clone the repo
git clone https://github.com/shahin-bayat/lokl.git
cd lokl

# Build
make build

# Run tests
make test

# Format code (required before commits)
make fmt

# Full check (format, build, test, lint)
make check
```

## Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Make your changes
4. Run `make check` to ensure everything passes
5. Commit using [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat(scope): add new feature`
   - `fix(scope): fix bug`
   - `docs: update readme`
   - `refactor: restructure code`
6. Push and open a Pull Request

## Code Style

- Standard Go conventions (gofmt, golint)
- Error wrapping: `fmt.Errorf("doing x: %w", err)`
- Table-driven tests
- Keep packages focused and small

## Pull Requests

- Keep PRs focused on a single change
- Include tests for new functionality
- Update documentation if needed
- All CI checks must pass

## Reporting Issues

- Search existing issues first
- Include steps to reproduce
- Include Go version and OS

## Questions?

Open an issue or start a discussion.
