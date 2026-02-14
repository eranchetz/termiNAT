# termiNATor - Agent Guide

This guide is optimized for AI agents (like Claude, GPT, etc.) to understand, run, test, and contribute to termiNATor.

## Project Overview

**Purpose**: Analyze AWS NAT Gateway traffic to identify cost savings by detecting services that should use free VPC Gateway Endpoints instead.

**Key Insight**: NAT Gateways charge $0.045/GB for data processing. S3 and DynamoDB Gateway Endpoints are FREE. This tool finds traffic going through NAT that should use endpoints.

**Tech Stack**: Go 1.21+, AWS SDK, Bubble Tea (TUI), Lip Gloss (styling), CloudWatch Logs Insights

## Quick Context

### What This Tool Does

1. **Quick Scan**: Checks VPC config for missing endpoints (instant, read-only)
2. **Deep Dive Scan**: Creates temporary Flow Logs, analyzes real traffic, calculates savings (10+ minutes)
3. **Demo Mode**: Preview the report UI with realistic fake data, no AWS credentials needed

### Architecture

```
terminator/
├── cmd/                    # CLI commands (cobra)
│   ├── root.go            # Root command, version handling
│   └── scan.go            # Scan commands (quick/deep/demo)
├── internal/
│   ├── core/              # Business logic
│   │   └── scanner.go     # Main scanner orchestration
│   ├── aws/               # AWS clients
│   │   ├── ec2.go         # NAT Gateway, Flow Logs, VPC endpoints
│   │   └── cloudwatch.go  # Logs Insights queries
│   ├── analysis/          # Traffic analysis
│   │   ├── classifier.go  # Classify traffic by AWS service
│   │   ├── analyzer.go    # Traffic statistics
│   │   ├── cost.go        # Cost calculations
│   │   └── endpoints.go   # VPC endpoint analysis
│   ├── datahub/           # DoiT DataHub integration
│   │   ├── config.go      # TOML config (~/.terminat/config.toml)
│   │   └── datahub.go     # Event building, batched HTTP send with retry
│   └── report/            # Report generation
│       └── report.go      # Markdown/JSON export
├── ui/                    # Terminal UI (Bubble Tea)
│   ├── quick_scan.go      # Quick scan UI
│   ├── deep_scan.go       # Deep dive scan UI with progress + NAT selection
│   ├── report_render.go   # Template-based report renderer
│   ├── demo.go            # Demo mode with fake data
│   ├── format_test.go     # formatCurrency tests
│   └── templates/
│       └── report.tmpl    # Go text/template for report body
├── pkg/types/             # Shared types
├── scripts/
│   ├── release.sh         # Build + upload release binaries
│   └── setup-flowlogs-role.sh
└── test/                  # E2E testing infrastructure
    ├── infrastructure/    # CloudFormation templates
    └── scripts/           # Test automation scripts
```

### Key Files to Understand

1. **internal/core/scanner.go** - Main orchestrator, calls AWS clients and analysis
2. **internal/analysis/classifier.go** - Downloads AWS IP ranges, classifies traffic
3. **ui/deep_scan.go** - Bubble Tea model for interactive scan with progress + NAT selection
4. **ui/report_render.go** - Template-based report renderer (builds data, executes template)
5. **ui/templates/report.tmpl** - Go text/template for the report body layout
6. **internal/analysis/endpoints.go** - VPC endpoint detection and route analysis
7. **internal/datahub/datahub.go** - DoiT DataHub event building and HTTP send

## Running the Tool

### Prerequisites Check

```bash
# Verify Go version
go version  # Need 1.21+

# Verify AWS access
aws sts get-caller-identity

# Set environment
export AWS_PROFILE=your-profile
export AWS_REGION=us-east-1
```

### Build and Run

```bash
# Build
go build -o terminat .

# Quick scan (instant)
./terminat scan quick --region us-east-1

# Deep dive scan (10 minutes)
./terminat scan deep --region us-east-1 --duration 5

# Demo mode (no AWS credentials needed)
./terminat scan demo

# Filter by VPC
./terminat scan deep --region us-east-1 --vpc-id vpc-xxx

# Scan specific NATs
./terminat scan deep --region us-east-1 --nat-gateway-ids nat-xxx,nat-yyy
```

