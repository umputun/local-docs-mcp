# Contributing to Local Docs MCP Server

Thank you for your interest in contributing to our project!

## Development Setup

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/local-docs-mcp.git`
3. Create a feature branch: `git checkout -b feature-name`
4. Make your changes
5. Run tests: `make test`
6. Run linter: `make lint`
7. Format code: `make fmt`
8. Commit your changes: `git commit -am 'Add feature'`
9. Push to the branch: `git push origin feature-name`
10. Submit a pull request

## Code Style

Please follow the code style guidelines in [CLAUDE.md](CLAUDE.md).

Key points:
- Use lowercase comments inside functions
- No emojis in code or comments
- No historical comments describing changes
- One test file per source file (foo.go → foo_test.go)
- Keep functions under 60 lines when possible
- Maintain 80%+ test coverage

## Pull Request Process

1. Update the README.md with details of changes if applicable
2. The PR should work for all configured platforms and pass all tests
3. PR will be merged once it receives approval from maintainers

## Testing Requirements

- All new code must have tests
- Maintain minimum 80% coverage
- Use table-driven tests where appropriate
- Integration tests for tool implementations

## Migration Status

This project is actively being migrated from Python to Go. See [Migration Plan](docs/plans/migration-plan.md) for current status and planned features.

## Questions?

Open an issue for questions or discussions about contributions.
