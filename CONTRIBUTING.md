# Contributing to Bazinga

Thank you for considering contributing to Bazinga! This document outlines the process for contributing to the project.

## Development Setup

```bash
git clone https://github.com/tildaslashlef/bazinga.git
cd bazinga
make setup        # Install dependencies and tools
make test         # Run all tests
make build        # Build the binary
make lint         # Run linting checks
make clean        # Clean build artifacts
```

## Available Make Targets

- `make build` - Build the binary
- `make install` - Build and install to system
- `make test` - Run all tests with coverage
- `make lint` - Run golangci-lint
- `make clean` - Clean build artifacts
- `make setup` - Install development dependencies
- `make help` - Show all available targets

## Coding Standards

- Follow Go best practices and idiomatic Go code style
- Use the project's linting configuration (`make lint`)
- Write tests for new features and bug fixes
- Maintain or improve code coverage (`make test`)

## Contribution Workflow

### For External Contributors:
1. Fork the repository to your GitHub account
2. Clone your fork: `git clone https://github.com/YOUR-USERNAME/bazinga.git`
3. Add the original repo as upstream: `git remote add upstream https://github.com/tildaslashlef/bazinga.git`
4. Create a feature branch: `git checkout -b feature/your-feature-name`
5. Make your changes and commit them
6. Keep your fork in sync: `git fetch upstream && git merge upstream/main`
7. Push to your fork: `git push origin feature/your-feature-name`
8. Create a pull request from your fork to the main repository

### For Regular Contributors with Write Access:
1. Clone the repository directly: `git clone https://github.com/tildaslashlef/bazinga.git`
2. Create a feature branch: `git checkout -b feature/your-feature-name` 
3. Make your changes and commit them
4. Push your branch: `git push origin feature/your-feature-name`
5. Create a pull request from your branch to the main branch

### For All Pull Requests:
1. Add or update tests for new functionality
2. Update documentation as needed
3. Run tests (`make test`) and linting (`make lint`)
4. Submit your PR with a clear description of the changes
5. Respond to review feedback if requested

## Commit Guidelines

- Use clear, descriptive commit messages
- Reference issue numbers in commit messages when applicable
- Keep commits focused on a single logical change

## Security Considerations

- Be careful with tool permissions and security settings
- Never enable the `terminator` option in production code
- Consider security implications of any new tools or features

## Documentation

- Update documentation for any changed functionality
- Add examples for new features
- Keep the ARCHITECTURE.md document in sync with code changes

## Code Review

All submissions require review before being merged. As this is a personal project:

- PR responses may take some time (please be patient)
- Constructive feedback will be provided when possible
- Help with improving submissions will be offered when time allows

## License

By contributing to Bazinga, you agree that your contributions will be licensed under the project's [MIT License](LICENSE).

## Questions?

If you have questions about contributing, feel free to open an issue or join our community discussion.

Thank you for helping make Bazinga better!
