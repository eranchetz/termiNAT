# Task Specifications for termiNATor Fixes

## Task 4: Fix Interface Endpoint AZ Count Detection

**Goal**: Replace hardcoded `azCount = 1` with actual AZ count from AWS API data.

**Why**: Interface endpoints cost $0.01/hour per AZ. A 3-AZ endpoint costs $21.60/month, not $7.20. Current code always reports $7.20.

### Step 1: Add SubnetIDs to VPCEndpoint struct

**File**: `pkg/types/types.go`, lines 18-27

Change:
```go
type VPCEndpoint struct {
	ID          string
	VPCID       string
	ServiceName string
	Type        string // "Gateway" or "Interface"
	State       string
	RouteTables []string
	PrivateDNS  bool
	Tags        map[string]string
}
```

To:
```go
type VPCEndpoint struct {
	ID          string
	VPCID       string
	ServiceName string
	Type        string // "Gateway" or "Interface"
	State       string
	RouteTables []string
	SubnetIDs   []string // Subnets = AZs for Interface endpoints
	PrivateDNS  bool
	Tags        map[string]string
}
```

### Step 2: Populate SubnetIDs from AWS API

**File**: `internal/aws/ec2.go`, lines 117-127

The AWS SDK's `DescribeVpcEndpoints` response already includes `ep.SubnetIds` (a `[]string`). Add one line to the endpoint struct literal.

Change:
```go
		endpoint := pkgtypes.VPCEndpoint{
			ID:          *ep.VpcEndpointId,
			VPCID:       *ep.VpcId,
			ServiceName: *ep.ServiceName,
			Type:        string(ep.VpcEndpointType),
			State:       string(ep.State),
			RouteTables: ep.RouteTableIds,
			PrivateDNS:  ep.PrivateDnsEnabled != nil && *ep.PrivateDnsEnabled,
			Tags:        tags,
		}
```

To:
```go
		endpoint := pkgtypes.VPCEndpoint{
			ID:          *ep.VpcEndpointId,
			VPCID:       *ep.VpcId,
			ServiceName: *ep.ServiceName,
			Type:        string(ep.VpcEndpointType),
			State:       string(ep.State),
			RouteTables: ep.RouteTableIds,
			SubnetIDs:   ep.SubnetIds,
			PrivateDNS:  ep.PrivateDnsEnabled != nil && *ep.PrivateDnsEnabled,
			Tags:        tags,
		}
```

### Step 3: Use actual AZ count in cost calculation

**File**: `internal/analysis/endpoints.go`, lines 233-236

Change:
```go
		// Assume 1 AZ per endpoint (conservative estimate)
		// In reality, we'd need to check subnet associations
		azCount := 1
```

To:
```go
		azCount := len(ep.SubnetIDs)
		if azCount == 0 {
			azCount = 1 // Fallback for Gateway endpoints or missing data
		}
```

### Verify

Run `go build ./...` — should compile with no errors. No new imports needed.

---

## Task 3: Sanitize Tag Values in Generated CLI Commands

**Goal**: Prevent shell injection via malicious AWS resource tag values that get interpolated into CLI commands shown to users.

**Why**: Route table names come from `rt.Tags["Name"]`. A tag value like `foo; rm -rf /` could be dangerous if copy-pasted. Also, tag values with ANSI escape sequences could manipulate terminal output.

### Step 1: Add two helper functions to endpoints.go

**File**: `internal/analysis/endpoints.go`

Add these functions after the imports (after line 9), before the `EndpointAnalysis` struct:

```go
// shellQuote wraps a string in single quotes for safe shell usage.
// Single quotes inside the value are escaped as '\'' (end quote, escaped quote, start quote).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// sanitizeForDisplay strips control characters (ASCII 0-31 except newline/tab) from a string
// to prevent ANSI escape sequence injection in terminal output.
func sanitizeForDisplay(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

No new imports needed — `strings` is already imported.

### Step 2: Apply shellQuote to GetCreateEndpointCommands

**File**: `internal/analysis/endpoints.go`, line 192

Change:
```go
		cmd := fmt.Sprintf("aws ec2 create-vpc-endpoint \\\n  --vpc-id %s \\\n  --service-name %s \\\n  --route-table-ids %s",
			a.VPCID, svc, rtIDsStr)
```

To:
```go
		cmd := fmt.Sprintf("aws ec2 create-vpc-endpoint \\\n  --vpc-id %s \\\n  --service-name %s \\\n  --route-table-ids %s",
			shellQuote(a.VPCID), shellQuote(svc), rtIDsStr)
```

Note: `rtIDsStr` is a space-separated list of IDs used as multiple positional args, so each ID needs quoting individually. Change the join too (lines 186-190):

Change:
```go
	rtIDsStr := strings.Join(rtIDs, " ")
