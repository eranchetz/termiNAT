# termiNATor

**Terminate unnecessary NAT Gateway costs by detecting services that should use VPC endpoints.**

termiNATor is a CLI tool that analyzes your AWS NAT Gateway traffic to identify cost optimization opportunities. It detects when your applications are routing traffic to AWS services (like S3 and DynamoDB) through NAT Gateways instead of using free VPC Gateway Endpoints, helping you eliminate unnecessary data processing charges.

## Features

- **Quick Scan**: Instant analysis of your VPC configuration to detect missing VPC endpoints
- **Deep Dive Scan**: Real-time traffic analysis using VPC Flow Logs to identify actual service usage patterns
- **Cost Estimation**: Calculate potential monthly and annual savings with clear disclaimers
- **Traffic Classification**: Automatically identifies S3, DynamoDB, and other AWS service traffic
- **Detailed Reporting**: Comprehensive analysis with actionable recommendations
- **Multi-Region Support**: Works across all AWS regions with regional pricing

## Why termiNATor?

NAT Gateways charge $0.045 per GB for data processing. If your applications access S3 or DynamoDB through a NAT Gateway, you're paying for traffic that could be **completely free** using Gateway VPC Endpoints.

**Example Savings:**
- 100 GB/month to S3 through NAT Gateway: **$4.50/month** → **$0/month** with VPC Endpoint
- 1 TB/month to DynamoDB through NAT Gateway: **$45/month** → **$0/month** with VPC Endpoint

## Quick Start

```bash
# Install
git clone https://github.com/doitintl/terminator.git
cd terminator
go build -o terminat

# Configure AWS credentials
export AWS_PROFILE=your-profile
export AWS_REGION=us-east-1

# Run quick scan (instant, no resources created)
./terminat scan quick --region us-east-1

# Run deep dive scan (analyzes actual traffic, ~10 minutes)
./terminat scan deep --region us-east-1 --duration 5

# Run demo scan with fake data (stream output by default)
./terminat scan demo

# Optional: run interactive full-screen TUI instead of serial stream output
./terminat scan deep --region us-east-1 --duration 5 --ui tui
./terminat scan demo --ui tui
```

📖 **[Complete Usage Guide](USAGE.md)** - Detailed instructions for production use  
🧪 **[E2E Testing Guide](E2E_TESTING.md)** - Run automated tests with sample infrastructure

## Installation

```bash
go install github.com/doitintl/terminator@latest
```

Or build from source:

```bash
git clone https://github.com/doitintl/terminator.git
cd terminator
go build -o terminat
```

## Prerequisites

### AWS Credentials

Configure AWS credentials using one of these methods:

```bash
# AWS CLI configuration
aws configure

# Environment variables
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"

# AWS Profile
export AWS_PROFILE="your-profile"
```

### IAM Permissions

For **Quick Scan**, you need read-only permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeNatGateways",
        "ec2:DescribeVpcEndpoints",
        "ec2:DescribeRouteTables",
        "ec2:DescribeSubnets",
        "ec2:DescribeVpcs"
      ],
      "Resource": "*"
    }
  ]
}
```

For **Deep Dive Scan**, additional permissions are required:

```bash
# Run the setup script to create the IAM role
./scripts/setup-flowlogs-role.sh
```

This creates a role with permissions for:
- Creating and deleting VPC Flow Logs
- Creating and querying CloudWatch Logs
- Getting caller identity (for account ID)

## Quick Start

### Quick Scan (No Flow Logs)

Instantly analyze your VPC configuration:

```bash
terminat scan quick --region us-east-1
```

This will:
- Discover all NAT Gateways in the region
- Check for existing VPC endpoints
- Identify missing S3 and DynamoDB endpoints
- Provide immediate recommendations

### Deep Dive Scan (With Flow Logs)

Analyze actual traffic patterns:

```bash
terminat scan deep --region us-east-1 --duration 5
```

By default, scans run in serial stream mode (`--ui stream`) so output is append-only and CI/log friendly.
Use `--ui tui` for the interactive Bubble Tea interface.

This will:
1. Create temporary VPC Flow Logs for your NAT Gateway
2. Wait 5 minutes for Flow Logs to initialize
3. Collect traffic data for 5 minutes (configurable: 5-60 minutes)
4. Classify traffic by destination service (S3, DynamoDB, other)
5. Calculate cost estimates and potential savings
6. Clean up Flow Logs (log data retained for review)

**Total time:** Collection duration + 5 minutes (startup delay)

**Example output:**
```
NAT Gateway Topology:
  nat-1234567890abcdef0 (zonal, vpc-abcd1234)

