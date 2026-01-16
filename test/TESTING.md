# termiNATor Testing Guide

## Overview

This guide provides a complete, repeatable testing workflow for termiNATor using the `chetz-playground` AWS account. The test infrastructure is designed to be:

- **Easy to set up**: Single CloudFormation stack deployment
- **Easy to run**: Simple scripts to generate test traffic
- **Easy to validate**: Clear before/after metrics
- **Easy to clean up**: Single stack deletion removes everything

---

## Test Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Test VPC (10.0.0.0/16)                    │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Public Subnet (10.0.1.0/24)                                │
│  ├─ Internet Gateway                                         │
│  └─ NAT Gateway (nat-test-xxx)                              │
│                                                               │
│  Private Subnet (10.0.2.0/24)                               │
│  ├─ Route: 0.0.0.0/0 → NAT Gateway                          │
│  ├─ EC2 Test Instance (Amazon Linux 2023)                   │
│  │  └─ Generates S3 and DynamoDB traffic via NAT            │
│  └─ Lambda Test Function (optional)                         │
│     └─ Generates S3 traffic via NAT                         │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Test Scenarios

### Scenario 1: Baseline (NAT Gateway without endpoints)

**Setup:**
- VPC with NAT Gateway
- Private subnet routes all traffic through NAT
- NO S3 or DynamoDB Gateway endpoints
- EC2 instance in private subnet

**Expected behavior:**
- All S3 and DynamoDB traffic goes through NAT Gateway
- termiNATor Quick Scan should report missing endpoints
- termiNATor Deep Dive should detect S3/DynamoDB bytes through NAT
- Cost calculation should show avoidable NAT data processing charges

**Validation:**
- VPC Flow Logs show `pkt-dstaddr` matching S3/DynamoDB IP ranges
- NAT Gateway CloudWatch metrics show BytesOutToDestination increasing
- termiNATor report shows non-zero S3/DynamoDB traffic

---

### Scenario 2: S3 Gateway Endpoint Added

**Setup:**
- Same as Scenario 1
- Add S3 Gateway VPC endpoint
- Associate endpoint with private subnet route table

**Expected behavior:**
- S3 traffic bypasses NAT Gateway (goes directly to S3 via endpoint)
- DynamoDB traffic still goes through NAT
- termiNATor Deep Dive should show zero S3 bytes, non-zero DynamoDB bytes

**Validation:**
- VPC Flow Logs show no S3 traffic through NAT ENI
- NAT Gateway metrics show reduced BytesOutToDestination
- termiNATor report shows S3 traffic = 0, DynamoDB traffic > 0

---

### Scenario 3: Both S3 and DynamoDB Gateway Endpoints

**Setup:**
- Same as Scenario 2
- Add DynamoDB Gateway VPC endpoint
- Associate endpoint with private subnet route table

**Expected behavior:**
- Both S3 and DynamoDB traffic bypass NAT Gateway
- termiNATor Deep Dive should show zero bytes for both services

**Validation:**
- VPC Flow Logs show minimal/no AWS service traffic through NAT
- NAT Gateway metrics show minimal BytesOutToDestination
- termiNATor report shows S3 = 0, DynamoDB = 0

---

### Scenario 4: Regional NAT Gateway (optional, if supported in test region)

**Setup:**
- VPC with Regional NAT Gateway (AvailabilityMode=regional)
- Private subnets in multiple AZs
- NO S3 or DynamoDB endpoints

**Expected behavior:**
- termiNATor correctly identifies regional NAT mode
- Flow Logs created on NAT Gateway resource (not ENI)
- Traffic classification works correctly

**Validation:**
- termiNATor report shows AvailabilityMode=regional
- Flow Logs resource type is RegionalNatGateway
- Detection accuracy matches zonal NAT scenario

---

## Test Infrastructure Setup

### Prerequisites

- AWS CLI configured with `chetz-playground` account access
- Permissions to create VPC, NAT Gateway, EC2, Lambda, CloudFormation
- Region: `us-east-1` (or configurable)

### Step 1: Deploy Test Stack

```bash
# Deploy the test infrastructure
aws cloudformation create-stack \
  --stack-name terminator-test-infra \
  --template-body file://test/infrastructure/test-stack.yaml \
  --capabilities CAPABILITY_IAM \
  --region us-east-1

# Wait for stack creation
aws cloudformation wait stack-create-complete \
  --stack-name terminator-test-infra \
  --region us-east-1

# Get outputs (NAT Gateway ID, EC2 Instance ID, etc.)
aws cloudformation describe-stacks \
  --stack-name terminator-test-infra \
  --region us-east-1 \
  --query 'Stacks[0].Outputs'
```

