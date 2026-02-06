# Tasks 3, 4, 5 - Implementation Summary

## ✅ Task 4: Fix Interface Endpoint AZ Count Detection

**Problem**: Interface endpoints cost $0.01/hour per AZ, but code hardcoded `azCount = 1`, underestimating costs for multi-AZ deployments.

**Solution**:
- Added `SubnetIDs []string` field to `VPCEndpoint` struct
- Populated from AWS API response (`ep.SubnetIds`)
- Changed cost calculation to use `len(ep.SubnetIDs)` with fallback to 1

**Impact**:
- 3-AZ endpoint now correctly shows $21.60/month instead of $7.20/month
- Accurate cost reporting for Interface endpoints

**Files changed**: 3 files, 5 lines
- `pkg/types/types.go`
- `internal/aws/ec2.go`
- `internal/analysis/endpoints.go`

---

## ✅ Task 3: Sanitize Tag Values in CLI Commands

**Problem**: Tag values from AWS resources could contain shell metacharacters or ANSI escape sequences, creating security risks when displayed or used in generated CLI commands.

**Solution**:
- Added `shellQuote(s string)` helper - wraps values in single quotes, escapes embedded quotes
- Added `sanitizeForDisplay(s string)` helper - strips control characters (ASCII 0-31)
- Applied `shellQuote()` to all AWS CLI command generation
- Applied `sanitizeForDisplay()` to route table names from tags

**Security improvements**:
- Shell injection prevented: `foo; rm -rf /` → `'foo; rm -rf /'`
- Terminal injection prevented: ANSI escape sequences stripped from display
- All generated commands safe for copy-paste execution

**Files changed**: 2 files, 39 lines
- `internal/analysis/endpoints.go`
- `internal/analysis/recommendations.go`

---

## ✅ Task 5: Add Flow Logs Cost Estimation Before Scan

**Problem**: Users approved deep scans without knowing actual costs. High-throughput NAT Gateways could generate expensive Flow Logs data.

**Solution**:
- Added CloudWatch Metrics client to Scanner
- Implemented `EstimateFlowLogsCost()` method:
  - Queries `BytesOutToDestination` and `BytesInFromDestination` metrics from last hour
  - Extrapolates to scan duration (includes 5-min startup)
  - Uses 0.5x multiplier for Flow Logs overhead
  - Calculates cost at $0.50/GB ingestion
- Updated approval prompt to show dynamic estimate
- Graceful fallback to static text if metrics unavailable

**User experience**:
```
📊 Estimated Costs:
   • Estimated flow log data: ~0.15 GB (based on current NAT throughput)
   • Flow Logs ingestion (~$0.50/GB): ~$0.08
   • CloudWatch storage (~$0.03/GB/month): ~$0.0045/month
```

**Files changed**: 4 files, 108 lines
- `internal/core/scanner.go` - EstimateFlowLogsCost method
- `ui/deep_scan.go` - cost estimation integration
- `go.mod` / `go.sum` - CloudWatch SDK dependency

---

## Summary

| Task | Priority | Files | Lines | Status |
|------|----------|-------|-------|--------|
| 4 - AZ Count | MEDIUM | 3 | 5 | ✅ Complete |
| 3 - Sanitize | MEDIUM | 2 | 39 | ✅ Complete |
| 5 - Cost Est | MEDIUM | 4 | 108 | ✅ Complete |

**Total**: 9 files changed, 152 lines added

**Commits**:
1. `883b16c` - Tasks 3 & 4: Sanitize CLI commands and detect actual Interface endpoint AZ count
2. `dcd7dcb` - Task 5: Add Flow Logs cost estimation before scan

**Testing**:
- ✅ All code compiles successfully
- ✅ `gofmt` formatting applied
- ✅ shellQuote() tested with malicious inputs
- ✅ sanitizeForDisplay() tested with ANSI sequences
- ✅ No new test failures (no unit tests exist)

**Next steps**:
- Consider adding unit tests for new functions
- Test E2E with actual AWS resources
- Update AGENTS.md to reflect completed tasks
