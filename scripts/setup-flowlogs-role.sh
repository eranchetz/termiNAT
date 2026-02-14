#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

REGION="${AWS_REGION:-us-east-1}"
ROLE_NAME="termiNATor-FlowLogsRole"
POLICY_NAME="termiNATor-FlowLogsPolicy"

echo -e "${GREEN}=== Setting up IAM Role for Flow Logs ===${NC}"
echo "Region: $REGION"
echo "Role Name: $ROLE_NAME"
echo ""

# Create trust policy
cat > /tmp/flowlogs-trust-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "vpc-flow-logs.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

# Create IAM role
echo -e "${YELLOW}Creating IAM role...${NC}"
ROLE_ARN=$(aws iam create-role \
  --role-name "$ROLE_NAME" \
  --assume-role-policy-document file:///tmp/flowlogs-trust-policy.json \
  --description "Role for termiNATor Flow Logs delivery to CloudWatch Logs" \
  --query 'Role.Arn' \
  --output text 2>/dev/null || \
  aws iam get-role --role-name "$ROLE_NAME" --query 'Role.Arn' --output text)

echo "Role ARN: $ROLE_ARN"

# Create policy
cat > /tmp/flowlogs-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams"
      ],
      "Resource": "*"
    }
  ]
}
EOF

echo -e "${YELLOW}Attaching policy...${NC}"
aws iam put-role-policy \
  --role-name "$ROLE_NAME" \
  --policy-name "$POLICY_NAME" \
  --policy-document file:///tmp/flowlogs-policy.json

# Wait for role to propagate
echo -e "${YELLOW}Waiting for IAM role to propagate (10 seconds)...${NC}"
sleep 10

echo -e "${GREEN}✓ IAM role setup complete!${NC}"
echo ""
echo "Role ARN: $ROLE_ARN"
echo ""
echo "You can now run Deep Dive scans:"
echo "  ./terminat scan deep --region $REGION --duration 15"
echo ""

# Save role ARN for easy access
echo "$ROLE_ARN" > .terminat-role-arn

# Cleanup temp files
rm -f /tmp/flowlogs-trust-policy.json /tmp/flowlogs-policy.json
