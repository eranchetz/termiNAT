#!/bin/bash
# Continuous traffic generator for E2E testing
# Runs in background and generates traffic until stopped

set -e

# Get stack name from saved file or use default
if [ -f "test/results/stack-name.txt" ]; then
    STACK_NAME=$(cat test/results/stack-name.txt)
else
    STACK_NAME="${STACK_NAME:-terminator-test-infra}"
fi
REGION="${AWS_REGION:-us-east-1}"
PID_FILE="/tmp/terminator-traffic-gen.pid"

start_traffic() {
    echo "Starting continuous traffic generation..."
    
    INSTANCE_ID=$(aws cloudformation describe-stacks \
        --stack-name "$STACK_NAME" --region "$REGION" \
        --query 'Stacks[0].Outputs[?OutputKey==`TestInstanceId`].OutputValue' --output text)
    
    BUCKET=$(aws cloudformation describe-stacks \
        --stack-name "$STACK_NAME" --region "$REGION" \
        --query 'Stacks[0].Outputs[?OutputKey==`TestBucketName`].OutputValue' --output text)
    
    TABLE=$(aws cloudformation describe-stacks \
        --stack-name "$STACK_NAME" --region "$REGION" \
        --query 'Stacks[0].Outputs[?OutputKey==`TestTableName`].OutputValue' --output text)
    
    REPO=$(aws cloudformation describe-stacks \
        --stack-name "$STACK_NAME" --region "$REGION" \
        --query 'Stacks[0].Outputs[?OutputKey==`TestRepositoryUri`].OutputValue' --output text)
    
    # Run traffic generation for 30 minutes (longer than any scan)
    COMMAND="/home/ec2-user/generate-traffic.sh 30 50 25 $BUCKET $TABLE $REPO"
    
    COMMAND_ID=$(aws ssm send-command \
        --instance-ids "$INSTANCE_ID" \
        --document-name "AWS-RunShellScript" \
        --parameters "commands=[\"$COMMAND\"]" \
        --region "$REGION" \
        --query 'Command.CommandId' --output text)
    
    echo "$COMMAND_ID:$INSTANCE_ID" > "$PID_FILE"
    echo "Traffic generation started (Command ID: $COMMAND_ID)"
    echo "Run '$0 stop' to stop traffic generation"
}

stop_traffic() {
    if [ ! -f "$PID_FILE" ]; then
        echo "No traffic generation running"
        return
    fi
    
    IFS=':' read -r COMMAND_ID INSTANCE_ID < "$PID_FILE"
    echo "Stopping traffic generation (Command ID: $COMMAND_ID)..."
    
    aws ssm cancel-command --command-id "$COMMAND_ID" --region "$REGION" 2>/dev/null || true
    rm -f "$PID_FILE"
    echo "Traffic generation stopped"
}

status_traffic() {
    if [ ! -f "$PID_FILE" ]; then
        echo "No traffic generation running"
        return
    fi
    
    IFS=':' read -r COMMAND_ID INSTANCE_ID < "$PID_FILE"
    STATUS=$(aws ssm get-command-invocation \
        --command-id "$COMMAND_ID" --instance-id "$INSTANCE_ID" \
        --region "$REGION" --query 'Status' --output text 2>/dev/null || echo "Unknown")
    
    echo "Traffic generation status: $STATUS (Command ID: $COMMAND_ID)"
}

case "${1:-start}" in
    start) start_traffic ;;
    stop) stop_traffic ;;
    status) status_traffic ;;
    *) echo "Usage: $0 {start|stop|status}" ;;
esac