**Stack creates:**
- VPC with public and private subnets
- Internet Gateway
- NAT Gateway in public subnet
- Route tables (public routes to IGW, private routes to NAT)
- EC2 instance in private subnet with SSM Session Manager access
- IAM role for EC2 with S3 and DynamoDB read permissions
- Security group allowing outbound traffic
- S3 bucket for test data (optional)
- DynamoDB table for test data (optional)

---

### Step 2: Generate Test Traffic

#### Option A: Automated Traffic Generation Script

```bash
# Run traffic generation script on EC2 instance
./test/scripts/generate-traffic.sh \
  --instance-id <EC2_INSTANCE_ID> \
  --duration 5 \
  --s3-requests 100 \
  --dynamodb-requests 50
```

**Script does:**
1. Connects to EC2 via SSM Session Manager
2. Downloads test files from S3 (generates S3 GET traffic)
3. Writes/reads items to DynamoDB (generates DynamoDB traffic)
4. Runs for specified duration (default 5 minutes)
5. Reports total bytes transferred

#### Option B: Manual Traffic Generation

```bash
# Connect to EC2 instance
aws ssm start-session --target <EC2_INSTANCE_ID>

# Generate S3 traffic (download objects repeatedly)
for i in {1..100}; do
  aws s3 cp s3://terminator-test-bucket-<ACCOUNT_ID>/test-file-1mb.bin /tmp/test-$i.bin
  rm /tmp/test-$i.bin
done

# Generate DynamoDB traffic (scan table repeatedly)
for i in {1..50}; do
  aws dynamodb scan --table-name terminator-test-table --max-items 100
done

# Check traffic is going through NAT
# (should see private IP as source, NAT Gateway as next hop)
```

---

### Step 3: Run termiNATor Tests

#### Test 1: Quick Win Scan (Baseline)

```bash
# Run Quick Win Scan
./terminator-cli scan quick \
  --region us-east-1 \
  --output test/results/quick-scan-baseline.json

# Expected findings:
# - Missing S3 gateway endpoint in VPC vpc-xxx
# - Missing DynamoDB gateway endpoint in VPC vpc-xxx
```

#### Test 2: Deep Dive (Baseline - No Endpoints)

```bash
# Generate traffic for 5 minutes
./test/scripts/generate-traffic.sh --instance-id <ID> --duration 5

# Run Deep Dive (15 min collection window)
./terminator-cli scan deep \
  --region us-east-1 \
  --nat-gateway-ids <NAT_GW_ID> \
  --duration 15 \
  --output test/results/deep-dive-baseline.json

# Expected results:
# - S3 traffic: > 0 GB (should match generated traffic volume)
# - DynamoDB traffic: > 0 GB
# - Estimated monthly savings: $X.XX (based on NAT data processing)
```

#### Test 3: Deep Dive (After S3 Endpoint)

```bash
# Add S3 Gateway endpoint
aws ec2 create-vpc-endpoint \
  --vpc-id <VPC_ID> \
  --service-name com.amazonaws.us-east-1.s3 \
  --route-table-ids <PRIVATE_RT_ID>

# Generate traffic again
./test/scripts/generate-traffic.sh --instance-id <ID> --duration 5

# Run Deep Dive
./terminator-cli scan deep \
  --region us-east-1 \
  --nat-gateway-ids <NAT_GW_ID> \
  --duration 15 \
  --output test/results/deep-dive-s3-endpoint.json

# Expected results:
# - S3 traffic: 0 GB (traffic now goes via endpoint)
# - DynamoDB traffic: > 0 GB (still via NAT)
# - Estimated savings reduced (only DynamoDB avoidable now)
```

#### Test 4: Deep Dive (After Both Endpoints)

```bash
# Add DynamoDB Gateway endpoint
aws ec2 create-vpc-endpoint \
  --vpc-id <VPC_ID> \
  --service-name com.amazonaws.us-east-1.dynamodb \
  --route-table-ids <PRIVATE_RT_ID>

# Generate traffic again
./test/scripts/generate-traffic.sh --instance-id <ID> --duration 5

# Run Deep Dive
./terminator-cli scan deep \
  --region us-east-1 \
  --nat-gateway-ids <NAT_GW_ID> \
  --duration 15 \
  --output test/results/deep-dive-both-endpoints.json

# Expected results:
# - S3 traffic: 0 GB
# - DynamoDB traffic: 0 GB
# - Estimated savings: $0 (no avoidable NAT traffic)
```

