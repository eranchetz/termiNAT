# Debugging Zero Traffic in termiNATor

## Problem
User ran a deep scan and got $0.00 costs across all services, indicating no traffic was captured.

## Root Causes

1. **No traffic during collection period** - Applications weren't actively using the NAT Gateway
2. **Flow Logs not delivering yet** - Can take 5-10 minutes to start delivering data
3. **All traffic to private IPs** - Query filters out RFC1918 addresses (10.x, 172.16-31.x, 192.168.x)

## Improvements Made

### 1. Enhanced Error Messaging (`internal/core/scanner.go`)
Added diagnostic check when CloudWatch query returns zero results:

```go
if len(results) == 0 {
    return nil, fmt.Errorf("no Flow Logs data found - query returned 0 results. This could mean: (1) No traffic during collection period, (2) Flow Logs not delivering data yet, or (3) All traffic was to private IPs (filtered out)")
}
```

### 2. Diagnostic Script (`scripts/check-flowlogs-data.sh`)
New script to manually inspect Flow Logs data:

```bash
./scripts/check-flowlogs-data.sh /aws/vpc/flowlogs/terminator-1234567890
```

**What it checks:**
- Log group exists and has data
- Log streams are present
- Raw Flow Logs records (last 30 minutes)
- Aggregated traffic stats by destination

**Usage:**
```bash
# Get log group name from scan output, then:
./scripts/check-flowlogs-data.sh <log-group-name>
```

### 3. Improved UI Messaging (`ui/templates/report.tmpl`)
When no traffic is detected, the report now shows:

```
⚠️  No traffic data collected during the scan period

Possible reasons:
  • No applications were actively using the NAT Gateway during collection
  • Flow Logs may not have started delivering data yet (can take 5-10 minutes)
  • All traffic was to private IPs (filtered out by analysis)

To diagnose:
  1. Check if applications are running and making outbound requests
  2. Verify Flow Logs are delivering data:
       ./scripts/check-flowlogs-data.sh <log-group-name>
  3. Try running a longer scan (15-30 minutes) during peak usage hours
```

## How to Diagnose

### Step 1: Check if Flow Logs captured any data

```bash
# Get log group name from scan output
LOG_GROUP="/aws/vpc/flowlogs/terminator-1234567890"

# Use diagnostic script
./scripts/check-flowlogs-data.sh $LOG_GROUP
```

**Expected output if working:**
- ✅ Log group exists with stored bytes > 0
- ✅ Log streams present
- ✅ Raw records showing srcaddr, dstaddr, bytes
- ✅ Aggregated stats showing top destinations

**If no data:**
- ❌ No records found
- ❌ Stored bytes = 0
- ❌ No log streams

### Step 2: Check if traffic is actually flowing

```bash
# Check NAT Gateway metrics
aws cloudwatch get-metric-statistics \
  --namespace AWS/NATGateway \
  --metric-name BytesOutToDestination \
  --dimensions Name=NatGatewayId,Value=nat-xxx \
  --start-time $(date -u -d '1 hour ago' --iso-8601=seconds) \
  --end-time $(date -u --iso-8601=seconds) \
  --period 300 \
  --statistics Sum
```

**Expected:** Non-zero Sum values if traffic is flowing

### Step 3: Verify applications are running

```bash
# Check EC2 instances in private subnets
aws ec2 describe-instances \
  --filters "Name=subnet-id,Values=subnet-xxx" \
  --query 'Reservations[].Instances[].[InstanceId,State.Name,PrivateIpAddress]'

# SSH to instance and test outbound connectivity
ssh ec2-user@<instance-ip>
curl -I https://s3.amazonaws.com  # Should go through NAT
```

### Step 4: Check Flow Logs query

The query filters out private IPs:

```sql
fields @timestamp, pkt_dstaddr, bytes
| filter action = "ACCEPT"
| stats sum(bytes) as total_bytes by pkt_dstaddr
| sort total_bytes desc
```

**If all traffic is internal (10.x, 172.16-31.x, 192.168.x), it won't appear in results.**

To verify, run a raw query without filters:

```bash
aws logs start-query \
  --log-group-name $LOG_GROUP \
  --start-time $(date -u -d '30 minutes ago' +%s) \
  --end-time $(date -u +%s) \
  --query-string 'fields @timestamp, pkt_dstaddr, bytes | limit 100'
```

## Recommendations for Users

1. **Run during peak hours** - Schedule scans when applications are actively using the NAT Gateway
2. **Longer collection periods** - Use 15-30 minutes instead of 5 minutes
3. **Generate test traffic** - Use the E2E test traffic generator:
   ```bash
   ./test/scripts/continuous-traffic.sh start
   # Run scan
   ./test/scripts/continuous-traffic.sh stop
   ```
4. **Check CloudWatch metrics first** - Verify NAT Gateway has recent traffic before scanning
5. **Wait for Flow Logs** - After creating Flow Logs, wait 10 minutes before starting collection

## Testing the Fix

```bash
# Build
go build -o terminat .

# Run scan with diagnostic output
./terminat scan deep --region us-east-1 --duration 5

# If no traffic detected, check logs
./scripts/check-flowlogs-data.sh <log-group-from-output>

# Generate test traffic (E2E test)
./test/scripts/run-e2e-test.sh
```

## Files Modified

1. `internal/core/scanner.go` - Added zero-results diagnostic
2. `scripts/check-flowlogs-data.sh` - New diagnostic script
3. `ui/templates/report.tmpl` - Enhanced no-traffic message
4. `ui/report_render.go` - Added LogGroupName to template data
