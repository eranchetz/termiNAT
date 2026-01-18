#!/bin/bash
set -e

echo "🚀 Starting termiNATor E2E Test"
echo "================================"
echo ""

# Check prerequisites
if [ -z "$AWS_PROFILE" ]; then
    echo "❌ Error: AWS_PROFILE environment variable not set"
    echo "   Set it with: export AWS_PROFILE=your-profile-name"
    exit 1
fi

if [ -z "$AWS_REGION" ]; then
    echo "❌ Error: AWS_REGION environment variable not set"
    echo "   Set it with: export AWS_REGION=us-east-1"
    exit 1
fi

echo "✓ AWS Profile: $AWS_PROFILE"
echo "✓ AWS Region: $AWS_REGION"
echo ""

# Verify AWS access
echo "🔐 Verifying AWS access..."
if ! aws sts get-caller-identity --profile $AWS_PROFILE > /dev/null 2>&1; then
    echo "❌ Error: Cannot access AWS with profile $AWS_PROFILE"
    echo "   Run: aws sso login --profile $AWS_PROFILE"
    exit 1
fi
ACCOUNT_ID=$(aws sts get-caller-identity --profile $AWS_PROFILE --query Account --output text)
echo "✓ AWS Account: $ACCOUNT_ID"
echo ""

# Step 1: Deploy infrastructure
echo "📦 Step 1/7: Deploying test infrastructure..."
echo "   This creates: VPC, NAT Gateway, EC2, S3, DynamoDB, ECR"
echo "   Estimated time: 3-4 minutes"
echo ""
./test/scripts/deploy-test-infra.sh

# Extract stack name from outputs
STACK_NAME=$(jq -r '.[] | select(.OutputKey=="VPCId") | .ExportName' test/results/stack-outputs.json | sed 's/-vpc-id$//')
export STACK_NAME
echo "Stack name: $STACK_NAME"
echo ""

# Step 2: Build binary
echo "🔨 Step 2/7: Building termiNATor..."
go build -o terminator .
echo "✓ Build complete"
echo ""

# Step 3: Wait for EC2 to be ready
echo "⏳ Step 3/7: Waiting for EC2 instance to initialize..."
echo "   SSM agent needs time to start (2 minutes)"
sleep 120
echo "✓ EC2 ready"
echo ""

# Step 4: Start traffic generation
echo "🚦 Step 4/7: Starting continuous traffic generation..."
echo "   This generates S3, DynamoDB, and ECR traffic through NAT Gateway"
./test/scripts/continuous-traffic.sh start
echo ""

# Give traffic a moment to start
sleep 5

# Step 5: Run deep dive scan
echo "🔍 Step 5/7: Running deep dive scan..."
echo "   - 5 min: Flow Logs initialization (AWS requirement)"
echo "   - 5 min: Traffic collection"
echo "   - 1 min: Analysis"
echo "   Total: ~11 minutes"
echo ""
./terminator scan deep --region "$REGION" --duration 5 --auto-approve --auto-cleanup
echo ""

# Run scan (will prompt for approval)
./terminator scan deep --region $AWS_REGION --duration 5

echo ""

# Step 6: Stop traffic
echo "🛑 Step 6/7: Stopping traffic generation..."
./test/scripts/continuous-traffic.sh stop
echo ""

# Step 7: Cleanup
echo "🧹 Step 7/7: Cleaning up test infrastructure..."
echo "   This deletes: CloudFormation stack, S3 bucket, DynamoDB table, Flow Logs"
echo "   Estimated time: 2-3 minutes"
echo ""
./test/scripts/cleanup.sh
echo ""

echo "✅ E2E Test Complete!"
echo "================================"
echo ""
echo "Verification Checklist:"
echo "  [ ] Traffic classification showed S3 traffic (>0%)"
echo "  [ ] Traffic classification showed DynamoDB traffic (>0%)"
echo "  [ ] VPC endpoint analysis detected 2 missing endpoints"
echo "  [ ] Remediation commands were generated"
echo "  [ ] Source IPs were tracked and displayed"
echo "  [ ] Cost estimates were calculated"
echo "  [ ] Flow Logs were stopped automatically"
echo "  [ ] All resources were cleaned up"
echo ""
echo "If any items failed, check E2E_TESTING.md for troubleshooting."