---

## Validation Criteria

### Quick Win Scan Validation

| Test Case | Expected Finding | Pass Criteria |
|-----------|------------------|---------------|
| No endpoints exist | Missing S3 and DynamoDB endpoints | Both flagged as missing |
| Endpoint exists but not associated | Endpoint not associated with private RT | Flagged with route table recommendation |
| Endpoints properly configured | No findings | No recommendations for S3/DynamoDB |

### Deep Dive Validation

| Scenario | S3 Traffic (GB) | DynamoDB Traffic (GB) | Savings Estimate |
|----------|-----------------|----------------------|------------------|
| Baseline (no endpoints) | > 0 | > 0 | > $0 |
| S3 endpoint added | 0 | > 0 | Reduced (DDB only) |
| Both endpoints added | 0 | 0 | $0 |

### Flow Log Validation

```bash
# Verify Flow Logs were created
aws ec2 describe-flow-logs \
  --filter "Name=tag:CreatedBy,Values=termiNATor" \
  --region us-east-1

# Verify Flow Logs contain pkt-dstaddr field
aws logs get-log-events \
  --log-group-name <LOG_GROUP> \
  --log-stream-name <LOG_STREAM> \
  --limit 10 \
  | jq '.events[].message' \
  | grep pkt-dstaddr

# Verify cleanup removed Flow Logs
aws ec2 describe-flow-logs \
  --filter "Name=tag:RunId,Values=<RUN_ID>" \
  --region us-east-1
# Should return empty after cleanup
```

---

## Automated Test Suite

### Running All Tests

```bash
# Run complete test suite
./test/run-all-tests.sh

# This script:
# 1. Deploys test infrastructure
# 2. Runs baseline tests (no endpoints)
# 3. Adds S3 endpoint and reruns
# 4. Adds DynamoDB endpoint and reruns
# 5. Validates all results
# 6. Generates test report
# 7. Cleans up infrastructure
```

### Test Script Structure

```
test/
├── infrastructure/
│   ├── test-stack.yaml           # CloudFormation template
│   └── test-stack-regional.yaml  # Regional NAT variant
├── scripts/
│   ├── generate-traffic.sh       # Traffic generation
│   ├── validate-results.sh       # Result validation
│   └── cleanup.sh                # Manual cleanup helper
├── fixtures/
│   ├── test-file-1mb.bin         # Sample S3 object
│   └── dynamodb-seed-data.json   # Sample DynamoDB items
├── results/
│   └── .gitkeep                  # Test results stored here
└── run-all-tests.sh              # Main test orchestrator
```

---

## Cleanup

### Automated Cleanup

```bash
# Delete CloudFormation stack (removes all resources)
aws cloudformation delete-stack \
  --stack-name terminator-test-infra \
  --region us-east-1

# Wait for deletion
aws cloudformation wait stack-delete-complete \
  --stack-name terminator-test-infra \
  --region us-east-1
```

### Manual Cleanup (if stack deletion fails)

```bash
# Run cleanup script
./test/scripts/cleanup.sh --region us-east-1

# Script removes:
# - VPC endpoints created during tests
# - Flow Logs created by termiNATor
# - CloudWatch Log Groups
# - Test S3 bucket contents
# - DynamoDB table
# - NAT Gateway (waits for deletion)
# - Elastic IP
# - VPC and subnets
```

### Verify Cleanup

```bash
# Check for orphaned resources
aws ec2 describe-nat-gateways \
  --filter "Name=tag:Purpose,Values=termiNATor-test" \
  --region us-east-1

aws ec2 describe-flow-logs \
  --filter "Name=tag:CreatedBy,Values=termiNATor" \
  --region us-east-1

aws logs describe-log-groups \
  --log-group-name-prefix "/aws/vpc/flowlogs/terminator" \
  --region us-east-1
```

---

## Cost Estimation for Testing

### Per Test Run Costs (15-minute Deep Dive)