## E2E Testing

### Automated (Recommended)

```bash
export AWS_PROFILE=your-test-profile
export AWS_REGION=us-east-1
./test/scripts/run-e2e-test.sh
```

**What it does**: Deploys test infra → builds → generates traffic → runs scan → verifies → cleans up

**Time**: ~18 minutes  
**Cost**: ~$2.77 (mostly Flow Logs ingestion)

### Manual Steps

```bash
# 1. Deploy test infrastructure
./test/scripts/deploy-test-infra.sh

# 2. Build
go build -o terminat .

# 3. Start traffic (MUST run during scan, not before)
./test/scripts/continuous-traffic.sh start

# 4. Run scan
./terminat scan deep --region us-east-1 --duration 5

# 5. Stop traffic
./test/scripts/continuous-traffic.sh stop

# 6. Cleanup
./test/scripts/cleanup.sh
```

### Verification

After E2E test, check:
- ✅ S3 traffic > 0% (should be ~38%)
- ✅ DynamoDB traffic > 0% (should be ~10%)
- ✅ 2 missing endpoints detected
- ✅ Remediation commands generated
- ✅ Flow Logs stopped automatically

## Code Contribution Guide

### Adding a New Feature

**Example: Add support for analyzing Interface VPC Endpoints**

1. **Add AWS client method** (`internal/aws/ec2.go`):
```go
func (c *EC2Client) DescribeInterfaceEndpoints(ctx context.Context, vpcID string) ([]InterfaceEndpoint, error) {
    // Implementation
}
```

2. **Add analysis logic** (`internal/analysis/endpoints.go`):
```go
func (ea *EndpointAnalysis) AnalyzeInterfaceEndpoints() {
    // Check for missing interface endpoints
}
```

3. **Update scanner** (`internal/core/scanner.go`):
```go
func (s *Scanner) AnalyzeVPCEndpoints(ctx context.Context, vpcID string) (*analysis.EndpointAnalysis, error) {
    // Call new analysis method
}
```

4. **Update UI** (`ui/deep_scan.go`):
```go
// Add interface endpoint info to report
```

5. **Add tests**:
```go
func TestAnalyzeInterfaceEndpoints(t *testing.T) {
    // Test cases
}
```

### Code Style

- Use `gofmt` for formatting
- Follow Go best practices (effective Go)
- Add comments for exported functions
- Keep functions small and focused
- Use context for cancellation

### Testing Strategy

1. **Unit tests**: Test individual functions
2. **Integration tests**: Test AWS client interactions (mocked)
3. **E2E tests**: Full workflow with real AWS resources

## Common Tasks

### Debug Traffic Classification

**Issue**: Traffic showing 0% for S3/DynamoDB

**Check**:
```bash
# 1. Verify AWS IP ranges downloaded
# Look in classifier.go - DownloadAWSIPRanges()

# 2. Check Flow Logs query
# Look in cloudwatch.go - QueryFlowLogs()

# 3. Verify traffic is running during scan
./test/scripts/continuous-traffic.sh status
```

**Fix**: Traffic must run DURING scan, not before. Flow Logs only capture traffic after they start.

### Add New AWS Service Classification

**Example: Add RDS classification**

1. **Update classifier.go**:
```go
type TrafficClassifier struct {
    S3Ranges      []net.IPNet
    DynamoRanges  []net.IPNet
    RDSRanges     []net.IPNet  // Add this
}

func (tc *TrafficClassifier) ClassifyIP(ip string) string {
    // Add RDS check
    if tc.isInRanges(ipAddr, tc.RDSRanges) {
        return "RDS"
    }
}
```

2. **Update analyzer.go**:
```go
type TrafficStats struct {
    S3Bytes      int64
    DynamoBytes  int64
    RDSBytes     int64  // Add this
}
```

3. **Update cost.go**:
```go
// Add RDS cost calculations
```