VPC Endpoint Configuration:
  Gateway Endpoints:
    ✗ S3: NOT CONFIGURED
    ✗ DynamoDB: NOT CONFIGURED
  ECR Interface Endpoints (Paid):
    ⚠ ECR API (ecr.api): MISSING
    ⚠ ECR DKR (ecr.dkr): MISSING

Traffic Analysis:
  Total: 1,234 records, 45.67 GB
  S3: 890 records, 32.10 GB (70.3%)
  DynamoDB: 234 records, 8.45 GB (18.5%)
  ECR: 100 records, 2.45 GB (5.4%)
  Other: 110 records, 5.12 GB (11.2%)

Cost Savings Estimate:
  Current Monthly NAT Gateway Cost: $61.45
  Potential Savings with VPC Endpoints: $54.74/month ($656.88/year)
  
⚠️  IMPORTANT: This is an ESTIMATE based on the traffic sample collected.
```

## Commands

### Scan Commands

```bash
# Quick scan
terminat scan quick --region <region>

# Deep dive scan
terminat scan deep --region <region> --duration <minutes>

# Demo scan (fake data, no AWS credentials needed)
terminat scan demo

# Export markdown report to persistent reports/ folder
terminat scan deep --region us-east-1 --duration 5 --export markdown --output reports/terminat-report-$(date +%Y%m%d-%H%M%S).md

# Skip doctor preflight (enabled by default)
terminat scan quick --region <region> --doctor=false

# Optional TUI mode
terminat scan quick --region <region> --ui tui
terminat scan deep --region <region> --duration <minutes> --ui tui
terminat scan demo --ui tui

# Scan specific NAT Gateway
terminat scan deep --region us-east-1 --nat-id nat-1234567890abcdef0
```

### UI Modes

- Default mode is serial stream output (`--ui stream`) for `scan quick`, `scan deep`, and `scan demo`.
- Use `--ui tui` only when you want the interactive full-screen Bubble Tea experience.

### Doctor Preflight

- `scan quick` and `scan deep` run doctor preflight checks by default.
- Disable only this step with `--doctor=false` when needed.

### Fast Validation

Run the smoke test to verify stream-mode CLI wiring without creating AWS resources:

```bash
./test/scripts/smoke-ui-stream.sh
```

### Cleanup Commands

After a Deep Dive scan, Flow Logs data is retained for your review. Clean it up when done:

```bash
# List log groups
aws logs describe-log-groups --log-group-name-prefix "/aws/vpc/flowlogs/terminat"

