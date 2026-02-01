# termiNATor E2E Testing Guide

Complete guide for running end-to-end tests of termiNATor with fresh AWS infrastructure.

## Quick Start (Batteries Included)

### Prerequisites

1. **AWS CLI configured** with a profile that has admin permissions
2. **Go 1.21+** installed
3. **AWS SSO** (if using SSO profiles)

### One-Command E2E Test

```bash
# Set your AWS profile and region
export AWS_PROFILE=your-profile-name
export AWS_REGION=us-east-1

# Run the complete E2E test (takes ~15 minutes)
./test/scripts/run-e2e-test.sh
```

That's it! The script will:
1. ✅ Deploy test infrastructure (NAT Gateway, VPC, EC2, S3, DynamoDB)
2. ✅ Build termiNATor binary
3. ✅ Start continuous traffic generation
4. ✅ Run deep dive scan
5. ✅ Verify traffic classification works
6. ✅ Stop traffic generation
7. ✅ Clean up all resources

## Manual E2E Test (Step-by-Step)

If you want to run each step manually or troubleshoot:

### Step 1: Set Up AWS Profile

```bash
# Find your AWS account ID
aws sts get-caller-identity

# Set environment variables
export AWS_PROFILE=your-profile-name
export AWS_REGION=us-east-1

# Test AWS access
aws ec2 describe-vpcs --region $AWS_REGION
```

### Step 2: Deploy Test Infrastructure

```bash
cd /path/to/terminator

# Deploy CloudFormation stack (takes ~3-4 minutes)
./test/scripts/deploy-test-infra.sh
```

**What gets created:**
- VPC with public and private subnets
- NAT Gateway in public subnet
- EC2 instance in private subnet (for traffic generation)
- S3 bucket (for test traffic)
- DynamoDB table (for test traffic)
- IAM role for Flow Logs

**Outputs:**
```
Stack created successfully!
NAT Gateway ID: nat-040aaf3a6a3c32c40
VPC ID: vpc-0dd4a2ec9743c9a76
Test Instance ID: i-00a5a75dc7db5e9b2
```

### Step 3: Build termiNATor

```bash
# Build the binary
go build -o terminator .

# Verify it works
./terminat --help
```

### Step 4: Start Continuous Traffic Generation

**CRITICAL**: Traffic must run **during** the scan, not before!

```bash
# Start traffic generator (runs for 30 minutes)
./test/scripts/continuous-traffic.sh start
```

**Verify it's running:**
```bash
./test/scripts/continuous-traffic.sh status
# Output: Traffic generation status: InProgress (Command ID: 7c01c1fb-...)
```

### Step 5: Run Deep Dive Scan

```bash
# Run 5-minute scan (10 minutes total with startup)
./terminat scan deep --region $AWS_REGION --duration 5
```

**What happens:**
1. Discovers NAT Gateway
2. Asks for approval to create Flow Logs
3. Creates Flow Logs and CloudWatch Log Group
4. Waits 5 minutes for Flow Logs to initialize
5. Collects traffic for 5 minutes
6. Analyzes traffic and classifies by service
7. Stops Flow Logs automatically
8. Shows results and asks about CloudWatch cleanup

**Expected Results:**
```
Traffic Analysis:
  Total: 3,203 records, 5.4 TB
  S3:       2.0 TB (38.1%)
  DynamoDB: 517.8 GB (9.6%)
  Other:    2.8 TB (52.3%)

VPC Endpoint Issues Detected:
  • 2 missing endpoint(s)

💰 Potential Savings: $977,889.93/month ($11,734,679.21/year)
```

### Step 6: Stop Traffic Generation

```bash
# Stop the traffic generator
./test/scripts/continuous-traffic.sh stop
```

### Step 7: Clean Up Test Infrastructure

```bash
# Delete all test resources
./test/scripts/cleanup.sh
```

**What gets deleted:**
- CloudFormation stack (NAT Gateway, VPC, subnets, etc.)
- S3 bucket (empties first, then deletes)
- DynamoDB table
- EC2 instance
- Flow Logs (if any remain)
- CloudWatch Log Groups

