# Contributing to DevEnv

## Getting Started

```bash
git clone https://github.com/shahin-bayat/devenv.git
cd devenv
make build
make test
```

## Development Workflow

1. Fork the repo
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make changes
4. Run tests: `make test`
5. Run linter: `make lint`
6. Commit using [conventional commits](https://www.conventionalcommits.org/):
   ```
   feat(scope): add feature
   fix(scope): fix bug
   docs: update readme
   ```
7. Push and create a PR

## Code Style

- Run `gofmt` and `goimports`
- Follow standard Go conventions
- Write table-driven tests
- Wrap errors with context

## Questions?

Open an issue or discussion.
