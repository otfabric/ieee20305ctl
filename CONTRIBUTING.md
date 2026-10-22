# Contributing

Thank you for your interest in contributing to ieee20305ctl.

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Build
make build

# Run all checks
make check
```

## Pull Requests

1. Fork the repository.
2. Create a feature branch from `main`.
3. Make your changes with tests.
4. Run `make check` to verify everything passes.
5. Open a pull request against `main`.

## Code Style

- Follow standard Go conventions.
- Run `make fmt` before committing.
- All exported symbols must have doc comments.