4. **Update UI to display RDS stats**

### Modify Flow Logs Query

**Location**: `internal/aws/cloudwatch.go` - `QueryFlowLogs()`

**Current query**:
```sql
fields @timestamp, srcaddr, dstaddr, bytes, packets
| filter dstaddr not like /^10\./
| filter dstaddr not like /^172\.(1[6-9]|2[0-9]|3[0-1])\./
| filter dstaddr not like /^192\.168\./
| stats sum(bytes) as total_bytes, count(*) as records by srcaddr, dstaddr
```

**To add protocol filtering**:
```sql
fields @timestamp, srcaddr, dstaddr, bytes, packets, protocol
| filter protocol = 6  # TCP only
| filter dstaddr not like /^10\./
...
```

### Add New Scan Type

**Example: Add "historical" scan using CloudWatch metrics**

1. **Add command** (`cmd/scan.go`):
```go
var historicalCmd = &cobra.Command{
    Use:   "historical",
    Short: "Analyze historical NAT Gateway metrics",
    Run:   runHistoricalScan,
}
```

2. **Add scanner method** (`internal/core/scanner.go`):
```go
func (s *Scanner) HistoricalScan(ctx context.Context, days int) (*HistoricalAnalysis, error) {
    // Query CloudWatch metrics
}
```

3. **Add UI** (`ui/historical_scan.go`):
```go
// Bubble Tea model for historical scan
```

## Debugging

### Enable Verbose Logging

```bash
# Add to scanner.go
import "log"

func (s *Scanner) AnalyzeTraffic(...) {
    log.Printf("Querying Flow Logs: %s", logGroupName)
    log.Printf("Time range: %d to %d", startTime, endTime)
}
```

### Test Individual Components

```bash
# Test AWS client
go test ./internal/aws/...

# Test classifier
go test ./internal/analysis/...

# Test specific function
go test -run TestClassifyIP ./internal/analysis/
```

### Check Flow Logs Data

```bash
# Get log group from scan output
LOG_GROUP="/aws/vpc/flowlogs/terminat-1234567890"

# Query directly
aws logs start-query \
  --log-group-name $LOG_GROUP \
  --start-time $(date -u -d '10 minutes ago' +%s) \
  --end-time $(date -u +%s) \
  --query-string 'fields @timestamp, srcaddr, dstaddr, bytes'
```

## Important Implementation Details

### Flow Logs Lifecycle

1. **Creation**: `scanner.CreateFlowLogs()` - Creates on NAT Gateway ENI
2. **Startup delay**: 5 minutes (AWS requirement for Flow Logs to start delivering)
3. **Collection**: User-specified duration (5-60 minutes)
4. **Analysis**: CloudWatch Logs Insights query
5. **Cleanup**: `scanner.DeleteFlowLogs()` - Always called, even on interrupt

**Critical**: Signal handler in `ui/deep_scan.go` ensures cleanup on Ctrl+C

### Traffic Classification Algorithm

1. Download AWS IP ranges from `https://ip-ranges.amazonaws.com/ip-ranges.json`
2. Parse S3 and DynamoDB prefixes
3. For each Flow Log record:
   - Check if destination IP is in S3 ranges → classify as "S3"
   - Check if destination IP is in DynamoDB ranges → classify as "DynamoDB"
   - Otherwise → classify as "Other"
4. Sum bytes per classification

**Location**: `internal/analysis/classifier.go`

### Cost Calculation

```
Monthly Cost = (Bytes / 1GB) × $0.045 × (30 days / sample duration)
```

**Example**: 100 GB in 5 minutes
- Hourly: 100 GB × 12 = 1,200 GB/hour
- Daily: 1,200 GB × 24 = 28,800 GB/day
- Monthly: 28,800 GB × 30 = 864,000 GB/month
- Cost: 864,000 × $0.045 = $38,880/month

**Location**: `internal/analysis/cost.go`

### VPC Endpoint Detection