# Delete log group
terminat cleanup --region us-east-1 --log-group "/aws/vpc/flowlogs/terminat-1234567890"
```

## Understanding the Results

### Traffic Classification

- **S3 Traffic**: Requests to Amazon S3 (object storage)
- **DynamoDB Traffic**: Requests to Amazon DynamoDB (NoSQL database)
- **Other Traffic**: All other destinations (EC2, RDS, internet, etc.)

### Cost Calculations

**NAT Gateway Pricing:**
- Data processing: $0.045 per GB (most regions)
- Hourly charge: $0.045 per hour (not included in estimates)

**VPC Gateway Endpoints:**
- S3 Gateway Endpoint: **FREE** (no hourly or data charges)
- DynamoDB Gateway Endpoint: **FREE** (no hourly or data charges)

**ECR Interface Endpoints (paid):**
- Estimated using the scanner's static per-region PrivateLink pricing table (defaults to $0.01 per AZ-hour and $0.01 per GB for most regions).
- Pricing comes from the `internal/analysis/endpoints.go` table and is treated as an estimate; verify current AWS PrivateLink pricing for your region before provisioning.

**Important Notes:**
- Cost estimates are based on the traffic sample collected during the scan
- Actual costs may vary based on traffic patterns, time of day, and workload changes
- Estimates extrapolate sample data to monthly projections
- Only data processing costs are calculated (hourly NAT Gateway charges not included)

## Architecture

```
terminator/
├── cmd/              # CLI commands (scan, cleanup)
├── internal/
│   ├── core/        # Core business logic (scanner)
│   ├── aws/         # AWS service clients (EC2, CloudWatch)
│   ├── analysis/    # Traffic analysis and cost calculation
│   └── report/      # Report generation (future)
├── pkg/             # Public APIs and types
├── ui/              # Terminal UI components
└── scripts/         # Setup and utility scripts
```

## How It Works

### Quick Scan

1. Discovers NAT Gateways in your VPC
2. Checks route tables for traffic routing through NAT
3. Identifies missing VPC endpoints for S3 and DynamoDB
4. Provides recommendations

### Deep Dive Scan

1. **Discovery**: Finds NAT Gateways and their network interfaces
2. **Flow Logs Creation**: Creates temporary VPC Flow Logs on the NAT Gateway ENI
3. **Startup Delay**: Waits 5 minutes for Flow Logs to begin delivering data
4. **Collection**: Captures network traffic for the specified duration
5. **Analysis**: 
   - Downloads AWS IP ranges for S3 and DynamoDB
   - Classifies each flow by destination IP
   - Calculates data volumes per service
6. **Cost Calculation**: 
   - Applies regional NAT Gateway pricing
   - Extrapolates sample to monthly projections
   - Calculates potential savings with VPC endpoints
7. **Cleanup**: Deletes Flow Logs configuration (retains log data for review)

## Best Practices

1. **Run during peak hours**: Collect traffic samples during typical workload periods for accurate estimates
2. **Longer collection periods**: Use 15-30 minute collection windows for more representative samples
3. **Multiple scans**: Run scans at different times of day to understand traffic patterns
4. **Review log data**: Use CloudWatch Logs Insights to analyze detailed traffic patterns
5. **Test VPC endpoints**: Create endpoints in a test environment first to validate connectivity

## Troubleshooting

### "No NAT gateways found"

- Verify you're scanning the correct region
- Check that NAT Gateways exist in your VPC
- Ensure your IAM permissions include `ec2:DescribeNatGateways`

### "Failed to create Flow Logs"

- Run `./scripts/setup-flowlogs-role.sh` to create the required IAM role
- Verify the role ARN in the error message
- Check CloudWatch Logs permissions

### "No traffic data collected"

- Flow Logs require 5-10 minutes to start delivering data
- Ensure applications are actively using the NAT Gateway during collection
- Check CloudWatch Logs console for Flow Logs data

### "Cost estimates seem incorrect"

- Remember these are ESTIMATES based on traffic samples
- Verify the collection period was representative of typical usage
- Consider running multiple scans at different times
- Check if traffic patterns vary significantly throughout the day

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Apache License 2.0

## Support

For issues and questions:
- GitHub Issues: https://github.com/doitintl/terminator/issues
- Documentation: https://github.com/doitintl/terminator/wiki

## Roadmap

- [x] Support for Interface VPC Endpoints cost analysis
- [ ] Historical cost analysis from CloudWatch metrics
- [ ] Automated VPC endpoint creation
- [ ] Multi-account scanning
- [ ] JSON/CSV export for reporting
- [ ] Integration with AWS Cost Explorer

---

**Made with ❤️ by DoiT International**
