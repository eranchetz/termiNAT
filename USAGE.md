# termiNATor Usage Guide

Complete guide for running termiNATor in your AWS environment to identify NAT Gateway cost optimization opportunities.

## Prerequisites

### 1. Install termiNATor

```bash
# Option 1: Install from source
git clone https://github.com/doitintl/terminator.git
cd terminator
go build -o terminator

# Option 2: Download binary (when available)
# curl -L https://github.com/doitintl/terminator/releases/latest/download/terminator-$(uname -s)-$(uname -m) -o terminator
# chmod +x terminator
```

### 2. Configure AWS Credentials

```bash
# Option A: AWS CLI (recommended)
aws configure

# Option B: Environment variables
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"

# Option C: AWS Profile
export AWS_PROFILE="your-profile"
```

### 3. Set Up IAM Permissions

**For Quick Scan (read-only):**
```bash
# No setup needed - just needs EC2 read permissions
```

**For Deep Dive Scan (creates temporary resources):**
```bash
# Run the setup script to create required IAM role
./scripts/setup-flowlogs-role.sh
```

This creates a role with permissions for:
- Creating/deleting VPC Flow Logs
- Creating/querying CloudWatch Logs
- Reading VPC configuration

## Quick Start

### Option 1: Quick Scan (Instant, No Resources Created)

Analyze your VPC configuration without creating any resources:

```bash
./terminat scan quick --region us-east-1
```

**What it does:**
- ✅ Discovers NAT Gateways
- ✅ Checks for missing VPC endpoints
- ✅ Provides immediate recommendations
- ✅ No AWS resources created
- ✅ No costs incurred

**Output:**
```
Found 1 NAT Gateway in us-east-1
  • nat-1234567890abcdef0 (vpc-abcd1234)

Missing VPC Endpoints:
  ⚠️  S3 Gateway Endpoint - FREE, eliminates NAT charges for S3 traffic
  ⚠️  DynamoDB Gateway Endpoint - FREE, eliminates NAT charges for DynamoDB traffic

Recommendations:
  Create S3 Gateway Endpoint: aws ec2 create-vpc-endpoint --vpc-id vpc-abcd1234 ...
  Create DynamoDB Gateway Endpoint: aws ec2 create-vpc-endpoint --vpc-id vpc-abcd1234 ...
```

### Option 2: Deep Dive Scan (Analyzes Actual Traffic)

Analyze real traffic patterns to calculate actual savings:

```bash
./terminat scan deep --region us-east-1 --duration 5
```

By default, `scan quick` and `scan deep` use serial stream output (`--ui stream`) so logs stay append-only.
Use `--ui tui` for the interactive full-screen Bubble Tea interface.

**What it does:**
- ✅ Creates temporary VPC Flow Logs (you approve first)
- ✅ Collects 5 minutes of traffic data
- ✅ Classifies traffic by service (S3, DynamoDB, Other)
- ✅ Calculates cost estimates and savings
- ✅ Auto-cleans up Flow Logs after scan
- ✅ Asks if you want to keep CloudWatch logs

**Timeline:**
- 5 min: Flow Logs initialization (AWS requirement)
- 5 min: Traffic collection (configurable: 5-60 minutes)
- **Total: 10 minutes**

**Approval prompt:**
```
⚠️  RESOURCE CREATION APPROVAL REQUIRED

The following AWS resources will be created:

1. VPC Flow Logs (temporary)
   • NAT Gateway: nat-1234567890abcdef0 (VPC: vpc-abcd1234)
   → Flow Logs will be AUTOMATICALLY STOPPED after analysis

2. CloudWatch Log Group
   • /aws/vpc/flowlogs/terminator-1234567890
   → You'll be asked whether to keep or delete after scan

📊 Estimated Costs:
   • Flow Logs ingestion: ~$0.50 per GB
   • CloudWatch Logs storage: ~$0.03 per GB/month
   • For a 5-minute scan, typical cost: < $0.10

⏱️  Total scan time: 10 minutes
   • 5 min startup delay (Flow Logs initialization)
   • 5 min traffic collection

Proceed with scan? [Y/n]
```