1. Query VPC endpoints: `ec2:DescribeVpcEndpoints`
2. Filter by VPC ID and service name (s3, dynamodb)
3. Check route tables: `ec2:DescribeRouteTables`
4. Identify route tables without endpoint routes
5. Generate remediation commands

**Location**: `internal/analysis/endpoints.go`

## Known Issues & Gotchas

### 1. Flow Logs Delay
**Issue**: Flow Logs need 5-10 minutes to start delivering data  
**Solution**: Built-in 5-minute startup delay in deep scan

### 2. Traffic Timing
**Issue**: Traffic generated before scan won't be captured  
**Solution**: Use `continuous-traffic.sh` to run traffic DURING scan

### 3. CloudWatch Logs Insights Limits
**Issue**: Query results limited to 10,000 records  
**Solution**: Use aggregation in query (`stats sum(bytes)`)

### 4. Regional Pricing
**Issue**: NAT Gateway pricing varies by region  
**Solution**: Hardcoded to $0.045/GB (most regions), needs enhancement for accurate regional pricing

### 5. Flow Logs IAM Role
**Issue**: Flow Logs need IAM role with CloudWatch Logs permissions  
**Solution**: `scripts/setup-flowlogs-role.sh` creates required role

## Performance Considerations

- **Flow Logs ingestion**: ~$0.50/GB (AWS charges)
- **CloudWatch Logs storage**: ~$0.03/GB/month
- **Query performance**: Logs Insights can handle millions of records
- **Memory usage**: Minimal, streams data from CloudWatch

## Future Enhancements

Potential areas for contribution:

1. **Historical Analysis**: Use CloudWatch metrics instead of Flow Logs
2. **Multi-account**: Scan across AWS Organizations
3. **Cost Explorer Integration**: Compare with actual bills
4. **Automated Remediation**: Create endpoints automatically
5. **Regional Pricing**: Accurate pricing per region
6. **Savings Plans**: Factor in existing commitments
7. **Parallelize VPC analysis**: Use errgroup for concurrent VPC endpoint checks
8. **Multi-AZ traffic detection**: Detect cross-AZ patterns, suggest NAT per AZ
9. **Flow Logs data freshness**: Verify timestamps are within expected range

## Recent Updates (2026-02-08)

### v0.6.0 — Template Report, NAT Selection, DataHub, Demo

#### Template-Based Report Rendering
- Report layout moved from Go code to `ui/templates/report.tmpl` (Go `text/template`)
- `ui/report_render.go` contains the renderer: `buildReportData()`, `renderReportBody()`, `renderFooter()`
- Template uses custom FuncMap: `{{green "text"}}`, `{{warn "text"}}`, `{{header "TITLE"}}`, `{{currency 123.45}}`, `{{indent cmd}}`
- Fixes lipgloss alignment bugs — **never put `\n` inside `Style.Render()` calls**; lipgloss treats multi-line content as a block and adds padding

#### Scrollable Viewport with Mouse Support
- Report renders into `charmbracelet/bubbles/viewport` for scrolling (↑↓ PgUp PgDn)
- Mouse wheel scrolling via `tea.WithMouseCellMotion()` + `tea.WithAltScreen()`
- `tea.MouseMsg` forwarded to viewport in `Update()`
- Footer stays fixed below viewport

#### Interactive NAT Gateway Selection (closes #26)
- New `phaseSelectingNATs` phase between discovery and approval
- Shown when multiple NATs found and no `--nat-gateway-ids` filter
- UI: cursor-based multi-select with checkboxes
- Keys: `↑/↓` move, `space` toggle, `a` select all, `enter` confirm
- New `--vpc-id` flag filters NATs by VPC before selection
- Skipped with `--auto-approve` or explicit `--nat-gateway-ids`

#### DoiT DataHub Integration
- `internal/datahub/config.go` — TOML config parser (`~/.terminat/config.toml`), no external deps
- `internal/datahub/datahub.go` — `BuildEvents()` creates 5 events per NAT, `Send()` with 429 retry + 255-batch
- `apiURL` is `var` (not `const`) so tests can override it
- CLI flags: `--doit-datahub-api-key`, `--doit-customer-context`
- Interactive: press `D` in report view to send, with save-to-config prompt
- Resolution precedence: CLI flag > env var > config file

