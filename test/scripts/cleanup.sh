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
    STACK_NAME="${STACK_NAME:-terminat-test-infra}"
fi
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

# Get S3 bucket name and empty it
echo -e "${YELLOW}Emptying S3 bucket...${NC}"
BUCKET=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Outputs[?OutputKey==`TestBucketName`].OutputValue' \
    --output text 2>/dev/null || echo "")

if [ -n "$BUCKET" ]; then
    echo "Bucket: $BUCKET"
    aws s3 rm s3://$BUCKET --recursive --region "$REGION" 2>/dev/null || true
    echo -e "${GREEN}Bucket emptied${NC}"
else
    echo "No bucket found or already deleted"
fi

echo ""

# Get ECR repository name and empty it
echo -e "${YELLOW}Emptying ECR repository...${NC}"
REPO_URI=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Outputs[?OutputKey==`TestRepositoryUri`].OutputValue' \
    --output text 2>/dev/null || echo "")

if [ -n "$REPO_URI" ]; then
    REPO_NAME=$(echo "$REPO_URI" | cut -d'/' -f2)
    echo "Repository: $REPO_NAME"
    
    # Delete all images
    IMAGE_IDS=$(aws ecr list-images --repository-name "$REPO_NAME" --region "$REGION" --query 'imageIds[*]' --output json 2>/dev/null || echo "[]")
    if [ "$IMAGE_IDS" != "[]" ] && [ -n "$IMAGE_IDS" ]; then
        aws ecr batch-delete-image \
            --repository-name "$REPO_NAME" \
            --region "$REGION" \
            --image-ids "$IMAGE_IDS" >/dev/null 2>&1 || true
        echo -e "${GREEN}Repository emptied${NC}"
    else
        echo "Repository already empty"
    fi
else
    echo "No repository found or already deleted"
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

# Clean up local test results (keep stack-name.txt to reuse bucket name)
if [ -d "test/results" ]; then
    echo -e "${YELLOW}Cleaning up local test results...${NC}"
    find test/results -type f ! -name 'stack-name.txt' -delete 2>/dev/null || true
    echo -e "${GREEN}Local results cleaned (kept stack-name.txt for bucket reuse)${NC}"
fi

echo ""
echo -e "${GREEN}=== Cleanup Complete ===${NC}"
echo -e "${YELLOW}Note: stack-name.txt preserved to avoid S3 bucket name collision on next deploy${NC}"
echo -e "${YELLOW}To force a new stack name, delete test/results/stack-name.txt${NC}"
