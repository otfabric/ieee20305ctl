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

## When CI runs

| Event | Workflow | What runs |
|---|---|---|
| Pull request | `pr.yml` | `docs.yml` (docsync, otfabric-policy) always; `ci.yml` unless every changed file is documentation; then `OT Fabric Required Checks` |
| Push to a branch | none | nothing; dispatch `ci.yml` to test a branch without a pull request |
| Push to `main` | `docs.yml` | the light checks; the merged commit was tested as the pull request's merge commit |
| Release | `release.yml` | the release workflow tests the release commit again |

Documentation means an added or modified `*.md` file outside `.github/`, `testdata/` and `vendor/`, other than `AGENTS.md` and `CLAUDE.md`. A deleted or renamed document, or any other file, runs everything. `OT Fabric Required Checks` fails unless every job that applies succeeded; its summary lists them.
