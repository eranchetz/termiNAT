# termiNATor QA Testing Guide

## Pre-Testing Setup

### 1. Build the Application
```bash
cd /Users/eran/Projects/NATInspector
go build -o terminat
```

### 2. Verify AWS Credentials
```bash
export AWS_PROFILE=chetz-playground
aws sts get-caller-identity
```

### 3. Verify IAM Role Exists
```bash
aws iam get-role --role-name termiNATor-FlowLogsRole
```

If not exists, run:
```bash
./scripts/setup-flowlogs-role.sh
```

## Test Scenarios

### Test 1: Quick Scan
**Purpose**: Verify basic NAT Gateway discovery and VPC endpoint detection

```bash
./terminat scan quick --region us-east-1
```

**Expected Results:**
- ✅ Discovers NAT Gateways in the region
- ✅ Shows NAT Gateway IDs, VPCs, and subnets
- ✅ Identifies missing VPC endpoints
- ✅ Provides recommendations
- ✅ Completes in < 5 seconds

**Validation:**
- [ ] NAT Gateway count matches AWS Console
- [ ] VPC endpoint recommendations are accurate
- [ ] Output is clear and readable
- [ ] No errors or warnings

---

### Test 2: Deep Dive Scan (5 minutes)
**Purpose**: Verify Flow Logs creation, traffic collection, and analysis

```bash
./terminat scan deep --region us-east-1 --duration 5
```

**Expected Results:**
- ✅ Shows account ID and region
- ✅ Discovers NAT Gateways with full details (ID, VPC, Subnet, ENI)
- ✅ Creates Flow Logs successfully
- ✅ Shows Flow Log IDs and CloudWatch Log Group
- ✅ Waits 5 minutes for startup (Step 3/5)
- ✅ Collects traffic for 5 minutes (Step 4/5)
- ✅ Analyzes traffic and classifies by service (Step 5/5)
- ✅ Shows traffic statistics (Total, S3, DynamoDB, Other)
- ✅ Displays cost estimates with disclaimers
- ✅ Cleans up Flow Logs
- ✅ Retains log group for review
- ✅ Total time: ~10 minutes

**Validation:**
- [ ] All 5 steps complete successfully
- [ ] Traffic data is collected (check CloudWatch Logs)
- [ ] Cost estimates include disclaimer
- [ ] Flow Logs are deleted after scan
- [ ] Log group is retained
- [ ] UI is responsive and informative

---

### Test 3: Deep Dive Scan with Active Traffic
**Purpose**: Verify traffic classification with real S3/DynamoDB traffic

**Setup:**
```bash
# Start traffic generation
cd /Users/eran/Projects/NATInspector/test/scripts
DURATION=15 ./generate-traffic.sh
```

**Run Scan:**
```bash
./terminat scan deep --region us-east-1 --duration 5
```

**Expected Results:**
- ✅ Detects S3 traffic (non-zero GB)
- ✅ Detects DynamoDB traffic (non-zero GB)
- ✅ Shows percentage breakdown
- ✅ Cost estimates reflect actual traffic
- ✅ Savings calculations show S3 and DynamoDB separately

**Validation:**
- [ ] S3 traffic > 0%
- [ ] DynamoDB traffic > 0%
- [ ] Percentages add up to 100%
- [ ] Cost calculations are reasonable
- [ ] Savings estimates are shown

---

### Test 4: Cleanup Command
**Purpose**: Verify log group cleanup functionality

**List Log Groups:**
```bash
aws logs describe-log-groups --region us-east-1 --log-group-name-prefix "/aws/vpc/flowlogs/terminator" --query 'logGroups[*].[logGroupName,storedBytes]' --output table
```

**Run Cleanup:**
```bash
./terminat cleanup --region us-east-1 --log-group "/aws/vpc/flowlogs/terminator-XXXXXXXXXX"
```

**Expected Results:**
- ✅ Shows log group statistics (storage, streams)
- ✅ Checks for active Flow Logs
- ✅ Prompts for confirmation
- ✅ Deletes log group successfully
- ✅ Shows success message