```

To:
```go
	var quotedRTIDs []string
	for _, id := range rtIDs {
		quotedRTIDs = append(quotedRTIDs, shellQuote(id))
	}
	rtIDsStr := strings.Join(quotedRTIDs, " ")
```

### Step 3: Apply shellQuote to GetAddRouteCommands

**File**: `internal/analysis/endpoints.go`, line 214

Change:
```go
		cmd := fmt.Sprintf("aws ec2 modify-vpc-endpoint \\\n  --vpc-endpoint-id %s \\\n  --add-route-table-ids %s",
			endpointID, mr.RouteTableID)
```

To:
```go
		cmd := fmt.Sprintf("aws ec2 modify-vpc-endpoint \\\n  --vpc-endpoint-id %s \\\n  --add-route-table-ids %s",
			shellQuote(endpointID), shellQuote(mr.RouteTableID))
```

### Step 4: Apply shellQuote to recommendations.go

**File**: `internal/analysis/recommendations.go`, lines 63-67

Change:
```go
				Commands: []string{
					fmt.Sprintf("# Create Regional NAT Gateway for VPC %s", vpcID),
					fmt.Sprintf("aws ec2 create-nat-gateway \\"),
					fmt.Sprintf("  --vpc-id %s \\", vpcID),
```

To:
```go
				Commands: []string{
					fmt.Sprintf("# Create Regional NAT Gateway for VPC %s", vpcID),
					fmt.Sprintf("aws ec2 create-nat-gateway \\"),
					fmt.Sprintf("  --vpc-id %s \\", shellQuote(vpcID)),
```

This requires importing the helper. Since `shellQuote` is in `analysis` package and `recommendations.go` is also in `analysis` package — no import needed, it's the same package.

### Step 5: Sanitize tag values in display output

**File**: `internal/analysis/endpoints.go`, line 77

Change:
```go
		rtName := rt.Tags["Name"]
```

To:
```go
		rtName := sanitizeForDisplay(rt.Tags["Name"])
```

### Verify

Run `go build ./...` — should compile. No new dependencies.

---

## Task 5: Add Flow Logs Cost Estimation Before Scan

**Goal**: Before the user approves the deep scan, query CloudWatch metrics for the NAT Gateway's current throughput and show an estimated Flow Logs ingestion cost.

**Why**: Issue #13. Users should know the cost before committing. A high-throughput NAT could generate GBs of Flow Logs data at $0.50/GB.

### Step 1: Add CloudWatch Metrics client to Scanner

**File**: `internal/core/scanner.go`

Add `cloudwatch` to imports:
```go
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
```

Add field to Scanner struct (line ~21):
```go
type Scanner struct {
	region    string
	accountID string
	ec2Client *aws.EC2Client
	cwlClient *aws.CloudWatchLogsClient
	iamClient *iam.Client
	cwClient  *cloudwatch.Client // ADD THIS
}
```

In `NewScanner()`, after the existing client creation (around line 50, after `iamClient: iam.NewFromConfig(cfg)`), add:
```go
	cwClient:  cloudwatch.NewFromConfig(cfg),
```

### Step 2: Add EstimateFlowLogsCost method

**File**: `internal/core/scanner.go`

Add this method at the end of the file (after `CalculateCosts`):

```go
// EstimateFlowLogsCost estimates the CloudWatch Logs ingestion cost for a deep scan
// by querying recent NAT Gateway throughput from CloudWatch metrics.
// Returns estimated GB of flow log data and cost in USD, or (0, 0, err) on failure.
func (s *Scanner) EstimateFlowLogsCost(ctx context.Context, natIDs []string, durationMinutes int) (estimatedGB float64, estimatedCost float64, err error) {
	now := time.Now()
	startTime := now.Add(-1 * time.Hour)

	var totalBytes float64
	for _, natID := range natIDs {
		for _, metricName := range []string{"BytesOutToDestination", "BytesInFromDestination"} {
			result, err := s.cwClient.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
				Namespace:  strPtr("AWS/NATGateway"),
				MetricName: strPtr(metricName),
				Dimensions: []cloudwatchtypes.Dimension{
					{Name: strPtr("NatGatewayId"), Value: strPtr(natID)},
				},
				StartTime:  &startTime,
				EndTime:    &now,
				Period:     int32Ptr(3600),
				Statistics: []cloudwatchtypes.Statistic{cloudwatchtypes.StatisticSum},
			})
			if err != nil {
				return 0, 0, fmt.Errorf("failed to get NAT metrics: %w", err)
			}
			for _, dp := range result.Datapoints {
				if dp.Sum != nil {
					totalBytes += *dp.Sum
				}
			}
		}
	}

	// Extrapolate: bytes in last hour → bytes during scan duration
	// Flow Logs generate ~40-50 bytes per record, roughly 1:1 ratio with actual traffic bytes
	// but we use a conservative 0.5x multiplier since flow log records are smaller than payload
	bytesPerHour := totalBytes
	scanHours := float64(durationMinutes+5) / 60.0 // include 5-min startup
	estimatedFlowLogBytes := bytesPerHour * scanHours * 0.5
	estimatedGB = estimatedFlowLogBytes / (1024 * 1024 * 1024)
	estimatedCost = estimatedGB * 0.50 // $0.50/GB ingestion

	return estimatedGB, estimatedCost, nil
}
```

Add required imports to the file's import block:
```go
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
```

Add helper functions (if not already present):
```go
func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }
```

### Step 3: Call EstimateFlowLogsCost after NAT discovery

**File**: `ui/deep_scan.go`

Add a field to `deepScanModel` struct (around line 65):
```go
	estimatedScanCostGB  float64
	estimatedScanCostUSD float64
