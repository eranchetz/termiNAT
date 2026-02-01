#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration - get stack name from saved file or use default
if [ -f "test/results/stack-name.txt" ]; then
    STACK_NAME=$(cat test/results/stack-name.txt)
else
    STACK_NAME="${STACK_NAME:-terminator-test-infra}"
fi
REGION="${AWS_REGION:-us-east-1}"
DURATION="${DURATION:-5}"
S3_REQUESTS="${S3_REQUESTS:-100}"
DDB_REQUESTS="${DDB_REQUESTS:-50}"

echo -e "${GREEN}=== termiNATor Traffic Generation ===${NC}"
echo "Duration: $DURATION minutes"
echo "S3 requests per batch: $S3_REQUESTS"
echo "DynamoDB requests per batch: $DDB_REQUESTS"
echo ""

# Get instance ID from stack outputs
INSTANCE_ID=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Outputs[?OutputKey==`TestInstanceId`].OutputValue' \
    --output text)

if [ -z "$INSTANCE_ID" ]; then
    echo -e "${RED}Error: Could not find instance ID from stack outputs${NC}"
    exit 1
fi

echo "Instance ID: $INSTANCE_ID"
echo ""

# Check instance status
echo -e "${YELLOW}Checking instance status...${NC}"
INSTANCE_STATE=$(aws ec2 describe-instances \
    --instance-ids "$INSTANCE_ID" \
    --region "$REGION" \
    --query 'Reservations[0].Instances[0].State.Name' \
    --output text)

if [ "$INSTANCE_STATE" != "running" ]; then
    echo -e "${RED}Error: Instance is not running (state: $INSTANCE_STATE)${NC}"
    exit 1
fi

echo -e "${GREEN}Instance is running${NC}"
echo ""

# Get bucket and table names
BUCKET_NAME=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Outputs[?OutputKey==`TestBucketName`].OutputValue' \
    --output text)

TABLE_NAME=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Outputs[?OutputKey==`TestTableName`].OutputValue' \
    --output text)

echo "S3 Bucket: $BUCKET_NAME"
echo "DynamoDB Table: $TABLE_NAME"
echo ""

# Start traffic generation via SSM
echo -e "${YELLOW}Starting traffic generation...${NC}"
echo "This will run for $DURATION minutes"
echo ""

# Create SSM command document
COMMAND="/home/ec2-user/generate-traffic.sh $DURATION $S3_REQUESTS $DDB_REQUESTS $BUCKET_NAME $TABLE_NAME"

COMMAND_ID=$(aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters "commands=[\"$COMMAND\"]" \
    --region "$REGION" \
    --query 'Command.CommandId' \
    --output text)

echo "SSM Command ID: $COMMAND_ID"
echo ""

# Monitor command execution
echo -e "${YELLOW}Monitoring traffic generation...${NC}"
echo "Press Ctrl+C to stop monitoring (traffic will continue)"
echo ""

while true; do
    STATUS=$(aws ssm get-command-invocation \
        --command-id "$COMMAND_ID" \
        --instance-id "$INSTANCE_ID" \
        --region "$REGION" \
        --query 'Status' \
        --output text 2>/dev/null || echo "Pending")
    
    if [ "$STATUS" = "Success" ]; then
        echo -e "${GREEN}Traffic generation completed successfully${NC}"
        break
    elif [ "$STATUS" = "Failed" ] || [ "$STATUS" = "Cancelled" ]; then
        echo -e "${RED}Traffic generation failed or was cancelled${NC}"
        exit 1
    fi
    
    echo "Status: $STATUS ($(date +%H:%M:%S))"
    sleep 10
done

echo ""
echo -e "${GREEN}=== Traffic Generation Complete ===${NC}"
echo ""
echo "Next steps:"
echo "1. Run termiNATor Deep Dive scan"
echo "2. Check NAT Gateway metrics in CloudWatch"
echo "3. Verify VPC Flow Logs (if enabled)"
echo ""