**Validation:**
- [ ] Log group is deleted
- [ ] No active Flow Logs warning if applicable
- [ ] Confirmation prompt works
- [ ] Error handling for invalid log group

---

### Test 5: Error Handling

#### Test 5a: Invalid Region
```bash
./terminat scan quick --region invalid-region
```
**Expected**: Clear error message about invalid region

#### Test 5b: No NAT Gateways
```bash
./terminat scan quick --region ap-south-1
```
**Expected**: "No NAT gateways found" message (if region has no NAT Gateways)

#### Test 5c: Missing Permissions
```bash
# Use credentials without required permissions
./terminat scan deep --region us-east-1 --duration 5
```
**Expected**: Clear error about missing IAM permissions

#### Test 5d: Invalid Duration
```bash
./terminat scan deep --region us-east-1 --duration 100
```
**Expected**: Error message about duration limits (5-60 minutes)

**Validation:**
- [ ] All error messages are clear and helpful
- [ ] No stack traces or cryptic errors
- [ ] Suggestions for fixing issues are provided

---

### Test 6: UI and UX

**Verify:**
- [ ] Step-by-step explanations are clear (Step 1/5, 2/5, etc.)
- [ ] Each step explains what's happening and why
- [ ] Resource details are shown (Account, Region, NAT Gateway, VPC, Subnet, ENI)
- [ ] Progress spinner animates smoothly
- [ ] Colors and formatting enhance readability
- [ ] Cost disclaimer is prominent and clear
- [ ] Final results are well-organized

---

### Test 7: Cost Calculations

**Verify:**
- [ ] NAT Gateway pricing is $0.045/GB
- [ ] Monthly projections are extrapolated correctly
- [ ] S3 savings = S3 traffic × $0.045/GB
- [ ] DynamoDB savings = DynamoDB traffic × $0.045/GB
- [ ] Annual savings = Monthly savings × 12
- [ ] Disclaimer states "ESTIMATE based on traffic sample"
- [ ] Notes that Gateway VPC Endpoints are FREE

---

## Performance Testing

### Test 8: Different Collection Durations

```bash
# 5 minutes (minimum)
./terminat scan deep --region us-east-1 --duration 5

# 15 minutes (recommended)
./terminat scan deep --region us-east-1 --duration 15

# 30 minutes (comprehensive)
./terminat scan deep --region us-east-1 --duration 30
```

**Validation:**
- [ ] All durations work correctly
- [ ] Total time = duration + 5 minutes
- [ ] Longer durations provide more data
- [ ] Cost estimates scale appropriately

---

## Edge Cases

### Test 9: Multiple NAT Gateways
**Setup**: Test in VPC with multiple NAT Gateways
**Expected**: All NAT Gateways are discovered and analyzed

### Test 10: No Traffic
**Setup**: Run scan when NAT Gateway has no traffic
**Expected**: Shows 0 traffic, $0 costs, clear message

### Test 11: Interrupt Scan (Ctrl+C)
**Setup**: Start scan and press Ctrl+C during collection
**Expected**: Graceful cleanup, log group retained

---

## Documentation Review

### Test 12: README Accuracy
- [ ] Installation instructions work
- [ ] Quick start examples are correct
- [ ] All commands are documented
- [ ] Troubleshooting section is helpful

### Test 13: IAM Setup Script
```bash
./scripts/setup-flowlogs-role.sh
```
- [ ] Creates role successfully
- [ ] Attaches correct policies
- [ ] Outputs role ARN

---

## Final Checklist

- [ ] All test scenarios pass
- [ ] No critical bugs found
- [ ] Error messages are clear
- [ ] UI is polished and professional
- [ ] Documentation is accurate
- [ ] Cost calculations are correct
- [ ] Disclaimers are prominent
- [ ] Performance is acceptable
- [ ] Code is ready for GitHub

---

## Known Issues / Notes

Document any issues found during testing:

1. 
2. 
3. 

---

## Sign-off

- Tester: _______________
- Date: _______________
- Version: _______________
- Status: [ ] PASS [ ] FAIL [ ] NEEDS WORK
