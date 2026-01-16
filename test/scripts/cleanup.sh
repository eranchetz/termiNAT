#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
STACK_NAME="${STACK_NAME:-terminator-test-infra}"
REGION="${AWS_REGION:-us-east-1}"

echo -e "${YELLOW}=== termiNATor Test Infrastructure Cleanup ===${NC}"
echo "Stack Name: $STACK_NAME"
echo "Region: $REGION"
echo ""

# Check if stack exists
if ! aws cloudformation describe-stacks --stack-name "$STACK_NAME" --region "$REGION" >/dev/null 2>&1; then
    echo -e "${GREEN}Stack does not exist. Nothing to clean up.${NC}"
    exit 0
fi

# Confirm deletion
read -p "Are you sure you want to delete the test infrastructure? (yes/no): " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
    echo -e "${YELLOW}Cleanup cancelled${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}Deleting stack...${NC}"

# Delete stack
aws cloudformation delete-stack \
    --stack-name "$STACK_NAME" \
    --region "$REGION"

echo -e "${YELLOW}Waiting for stack deletion to complete...${NC}"
echo "This may take several minutes (NAT Gateway deletion is slow)"
echo ""

aws cloudformation wait stack-delete-complete \
    --stack-name "$STACK_NAME" \
    --region "$REGION"

echo -e "${GREEN}Stack deleted successfully!${NC}"
echo ""

# Clean up local test results
if [ -d "test/results" ]; then
    echo -e "${YELLOW}Cleaning up local test results...${NC}"
    rm -rf test/results/*
    echo -e "${GREEN}Local results cleaned${NC}"
fi

echo ""
echo -e "${GREEN}=== Cleanup Complete ===${NC}"