```

In the `discoverNATs()` function (around line 700, after `recommendations := analysis.AnalyzeNATGatewaySetup(nats)`), add:

```go
	// Estimate scan cost from recent NAT throughput
	var natIDs []string
	for _, nat := range nats {
		natIDs = append(natIDs, nat.ID)
	}
	estGB, estCost, _ := m.scanner.EstimateFlowLogsCost(m.ctx, natIDs, m.duration)
```

Update the `deepNatsDiscoveredMsg` struct to carry these values, or store them directly. The simplest approach: add fields to the message type.

Find the message type (search for `deepNatsDiscoveredMsg`):
```go
type deepNatsDiscoveredMsg struct {
	nats            []types.NATGateway
	recommendations []analysis.Recommendation
}
```

Change to:
```go
type deepNatsDiscoveredMsg struct {
	nats            []types.NATGateway
	recommendations []analysis.Recommendation
	estGB           float64
	estCost         float64
}
```

Update the return in `discoverNATs()`:
```go
	return deepNatsDiscoveredMsg{nats: nats, recommendations: recommendations, estGB: estGB, estCost: estCost}
```

In the `Update()` function where `deepNatsDiscoveredMsg` is handled, store the values:
```go
	case deepNatsDiscoveredMsg:
		m.nats = msg.nats
		m.recommendations = msg.recommendations
		m.estimatedScanCostGB = msg.estGB
		m.estimatedScanCostUSD = msg.estCost
```

### Step 4: Update the approval prompt to show dynamic estimate

**File**: `ui/deep_scan.go`, in `renderApprovalPrompt()` (around line 365)

Change:
```go
	b.WriteString(stepStyle.Render("\n📊 Estimated Costs:\n"))
	b.WriteString("   • Flow Logs ingestion: ~$0.50 per GB\n")
	b.WriteString("   • CloudWatch Logs storage: ~$0.03 per GB/month\n")
	b.WriteString("   • For a 5-minute scan, typical cost: < $0.10\n")
```

To:
```go
	b.WriteString(stepStyle.Render("\n📊 Estimated Costs:\n"))
	if m.estimatedScanCostGB > 0 {
		b.WriteString(fmt.Sprintf("   • Estimated flow log data: ~%.2f GB (based on current NAT throughput)\n", m.estimatedScanCostGB))
		b.WriteString(fmt.Sprintf("   • Flow Logs ingestion (~$0.50/GB): ~$%.2f\n", m.estimatedScanCostUSD))
		b.WriteString(fmt.Sprintf("   • CloudWatch storage (~$0.03/GB/month): ~$%.4f/month\n", m.estimatedScanCostGB*0.03))
	} else {
		b.WriteString("   • Flow Logs ingestion: ~$0.50 per GB\n")
		b.WriteString("   • CloudWatch Logs storage: ~$0.03 per GB/month\n")
		b.WriteString("   • For a 5-minute scan, typical cost: < $0.10\n")
	}
```

### Verify

1. Run `go build ./...` — must compile
2. The new CloudWatch import needs `go mod tidy` if `cloudwatch` isn't already in go.mod (it likely is since `cloudwatchlogs` is already used, but they're separate packages)

### Fallback behavior

If `EstimateFlowLogsCost` fails (no metrics, permissions issue, new NAT with no history), the error is ignored (`_`) and `estGB`/`estCost` remain 0. The prompt falls back to the existing static text. No user-facing error.

---

## Summary

| Task | Files Changed | Lines Added/Changed |
|------|--------------|-------------------|
| 4 (AZ count) | `types.go`, `ec2.go`, `endpoints.go` | ~5 lines |
| 3 (sanitize) | `endpoints.go`, `recommendations.go` | ~25 lines |
| 5 (cost estimate) | `scanner.go`, `deep_scan.go` | ~50 lines |