#### Demo Command
- `terminat scan demo` — creates `deepScanModel` at `phaseDone` with realistic fake data
- Uses `tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())`
- No AWS credentials needed — for visual QA and development

#### Release Script (`scripts/release.sh`)
- Usage: `./scripts/release.sh v0.7.0`
- Requires version argument (no more defaulting to `dev`)
- Builds all 5 platforms (darwin/linux amd64/arm64 + windows)
- Injects version via `-ldflags "-s -w -X main.Version=<version>"`
- Verifies version string in local binary before uploading
- Auto-creates GitHub release if it doesn't exist, uploads with `--clobber`

#### Bug Fixes
- `formatCurrency`: `"$%,.2f"` → `"$%.2f"` (`,` flag unsupported by `message.Printer`)
- ECR cost line added to both TUI report and markdown export

#### Tests (22 new)
- `internal/datahub/config_test.go` — 8 tests (parsing, save/load, precedence)
- `internal/datahub/datahub_test.go` — 10 tests (BuildEvents, Send, retry, batching)
- `ui/format_test.go` — `TestFormatCurrency` (5 cases) + `TestFormatCurrencyNoBrokenVerbs`
- `internal/report/report_test.go` — `TestMarkdownContainsECRCost`, `TestMarkdownOmitsECRWhenZero`

### Key Type References
- `types.NATGateway`: `ID`, `VPCID`, `SubnetID`, `State`, `AvailabilityMode`, `NetworkInterfaceID`, `Tags`
- `types.RouteTable`: `ID`, `VPCID`, `Routes []Route`, `Subnets`, `Main`, `Tags` (NO `Name` field)
- `types.Route`: `DestinationCIDR`, `Target`, `TargetType` (NO `TargetID` field)
- `analysis.CostEstimate`: has `OtherPercentage()` method
- `analysis.TrafficStats`: has `ECRPercentage()`, `S3Percentage()`, `DynamoPercentage()`, `OtherPercentage()`, `TopSourceIPs(n)` methods
- `analysis.EndpointAnalysis`: has `HasIssues()`, `HasInterfaceEndpoints()`, `GetCreateEndpointCommands()`, `GetAddRouteCommands()`, `GetInterfaceEndpointCosts()`, `GetTotalInterfaceEndpointMonthlyCost()` methods

### Style Variables
Defined in `ui/quick_scan.go`:
```go
stepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D7D7D"))
successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
```
Defined in `ui/deep_scan.go`:
```go
warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Bold(true)
highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
tipStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true)
```

### Critical Lipgloss Rule
**NEVER** put `\n` inside `Style.Render()` calls. Lipgloss treats multi-line content as a block and adds padding/alignment to each line. Always do: `style.Render("text") + "\n"`

## Resources

- **AWS IP Ranges**: https://ip-ranges.amazonaws.com/ip-ranges.json
- **Flow Logs Format**: https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs.html
- **VPC Endpoints**: https://docs.aws.amazon.com/vpc/latest/privatelink/vpc-endpoints.html
- **Bubble Tea**: https://github.com/charmbracelet/bubbletea

## Getting Help

When asking for help or reporting issues:

1. **Include context**:
   - Go version: `go version`
   - AWS region: `echo $AWS_REGION`
   - Command run: exact command with flags

2. **Include logs**:
   - Error messages (full stack trace)
   - CloudWatch Logs query results
   - Flow Logs status

3. **Include verification**:
   - Did traffic run during scan?
   - Did Flow Logs create successfully?
   - What does `continuous-traffic.sh status` show?

## Quick Reference

### Build & Test
```bash
go build -o terminat .                    # Build
go test ./...                               # Run all tests
./test/scripts/run-e2e-test.sh             # E2E test (fully automated)
```

