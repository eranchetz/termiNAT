# termiNATor (NATInspector) - Gemini AI Context

This file provides context for AI agents interacting with the termiNATor codebase.

## Project Overview

**termiNATor** is a Go-based CLI tool designed to help AWS customers identify and quantify avoidable NAT Gateway costs. NAT Gateways charge for data processing, but traffic to services like Amazon S3 and DynamoDB can be routed through free VPC Gateway Endpoints instead. termiNATor analyzes VPC configurations and actual network traffic (via temporary VPC Flow Logs) to detect missing endpoints and calculate potential savings.

### Key Technologies
- **Language**: Go (1.21+)
- **AWS SDK**: `github.com/aws/aws-sdk-go-v2`
- **CLI Framework**: Cobra (`github.com/spf13/cobra`)
- **Terminal UI**: Bubble Tea (`github.com/charmbracelet/bubbletea`), Bubbles (`github.com/charmbracelet/bubbles`)
- **Styling**: Lip Gloss (`github.com/charmbracelet/lipgloss`)
- **AWS Services Used**: EC2 (NAT Gateways, VPC Endpoints, Flow Logs), CloudWatch Logs (Insights queries), STS, IAM

## Architecture & Directory Structure

```
.
├── cmd/              # Cobra CLI commands (root, scan, cleanup)
├── internal/
│   ├── core/         # Core business logic (scanner orchestration)
│   ├── aws/          # AWS clients (EC2, CloudWatch Logs Insights)
│   ├── analysis/     # Traffic classification, stats, cost calculation, endpoint detection
│   ├── datahub/      # DoiT DataHub integration (config, event building, HTTP sending)
│   └── report/       # Markdown/JSON report generation
├── pkg/types/        # Shared data structures (NATGateway, RouteTable, etc.)
├── ui/               # Terminal UI components (Bubble Tea models, report templates)
│   └── templates/    # Go text/template for report body
├── scripts/          # Utility scripts (release, IAM role setup)
└── test/             # E2E testing infrastructure and automation scripts
```

## Building and Running

### Build

```bash
# Build the binary
go build -o terminat .
```

### Run Commands

```bash
# Set your AWS profile and region first
export AWS_PROFILE=your-profile
export AWS_REGION=us-east-1

# 1. Quick Scan (Instant, read-only VPC config check)
./terminat scan quick --region us-east-1

# 2. Deep Dive Scan (Analyzes real traffic, takes ~10 minutes)
./terminat scan deep --region us-east-1 --duration 5

# 3. Demo Mode (Preview report UI with fake data, no AWS creds needed)
./terminat scan demo

# Filter by VPC or specify exact NAT Gateways
./terminat scan deep --region us-east-1 --vpc-id vpc-xxx
./terminat scan deep --region us-east-1 --nat-gateway-ids nat-xxx,nat-yyy
```

### Testing

```bash
# Run unit and integration tests
go test ./...

# Run the complete E2E test suite (Deploys infra, generates traffic, runs scan, cleans up)
./test/scripts/run-e2e-test.sh
```

## Development Conventions & Guidelines

1. **Go Standards**: Follow standard Go practices (Effective Go). Always use `gofmt` to format code.
2. **Context & Cancellation**: Use `context.Context` for all AWS API calls and long-running operations. Gracefully handle interruptions (e.g., `Ctrl+C` should trigger cleanup of Flow Logs).
3. **Adding New AWS Services**: 
   - Add new AWS IP ranges to `internal/analysis/classifier.go`.
   - Update traffic statistics in `internal/analysis/analyzer.go`.
   - Add cost calculations in `internal/analysis/cost.go`.
   - Update the UI and report templates to display the new service.
4. **UI Styling (CRITICAL)**: **NEVER put `\n` inside `lipgloss.Style.Render()` calls.** Lip Gloss treats multi-line content as a block and adds padding/alignment to every line, breaking the layout. Instead, append newlines outside: `style.Render("text") + "\n"`.
5. **Testing Requirements**: Include tests for all new functionality. E2E tests are required for major flow changes to ensure CloudFormation templates, traffic generation scripts, and cleanup mechanisms all work together.
6. **Data & Credentials**: Never log, print, or commit AWS account IDs, secrets, or API keys. 
7. **Reporting**: Report rendering uses `text/template` in `ui/templates/report.tmpl`. Any new output fields must be wired from the analysis models through `ui/report_render.go` into the template data struct.