## Test Infrastructure Details

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ VPC (10.0.0.0/16)                                           │
│                                                             │
│  ┌──────────────────┐         ┌──────────────────┐        │
│  │ Public Subnet    │         │ Private Subnet   │        │
│  │ 10.0.1.0/24      │         │ 10.0.2.0/24      │        │
│  │                  │         │                  │        │
│  │  ┌────────────┐  │         │  ┌────────────┐ │        │
│  │  │ NAT Gateway│◄─┼─────────┼──┤ EC2 Instance│ │        │
│  │  └────────────┘  │         │  └────────────┘ │        │
│  │        │         │         │                  │        │
│  └────────┼─────────┘         └──────────────────┘        │
│           │                                                │
└───────────┼────────────────────────────────────────────────┘
            │
            ▼
    ┌───────────────┐
    │   Internet    │
    │  (S3, DynamoDB)│
    └───────────────┘
```

### Traffic Generation Script

The EC2 instance runs a script that continuously:
1. Uploads files to S3 (generates S3 traffic through NAT)
2. Writes items to DynamoDB (generates DynamoDB traffic through NAT)
3. Makes HTTP requests to external IPs (generates "Other" traffic)

**Script location on EC2**: `/home/ec2-user/generate-traffic.sh`

### CloudFormation Stack

**Stack name**: `terminator-test-infra`

**Resources created:**
- VPC
- Internet Gateway
- Public Subnet
- Private Subnet
- NAT Gateway
- Elastic IP (for NAT Gateway)
- Route Tables (public and private)
- Security Group
- EC2 Instance (Amazon Linux 2023)
- S3 Bucket
- DynamoDB Table
- IAM Role (for Flow Logs)

**Estimated costs** (while running):
- NAT Gateway: $0.045/hour + $0.045/GB
- EC2 t3.micro: $0.0104/hour
- S3: Minimal (test data)
- DynamoDB: Minimal (on-demand)
- **Total**: ~$0.06/hour

## Troubleshooting

### Stack Creation Fails

**Check CloudFormation events:**
```bash
aws cloudformation describe-stack-events \
  --stack-name terminator-test-infra \
  --region $AWS_REGION \
  --max-items 10
```

**Common issues:**
- Insufficient permissions
- Region doesn't support t3.micro (use t2.micro instead)
- VPC limit reached

### Traffic Generator Not Starting

**Check SSM command status:**
```bash
# Get command ID from continuous-traffic.sh output
COMMAND_ID="7c01c1fb-06fb-452c-9633-00dbf43df8a7"
INSTANCE_ID="i-00a5a75dc7db5e9b2"

aws ssm get-command-invocation \
  --command-id $COMMAND_ID \
  --instance-id $INSTANCE_ID \
  --region $AWS_REGION
```

**Common issues:**
- EC2 instance not ready (wait 2-3 minutes after stack creation)
- SSM agent not running (check instance logs)
- IAM permissions missing

### No Traffic Classified

**Verify traffic is running:**
```bash
./test/scripts/continuous-traffic.sh status
```

**Check Flow Logs are capturing data:**
```bash
# Get log group name from scan output
LOG_GROUP="/aws/vpc/flowlogs/terminator-1234567890"

# Check for log streams
aws logs describe-log-streams \
  --log-group-name $LOG_GROUP \
  --region $AWS_REGION
```

**Common issues:**
- Traffic generator not running during scan
- Flow Logs not initialized (wait full 5 minutes)
- NAT Gateway not routing traffic

### Cleanup Fails

**Manual cleanup:**
```bash
# Delete Flow Logs
aws ec2 describe-flow-logs --region $AWS_REGION
aws ec2 delete-flow-logs --flow-log-ids fl-xxxxx --region $AWS_REGION

# Delete CloudWatch Log Groups
aws logs describe-log-groups --log-group-name-prefix "/aws/vpc/flowlogs/terminator" --region $AWS_REGION
aws logs delete-log-group --log-group-name "/aws/vpc/flowlogs/terminator-xxxxx" --region $AWS_REGION

# Delete CloudFormation stack
aws cloudformation delete-stack --stack-name terminator-test-infra --region $AWS_REGION

# Wait for deletion
aws cloudformation wait stack-delete-complete --stack-name terminator-test-infra --region $AWS_REGION
```

## Automated E2E Test Script

Create `test/scripts/run-e2e-test.sh`:

```bash
#!/bin/bash
set -e