### Scan Commands
```bash
# Quick scan (no Flow Logs)
./terminat scan quick --region us-east-1

# Deep scan (interactive)
./terminat scan deep --region us-east-1 --duration 5

# Deep scan (automated)
./terminat scan deep --region us-east-1 --duration 5 --auto-approve --auto-cleanup

# Demo mode (no AWS needed)
./terminat scan demo

# Filter by VPC
./terminat scan deep --region us-east-1 --vpc-id vpc-xxx

# Scan specific NATs
./terminat scan deep --region us-east-1 --nat-gateway-ids nat-xxx,nat-yyy

# Export report
./terminat scan deep --region us-east-1 --export markdown --output reports/terminat-report-$(date +%Y%m%d-%H%M%S).md

# With DataHub integration
./terminat scan deep --region us-east-1 --doit-datahub-api-key KEY --doit-customer-context CTX
```

### Release
```bash
./scripts/release.sh v0.7.0               # Build all platforms + upload to GitHub
```

### AWS Commands
```bash
# Check Flow Logs
aws ec2 describe-flow-logs --region us-east-1

# Check CloudWatch Log Groups
aws logs describe-log-groups --log-group-name-prefix "/aws/vpc/flowlogs/terminat"

# Check NAT Gateways
aws ec2 describe-nat-gateways --region us-east-1

# Check VPC Endpoints
aws ec2 describe-vpc-endpoints --region us-east-1

# Check ECR repositories
aws ecr describe-repositories --region us-east-1
```

### Cleanup
```bash
./test/scripts/cleanup.sh                   # Clean test resources (includes ECR)
aws ec2 delete-flow-logs --flow-log-ids fl-xxx  # Manual Flow Logs cleanup
aws logs delete-log-group --log-group-name xxx  # Manual log group cleanup
aws ecr batch-delete-image --repository-name xxx --image-ids imageDigest=xxx  # Manual ECR cleanup
```

## Contributing Checklist

Before submitting PR:

- [ ] Code follows Go conventions (`gofmt`)
- [ ] Added tests for new functionality
- [ ] Updated relevant documentation
- [ ] Ran E2E test successfully
- [ ] No hardcoded credentials or account IDs
- [ ] Error handling for AWS API calls
- [ ] Context cancellation support
- [ ] Cleanup resources on error/interrupt

## Agent-Specific Notes

### When Helping Users

1. **Check prerequisites first**: AWS credentials, Go version, IAM permissions
2. **Verify environment**: `AWS_PROFILE` and `AWS_REGION` set
3. **For traffic issues**: Always check if traffic ran DURING scan
4. **For cost questions**: Explain it's estimates based on samples
5. **For cleanup**: Verify Flow Logs stopped, offer manual cleanup commands
6. **For ECR issues**: Check Docker is installed and running, ECR permissions granted

### When Debugging

1. **Start with E2E test**: Validates entire workflow including ECR
2. **Check CloudWatch Logs**: Raw data shows what Flow Logs captured
3. **Verify AWS IP ranges**: Download and inspect manually
4. **Test classifier**: Unit test with known IPs (including EC2 ranges for ECR)
5. **Check cache**: `~/.terminat/cache/` for IP ranges cache

### When Contributing

1. **Read existing code**: Understand patterns before adding
2. **Keep it simple**: Minimal code for the requirement
3. **Test thoroughly**: Unit + integration + E2E
4. **Document clearly**: Comments for exported functions
5. **Handle errors**: AWS APIs can fail, handle gracefully

### Common Debugging Scenarios

**ECR Traffic Not Detected**:
- Verify Docker is running on EC2 instance
- Check ECR repository exists and has images
- Confirm traffic generation script is pushing images
- Verify EC2 IP ranges loaded in classifier

**Cache Issues**:
- Check `~/.terminat/cache/` directory exists and is writable
- Verify timestamp file format (RFC3339)
- Delete cache to force refresh: `rm -rf ~/.terminat/cache/`

**E2E Test Failures**:
- Ensure `STACK_NAME` environment variable is set
- Check all resources created (VPC, NAT, EC2, S3, DynamoDB, ECR)
- Verify traffic script is running: `./test/scripts/continuous-traffic.sh status`
- Check for ECR repository cleanup issues
