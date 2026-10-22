# Release Process

1. Ensure `main` is green on CI.
2. Tag the release: `git tag v0.1.0`
3. Push the tag: `git push origin v0.1.0`
4. The release workflow builds binaries for all platforms and creates a GitHub release.

## Versioning

This project follows [Semantic Versioning](https://semver.org/).