**Final report:**
```
═══════════════════════════════════════════════════════════════
                  VPC ENDPOINT CONFIGURATION
═══════════════════════════════════════════════════════════════

VPC: vpc-0dd4a2ec9743c9a76

Gateway Endpoints:
  ✗ S3: NOT CONFIGURED
  ✗ DynamoDB: NOT CONFIGURED

═══════════════════════════════════════════════════════════════
                      TRAFFIC ANALYSIS
═══════════════════════════════════════════════════════════════

Total Traffic: 3,203 records, 5.4 TB

Traffic by Service:
  Service      Data          Percentage
  ─────────── ─────────     ──────────
  S3          2.0 TB        38.1%
  DynamoDB    517.8 GB      9.6%
  Other       2.8 TB        52.3%

Top Source IPs:
  • 10.0.2.189: 1.3 TB (787 records)
  • 10.0.1.97: 1.3 TB (781 records)
  ... and 590 more sources

═══════════════════════════════════════════════════════════════
                      COST ESTIMATE
═══════════════════════════════════════════════════════════════

NAT Gateway Data Processing: $0.0450 per GB

Projected Monthly Costs:
  Current NAT Gateway cost:     $2,051,199.38/month
  Potential S3 savings:         $781,287.31/month
  Potential DynamoDB savings:   $196,602.63/month
  ─────────────────────────────────────────
  TOTAL POTENTIAL SAVINGS:      $977,889.93/month ($11,734,679.21/year)

═══════════════════════════════════════════════════════════════
                    REMEDIATION STEPS
═══════════════════════════════════════════════════════════════

📦 Create Missing VPC Endpoints:

aws ec2 create-vpc-endpoint \
  --vpc-id vpc-0dd4a2ec9743c9a76 \
  --service-name com.amazonaws.us-east-1.s3 \
  --route-table-ids rtb-0b83dfd7b61cda66e

aws ec2 create-vpc-endpoint \
  --vpc-id vpc-0dd4a2ec9743c9a76 \
  --service-name com.amazonaws.us-east-1.dynamodb \
  --route-table-ids rtb-0b83dfd7b61cda66e

⚠️  DISCLAIMERS:
   • Cost estimates based on traffic sample collected
   • Actual costs may vary based on traffic patterns
   • Gateway VPC Endpoints for S3 and DynamoDB are FREE
```

## Advanced Usage

### Scan Specific NAT Gateway

```bash
./terminat scan deep --region us-east-1 --nat-id nat-1234567890abcdef0 --duration 5
```

### Longer Collection Period (More Accurate)

```bash
# 30-minute collection for better traffic sampling
./terminat scan deep --region us-east-1 --duration 30
```

### Interactive TUI Mode

```bash
./terminat scan quick --region us-east-1 --ui tui
./terminat scan deep --region us-east-1 --duration 15 --ui tui
```

### Multiple Regions

```bash
# Scan each region separately
./terminat scan quick --region us-east-1
./terminat scan quick --region us-west-2
./terminat scan quick --region eu-west-1
```

## Best Practices

### 1. When to Run Scans

- **Quick Scan**: Run anytime, instant results
- **Deep Dive Scan**: Run during typical workload hours for representative data

### 2. Collection Duration

- **5 minutes**: Quick check, good for initial assessment
- **15-30 minutes**: Recommended for accurate cost estimates
- **60 minutes**: Most accurate, captures traffic variations

### 3. Multiple Scans

Run scans at different times to understand traffic patterns:
```bash
# Morning traffic
./terminat scan deep --region us-east-1 --duration 15

# Afternoon traffic
./terminat scan deep --region us-east-1 --duration 15

# Evening traffic
./terminat scan deep --region us-east-1 --duration 15
```