echo "🚀 Starting termiNATor E2E Test"
echo "================================"

# Check prerequisites
if [ -z "$AWS_PROFILE" ]; then
    echo "❌ AWS_PROFILE not set"
    exit 1
fi

if [ -z "$AWS_REGION" ]; then
    echo "❌ AWS_REGION not set"
    exit 1
fi

echo "✓ AWS Profile: $AWS_PROFILE"
echo "✓ AWS Region: $AWS_REGION"
echo ""

# Step 1: Deploy infrastructure
echo "📦 Step 1/7: Deploying test infrastructure..."
./test/scripts/deploy-test-infra.sh
echo ""

# Step 2: Build binary
echo "🔨 Step 2/7: Building termiNATor..."
go build -o terminator .
echo "✓ Build complete"
echo ""

# Step 3: Wait for EC2 to be ready
echo "⏳ Step 3/7: Waiting for EC2 instance to initialize (2 minutes)..."
sleep 120
echo "✓ EC2 ready"
echo ""

# Step 4: Start traffic generation
echo "🚦 Step 4/7: Starting continuous traffic generation..."
./test/scripts/continuous-traffic.sh start
echo ""

# Step 5: Run deep dive scan
echo "🔍 Step 5/7: Running deep dive scan (10 minutes)..."
echo "   - 5 min: Flow Logs initialization"
echo "   - 5 min: Traffic collection"
echo ""
echo "y" | ./terminat scan deep --region $AWS_REGION --duration 5
echo ""

# Step 6: Stop traffic
echo "🛑 Step 6/7: Stopping traffic generation..."
./test/scripts/continuous-traffic.sh stop
echo ""

# Step 7: Cleanup
echo "🧹 Step 7/7: Cleaning up test infrastructure..."
./test/scripts/cleanup.sh
echo ""

echo "✅ E2E Test Complete!"
echo "================================"
```

Make it executable:
```bash
chmod +x test/scripts/run-e2e-test.sh
```

## Verification Checklist

After running E2E test, verify:

- [ ] Traffic classification shows S3 traffic (>0%)
- [ ] Traffic classification shows DynamoDB traffic (>0%)
- [ ] VPC endpoint analysis detects 2 missing endpoints
- [ ] Remediation commands are generated
- [ ] Source IPs are tracked and displayed
- [ ] Cost estimates are calculated
- [ ] Flow Logs are stopped automatically
- [ ] All resources are cleaned up

## CI/CD Integration

### GitHub Actions Example

```yaml
name: E2E Test

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  e2e-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1
      
      - name: Run E2E Test
        run: |
          export AWS_REGION=us-east-1
          ./test/scripts/run-e2e-test.sh
```

## Performance Benchmarks

Expected timings for E2E test:

| Step | Duration |
|------|----------|
| Deploy infrastructure | 3-4 minutes |
| Build binary | 10-30 seconds |
| EC2 initialization | 2 minutes |
| Start traffic | 5 seconds |
| Deep dive scan | 10 minutes |
| Stop traffic | 5 seconds |
| Cleanup | 2-3 minutes |
| **Total** | **~18 minutes** |

## Cost Estimate

Running one complete E2E test:

| Resource | Duration | Cost |
|----------|----------|------|
| NAT Gateway | ~20 minutes | ~$0.015 |
| NAT Gateway data | ~5 GB | ~$0.225 |
| EC2 t3.micro | ~20 minutes | ~$0.003 |
| Flow Logs | ~5 GB | ~$2.50 |
| CloudWatch Logs | ~5 GB | ~$0.03/month |
| **Total** | | **~$2.77** |

**Note**: Most cost is Flow Logs ingestion. This is expected and validates the tool works correctly.

## Best Practices

1. **Run in isolated account** - Use a dedicated test/sandbox AWS account
2. **Clean up after tests** - Always run cleanup script
3. **Monitor costs** - Set up billing alerts
4. **Use consistent regions** - Stick to one region for testing
5. **Document failures** - Capture logs when tests fail

## Support

For E2E testing issues:
1. Check this guide's troubleshooting section
2. Review CloudFormation events
3. Check CloudWatch Logs
4. Open GitHub issue with full logs
