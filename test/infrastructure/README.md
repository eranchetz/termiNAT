# termiNATor Test Infrastructure

## Quick Start

### 1. Deploy Test Infrastructure

```bash
./test/scripts/deploy-test-infra.sh
```

This creates:
- VPC with public and private subnets
- NAT Gateway in public subnet
- EC2 instance in private subnet (with SSM access)
- S3 bucket with test data
- DynamoDB table with test data

**Time:** ~5 minutes  
**Cost:** ~$0.05/hour while running

### 2. Generate Test Traffic

```bash
./test/scripts/generate-traffic.sh
```

Default: 5 minutes, 100 S3 requests/batch, 50 DynamoDB requests/batch

Custom duration:
```bash
DURATION=10 S3_REQUESTS=200 DDB_REQUESTS=100 ./test/scripts/generate-traffic.sh
```

### 3. Run termiNATor Scan

```bash
# Get NAT Gateway ID from outputs
NAT_ID=$(aws cloudformation describe-stacks \
  --stack-name terminator-test-infra \
  --query 'Stacks[0].Outputs[?OutputKey==`NATGatewayId`].OutputValue' \
  --output text)

# Run scan (when termiNATor CLI is ready)
./terminator-cli scan deep \
  --region us-east-1 \
  --nat-gateway-ids $NAT_ID \
  --duration 15
```

### 4. Add Endpoints and Retest

```bash
# Get VPC and Route Table IDs
VPC_ID=$(aws cloudformation describe-stacks \
  --stack-name terminator-test-infra \
  --query 'Stacks[0].Outputs[?OutputKey==`VPCId`].OutputValue' \
  --output text)

RT_ID=$(aws cloudformation describe-stacks \
  --stack-name terminator-test-infra \
  --query 'Stacks[0].Outputs[?OutputKey==`PrivateRouteTableId`].OutputValue' \
  --output text)

# Add S3 Gateway endpoint
aws ec2 create-vpc-endpoint \
  --vpc-id $VPC_ID \
  --service-name com.amazonaws.us-east-1.s3 \
  --route-table-ids $RT_ID

# Generate traffic again
./test/scripts/generate-traffic.sh

# Rerun scan - should show S3 traffic = 0
```

### 5. Cleanup

```bash
./test/scripts/cleanup.sh
```

Removes all resources created by the test stack.

## Stack Outputs

After deployment, get all outputs:

```bash
aws cloudformation describe-stacks \
  --stack-name terminator-test-infra \
  --query 'Stacks[0].Outputs[*].[OutputKey,OutputValue]' \
  --output table
```

Or from saved file:
```bash
cat test/results/stack-outputs.json | jq
```

## Manual Testing

### Connect to EC2 Instance

```bash
INSTANCE_ID=$(aws cloudformation describe-stacks \
  --stack-name terminator-test-infra \
  --query 'Stacks[0].Outputs[?OutputKey==`TestInstanceId`].OutputValue' \
  --output text)

aws ssm start-session --target $INSTANCE_ID
```

### Generate Traffic Manually

```bash
# On EC2 instance
/home/ec2-user/generate-traffic.sh 5 100 50
```

### Check NAT Gateway Metrics

```bash
NAT_ID=$(aws cloudformation describe-stacks \
  --stack-name terminator-test-infra \
  --query 'Stacks[0].Outputs[?OutputKey==`NATGatewayId`].OutputValue' \
  --output text)

aws cloudwatch get-metric-statistics \
  --namespace AWS/NATGateway \
  --metric-name BytesOutToDestination \
  --dimensions Name=NatGatewayId,Value=$NAT_ID \
  --start-time $(date -u -d '10 minutes ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum
```

## Troubleshooting

### Stack creation fails

Check CloudFormation events:
```bash
aws cloudformation describe-stack-events \
  --stack-name terminator-test-infra \
  --max-items 20
```

### Instance not accessible via SSM

Wait 2-3 minutes after stack creation for SSM agent to register.

Check instance status:
```bash
aws ssm describe-instance-information \
  --filters "Key=InstanceIds,Values=$INSTANCE_ID"
```

### No traffic showing in metrics

- Verify instance is in private subnet
- Check route table has 0.0.0.0/0 → NAT Gateway
- Ensure security group allows outbound traffic
- Wait 5-10 minutes for CloudWatch metrics to populate

## Cost Breakdown

| Resource | Cost/Hour | Notes |
|----------|-----------|-------|
| NAT Gateway | $0.045 | Plus $0.045/GB processed |
| EC2 t3.micro | $0.0104 | us-east-1 pricing |
| S3 bucket | Minimal | Only test data storage |
| DynamoDB | Minimal | On-demand, low usage |
| **Total** | **~$0.06/hour** | ~$1.40/day if left running |

**Important:** Always run cleanup script when done testing!