| Resource | Cost | Notes |
|----------|------|-------|
| NAT Gateway (hourly) | ~$0.07 | $0.045/hour × 1.5 hours |
| NAT Gateway (data processing) | ~$0.05 | ~1 GB test traffic × $0.045/GB |
| VPC Flow Logs (CloudWatch) | ~$0.01 | ~20 MB logs × $0.50/GB |
| EC2 t3.micro (hourly) | ~$0.02 | $0.0104/hour × 1.5 hours |
| **Total per test run** | **~$0.15** | Approximate |

### Full Test Suite Cost

- Baseline test: $0.15
- S3 endpoint test: $0.15
- DynamoDB endpoint test: $0.15
- **Total: ~$0.45** per complete test suite run

### Monthly Testing Budget

- Daily testing (1 run/day): ~$4.50/month
- Weekly testing (1 run/week): ~$1.80/month
- CI/CD per PR (assume 10 PRs/month): ~$1.50/month

---

## Troubleshooting

### Issue: Traffic not going through NAT

**Symptoms:** Deep Dive shows 0 bytes for S3/DynamoDB even without endpoints

**Checks:**
```bash
# Verify route table
aws ec2 describe-route-tables --route-table-ids <PRIVATE_RT_ID>
# Should show 0.0.0.0/0 → nat-xxx

# Verify EC2 instance subnet
aws ec2 describe-instances --instance-ids <INSTANCE_ID> \
  --query 'Reservations[0].Instances[0].SubnetId'
# Should match private subnet

# Check security group allows outbound
aws ec2 describe-security-groups --group-ids <SG_ID>
```

### Issue: Flow Logs not capturing traffic

**Symptoms:** Flow Logs exist but show no records

**Checks:**
```bash
# Verify Flow Log status
aws ec2 describe-flow-logs --flow-log-ids <FLOW_LOG_ID>
# Status should be "ACTIVE"

# Check CloudWatch Log Group
aws logs describe-log-streams \
  --log-group-name <LOG_GROUP> \
  --order-by LastEventTime \
  --descending
# Should show recent log streams

# Verify log format includes pkt-dstaddr
aws ec2 describe-flow-logs --flow-log-ids <FLOW_LOG_ID> \
  --query 'FlowLogs[0].LogFormat'
```

### Issue: termiNATor misclassifies traffic

**Symptoms:** S3 traffic reported as "Other" or incorrect service

**Checks:**
```bash
# Manually inspect Flow Logs
aws logs filter-log-events \
  --log-group-name <LOG_GROUP> \
  --filter-pattern "ACCEPT" \
  --limit 20

# Verify pkt-dstaddr matches S3 IP ranges
# Download AWS IP ranges
curl -s https://ip-ranges.amazonaws.com/ip-ranges.json \
  | jq '.prefixes[] | select(.service=="S3" and .region=="us-east-1")'

# Check if pkt-dstaddr falls in S3 ranges
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: termiNATor Integration Tests

on:
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 2 * * 1'  # Weekly on Monday 2 AM

jobs:
  integration-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v2
        with:
          role-to-assume: arn:aws:iam::381491970320:role/GitHubActionsRole
          aws-region: us-east-1
      
      - name: Run integration tests
        run: |
          ./test/run-all-tests.sh
      
      - name: Upload test results
        uses: actions/upload-artifact@v3
        with:
          name: test-results
          path: test/results/
      
      - name: Cleanup on failure
        if: failure()
        run: |
          ./test/scripts/cleanup.sh --region us-east-1
```

---

## Test Checklist for Contributors

Before submitting a PR that changes detection logic, run:

- [ ] Deploy test infrastructure
- [ ] Run baseline Deep Dive (no endpoints)
- [ ] Verify S3 and DynamoDB traffic detected
- [ ] Add S3 endpoint, rerun, verify S3 traffic = 0
- [ ] Add DynamoDB endpoint, rerun, verify DynamoDB traffic = 0
- [ ] Check Flow Logs cleanup completed
- [ ] Verify cost estimates are reasonable
- [ ] Delete test infrastructure
- [ ] Review test results in `test/results/`

---

## Next Steps

1. **Implement test infrastructure CloudFormation template** (`test/infrastructure/test-stack.yaml`)
2. **Create traffic generation script** (`test/scripts/generate-traffic.sh`)
3. **Build validation script** (`test/scripts/validate-results.sh`)
4. **Create main test orchestrator** (`test/run-all-tests.sh`)
5. **Document expected test outputs** (sample JSON reports)
6. **Set up CI/CD integration** (GitHub Actions or similar)
