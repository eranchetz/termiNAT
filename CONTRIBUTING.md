# Contributing to termiNATor

Thank you for your interest in contributing to termiNATor! This document provides guidelines and instructions for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/terminator.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes
6. Commit with clear messages: `git commit -m "Add feature: description"`
7. Push to your fork: `git push origin feature/your-feature-name`
8. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or later
- AWS CLI configured with credentials
- Access to an AWS account for testing

### Building

```bash
go build -o terminator
```

### Running Tests

```bash
go test ./...
```

### Running Locally

```bash
# Quick scan
go run main.go scan quick --region us-east-1

# Deep dive scan
go run main.go scan deep --region us-east-1 --duration 5
```

## Project Structure

```
terminator/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command setup
│   ├── scan.go            # Scan commands (quick, deep)
│   └── cleanup.go         # Cleanup command
├── internal/
│   ├── core/              # Core business logic
│   │   └── scanner.go     # Main scanner orchestration
│   ├── aws/               # AWS service clients
│   │   ├── ec2.go         # EC2/VPC operations
│   │   └── cloudwatch.go  # CloudWatch Logs operations
│   ├── analysis/          # Traffic analysis
│   │   ├── classifier.go  # IP classification (S3/DynamoDB)
│   │   ├── analyzer.go    # Traffic statistics
│   │   └── cost.go        # Cost calculations
│   └── report/            # Report generation (future)
├── pkg/
│   └── types/             # Shared types
│       └── types.go       # Data structures
├── ui/                    # Terminal UI
│   ├── quick_scan.go      # Quick scan UI
│   └── deep_scan.go       # Deep dive scan UI
└── scripts/               # Utility scripts
    └── setup-flowlogs-role.sh  # IAM role setup
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` to format code
- Add comments for exported functions
- Keep functions focused and small
- Use meaningful variable names

## Testing Guidelines

### Manual Testing

1. **Quick Scan**: Test with VPCs that have and don't have VPC endpoints
2. **Deep Dive Scan**: Test with active NAT Gateway traffic
3. **Cleanup**: Verify log groups are properly deleted
4. **Error Handling**: Test with invalid regions, missing permissions, etc.

### Test Checklist

- [ ] Quick scan discovers NAT Gateways correctly
- [ ] Quick scan identifies missing VPC endpoints
- [ ] Deep dive scan creates Flow Logs successfully
- [ ] Deep dive scan collects and analyzes traffic
- [ ] Cost calculations are accurate
- [ ] Cleanup command works properly
- [ ] Error messages are clear and helpful
- [ ] UI is responsive and informative

## Adding New Features

### Adding a New AWS Service

1. Add IP ranges to `internal/analysis/classifier.go`
2. Update `ClassifyIP()` method
3. Add statistics tracking in `internal/analysis/analyzer.go`
4. Update cost calculations in `internal/analysis/cost.go`
5. Update UI to display new service

### Adding a New Scan Type

1. Create new command in `cmd/`
2. Implement scanner logic in `internal/core/`
3. Create UI component in `ui/`
4. Update README with usage instructions

## Pull Request Guidelines

### PR Title Format

- `feat: Add support for X`
- `fix: Resolve issue with Y`
- `docs: Update documentation for Z`
- `refactor: Improve code structure in W`
- `test: Add tests for V`

### PR Description

Include:
- What changes were made
- Why the changes were necessary
- How to test the changes
- Any breaking changes
- Screenshots (if UI changes)

### Before Submitting

- [ ] Code builds without errors
- [ ] Tests pass
- [ ] Code is formatted with `gofmt`
- [ ] Documentation is updated
- [ ] Commit messages are clear
- [ ] No sensitive data (credentials, account IDs) in code

## Reporting Issues

### Bug Reports

Include:
- termiNATor version
- Go version
- AWS region
- Command executed
- Expected behavior
- Actual behavior
- Error messages
- Steps to reproduce

### Feature Requests

Include:
- Use case description
- Expected behavior
- Why this would be useful
- Potential implementation approach

## Code of Conduct

- Be respectful and inclusive
- Provide constructive feedback
- Focus on the code, not the person
- Help others learn and grow

## Questions?

- Open an issue for questions
- Check existing issues and PRs first
- Be patient and respectful

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
