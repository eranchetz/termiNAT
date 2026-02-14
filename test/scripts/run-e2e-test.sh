#!/bin/bash
set -e

echo "🚀 Starting termiNATor E2E Test"
echo "================================"
echo ""

# Check prerequisites
if [ -z "$AWS_PROFILE" ] && [ -z "$AWS_ACCESS_KEY_ID" ]; then
    echo "❌ Error: No AWS credentials configured"
    echo "   Set AWS_PROFILE or AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY"
    exit 1
fi

if [ -z "$AWS_REGION" ]; then
    echo "❌ Error: AWS_REGION environment variable not set"
    echo "   Set it with: export AWS_REGION=us-east-1"
    exit 1
fi

echo "✓ AWS Region: $AWS_REGION"
if [ -n "$AWS_PROFILE" ]; then
    echo "✓ AWS Profile: $AWS_PROFILE"
fi
echo ""

# Verify AWS access
echo "🔐 Verifying AWS access..."
if ! aws sts get-caller-identity > /dev/null 2>&1; then
    echo "❌ Error: Cannot access AWS"
    if [ -n "$AWS_PROFILE" ]; then
        echo "   Run: aws sso login --profile $AWS_PROFILE"
    fi
    exit 1
fi
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
echo "✓ AWS Account: $ACCOUNT_ID"
echo ""

# Step 1: Deploy infrastructure
echo "📦 Step 1/7: Deploying test infrastructure..."
echo "   This creates: VPC, NAT Gateway, EC2, S3, DynamoDB, ECR"
echo "   Estimated time: 3-4 minutes"
echo ""
./test/scripts/deploy-test-infra.sh

# Get stack name from saved file
STACK_NAME=$(cat test/results/stack-name.txt)
export STACK_NAME
echo "Stack name: $STACK_NAME"
echo ""

# Step 2: Build binary
echo "🔨 Step 2/7: Building termiNATor..."
go build -o terminat .
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

# Build scan command with optional profile
mkdir -p reports
SCAN_REPORT_MD="reports/e2e-deep-scan-$(date +%Y%m%d-%H%M%S).md"
SCAN_CMD="./terminat scan deep --region $AWS_REGION --duration 5 --auto-approve --auto-cleanup --export markdown --output $SCAN_REPORT_MD"
if [ -n "$AWS_PROFILE" ]; then
    SCAN_CMD="$SCAN_CMD --profile $AWS_PROFILE"
fi

SCAN_LOG="test/results/deep-scan-output.log"
echo "Scan output will be saved to: $SCAN_LOG"
echo "Markdown report will be saved to: $SCAN_REPORT_MD"
set +e
$SCAN_CMD 2>&1 | tee "$SCAN_LOG"
SCAN_EXIT=${PIPESTATUS[0]}
set -e
if [ $SCAN_EXIT -ne 0 ]; then
    echo "❌ Deep scan failed (exit code: $SCAN_EXIT)"
    exit $SCAN_EXIT
fi

# Fail fast on no-traffic regressions
if grep -q "Analysis complete: records=0" "$SCAN_LOG" || grep -q "No traffic records were collected in this run" "$SCAN_LOG"; then
    echo "❌ Deep scan collected zero traffic records."
    echo "   This indicates a regression or Flow Logs/query issue."
    echo "   Debug command:"
    echo "   ./scripts/check-flowlogs-data.sh \$(grep -o '/aws/vpc/flowlogs/terminat-[0-9]*' \"$SCAN_LOG\" | tail -1)"
    exit 1
fi
echo ""

# Step 6: Stop traffic
echo "🛑 Step 6/7: Stopping traffic generation..."
./test/scripts/continuous-traffic.sh stop
echo ""

# Step 7: Cleanup
echo "🧹 Step 7/7: Cleaning up test infrastructure..."
echo "   This deletes: CloudFormation stack, S3 bucket, ECR images"
echo "   Estimated time: 2-3 minutes"
echo ""
./test/scripts/cleanup.sh --yes
echo ""

echo "✅ E2E Test Complete!"
echo "================================"
echo ""
echo "Verification Checklist:"
echo "  [ ] Traffic classification showed S3 traffic (>0%)"
echo "  [ ] Traffic classification showed DynamoDB traffic (>0%)"
echo "  [ ] Traffic classification showed ECR traffic (>0%)"
echo "  [ ] VPC endpoint analysis detected missing Gateway endpoints"
echo "  [ ] Interface endpoints were listed with costs"
echo "  [ ] Remediation commands were generated"
echo "  [ ] Regional NAT Gateway recommendations shown (if applicable)"
echo "  [ ] Cost estimates were calculated"
echo "  [ ] Flow Logs were stopped automatically"
echo "  [ ] All resources were cleaned up"
echo ""
echo "If any items failed, check E2E_TESTING.md for troubleshooting."