### 4. Cleanup

After Deep Dive scan, you'll be asked about CloudWatch logs:

```
CloudWatch Log Group Cleanup

Log Group: /aws/vpc/flowlogs/terminator-1234567890

This log group contains the collected traffic data.
• Keep it to analyze traffic patterns in CloudWatch Logs Insights
• Delete it to avoid storage costs (~$0.03/GB/month)

Delete CloudWatch Log Group? [Y/n]
```

**Recommendation**: Delete unless you need detailed traffic analysis.

### Option 3: Demo Scan (No AWS Needed)

Preview report output using realistic fake data:

```bash
./terminat scan demo
```

Use full-screen TUI only when explicitly requested:

```bash
./terminat scan demo --ui tui
```

## Troubleshooting

### "No NAT gateways found"

**Solution:**
```bash
# Verify NAT Gateways exist
aws ec2 describe-nat-gateways --region us-east-1

# Check you're scanning the correct region
./terminat scan quick --region <your-region>
```

### "Failed to create Flow Logs"

**Solution:**
```bash
# Run the IAM setup script
./scripts/setup-flowlogs-role.sh

# Verify the role was created
aws iam get-role --role-name termiNATor-FlowLogsRole
```

### "No traffic data collected"

**Causes:**
- Flow Logs need 5-10 minutes to start delivering data (handled automatically)
- No applications using NAT Gateway during collection

**Solution:**
```bash
# Ensure applications are actively using NAT Gateway
# Run a longer collection period
./terminat scan deep --region us-east-1 --duration 30
```

### "Cost estimates seem incorrect"

**Remember:**
- Estimates are based on traffic samples
- Traffic patterns vary throughout the day
- Run multiple scans at different times for better accuracy

## Understanding the Results

### Traffic Classification

- **S3 Traffic**: Requests to Amazon S3 object storage
- **DynamoDB Traffic**: Requests to Amazon DynamoDB NoSQL database
- **Other Traffic**: All other destinations (EC2, RDS, internet, etc.)

### Cost Calculations

**NAT Gateway Pricing:**
- Data processing: $0.045 per GB (most regions)
- Hourly charge: $0.045 per hour (not included in estimates)

**VPC Gateway Endpoints:**
- S3 Gateway Endpoint: **FREE** (no charges)
- DynamoDB Gateway Endpoint: **FREE** (no charges)

**Savings = NAT Gateway data processing costs for S3/DynamoDB traffic**

### Source IPs

The report shows which instances/IPs send the most traffic through NAT Gateway:
- Helps identify which workloads benefit most from VPC endpoints
- Useful for prioritizing endpoint creation

## Next Steps

After running termiNATor:

1. **Review the recommendations** - Understand which endpoints are missing
2. **Create VPC endpoints** - Use the provided AWS CLI commands
3. **Verify connectivity** - Test that applications can reach S3/DynamoDB through endpoints
4. **Monitor savings** - Check NAT Gateway data processing metrics in CloudWatch

## Support

- **Issues**: https://github.com/doitintl/terminator/issues
- **Documentation**: https://github.com/doitintl/terminator/wiki
- **Questions**: Open a GitHub discussion

## Cost Transparency

### Quick Scan
- **AWS Costs**: $0 (read-only operations)
- **Time**: < 1 second

### Deep Dive Scan (5-minute collection)
- **Flow Logs ingestion**: ~$0.50 per GB ingested
- **CloudWatch Logs storage**: ~$0.03 per GB/month (if kept)
- **Typical cost**: < $0.10 for a 5-minute scan
- **Time**: 10 minutes (5 min startup + 5 min collection)

### Deep Dive Scan (30-minute collection)
- **Typical cost**: < $0.50
- **Time**: 35 minutes (5 min startup + 30 min collection)

**Note**: Actual costs depend on traffic volume through NAT Gateway.
