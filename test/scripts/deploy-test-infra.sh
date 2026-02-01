#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Generate 3-char random suffix for uniqueness
SUFFIX=$(openssl rand -hex 2 | cut -c1-3)

# Configuration
STACK_NAME="${STACK_NAME:-terminat-test-${SUFFIX}}"
REGION="${AWS_REGION:-us-east-1}"
TEMPLATE_FILE="test/infrastructure/test-stack.yaml"

# Save stack name for other scripts
mkdir -p test/results
echo "$STACK_NAME" > test/results/stack-name.txt

echo -e "${GREEN}=== termiNATor Test Infrastructure Deployment ===${NC}"
echo "Stack Name: $STACK_NAME"
echo "Region: $REGION"
echo "Suffix: $SUFFIX"
echo ""

# Check if stack exists
if aws cloudformation describe-stacks --stack-name "$STACK_NAME" --region "$REGION" >/dev/null 2>&1; then
    echo -e "${YELLOW}Stack already exists. Updating...${NC}"
    OPERATION="update-stack"
    WAIT_COMMAND="stack-update-complete"
else
    echo -e "${GREEN}Creating new stack...${NC}"
    OPERATION="create-stack"
    WAIT_COMMAND="stack-create-complete"
fi

# Deploy stack
aws cloudformation "$OPERATION" \
    --stack-name "$STACK_NAME" \
    --template-body "file://$TEMPLATE_FILE" \
    --capabilities CAPABILITY_NAMED_IAM \
    --region "$REGION" \
    --tags Key=Project,Value=termiNATor Key=Environment,Value=test

echo -e "${YELLOW}Waiting for stack operation to complete...${NC}"
aws cloudformation wait "$WAIT_COMMAND" \
    --stack-name "$STACK_NAME" \
    --region "$REGION"

echo -e "${GREEN}Stack operation complete!${NC}"
echo ""

# Get outputs
echo -e "${GREEN}=== Stack Outputs ===${NC}"
aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Outputs[*].[OutputKey,OutputValue]' \
    --output table

# Save outputs to file for easy access
OUTPUT_FILE="test/results/stack-outputs.json"
mkdir -p test/results
aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Outputs' \
    --output json > "$OUTPUT_FILE"

echo ""
echo -e "${GREEN}Outputs saved to: $OUTPUT_FILE${NC}"
echo ""
echo -e "${GREEN}=== Next Steps ===${NC}"
echo "1. Wait ~2 minutes for EC2 instance to fully initialize"
echo "2. Generate test traffic:"
echo "   ./test/scripts/generate-traffic.sh"
echo "3. Run termiNATor scan"
echo ""
