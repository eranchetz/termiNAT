# Release Notes - v0.7.0

Release date: 2026-02-14

## Highlights

### 1) Safer default UX on Linux/macOS terminals
- Default mode remains serial stream output (`--ui stream`), and full-screen TUI is explicitly opt-in (`--ui tui`).
- This avoids alt-screen clearing behavior for standard runs and keeps output visible in shell history workflows.

### 2) Traffic analysis reliability fixes
- Added pre-analysis wait for real Flow Logs traffic events (ignores `NODATA`/`SKIPDATA`) before querying.
- Hardened aggregated result parsing (`resolved_dst` / `dstaddr` / `pkt_dstaddr`, robust bytes parsing).
- Added fallback path to raw `@message` analysis if aggregated results produce zero records.

### 3) Better endpoint and cost guidance
- Report now includes:
  - NAT Gateway topology (with `zonal` / `regional` mode)
  - ECR interface endpoint status (`ecr.api`, `ecr.dkr`)
  - ECR remediation commands
  - Estimated regional interface endpoint pricing components

### 4) Automation improvements
- E2E script now:
  - Stores markdown report under `reports/`
  - Fails fast on zero-traffic regressions
  - Uses non-interactive cleanup via `cleanup.sh --yes`

## Issues fixed in this release

### Issue #25: "Does terminat check the timestamps of the flow logs?"
- Status: Fixed in v0.7.0
- What changed:
  - Added a wait loop for ingested traffic events before analysis:
    - `internal/core/scanner.go` (`waitForFlowLogsData`)
    - `internal/aws/cloudwatch.go` (`HasTrafficLogEvents`)
  - Query window end now uses current time when needed to reduce ingestion lag misses.
- Evidence:
  - Code paths: `internal/core/scanner.go`, `internal/aws/cloudwatch.go`

### Issue #30: "0.6.0 Price shows as zero, still missing data volume on WSL"
- Status: Fixed in v0.7.0
- What changed:
  - Aggregated parser now handles additional destination fields and numeric formats:
    - `internal/analysis/analyzer.go`
  - Added raw-message fallback analysis when aggregated parsing yields zero records:
    - `internal/core/scanner.go`
  - Query updated to parse flow-log message format consistently:
    - `internal/aws/cloudwatch.go`
- Evidence:
  - Unit tests: `internal/analysis/analyzer_test.go`
  - E2E sample report: `reports/e2e-deep-scan-20260214-104750.md` shows non-zero total GB and non-zero costs.

### Issue #32: "output is cleared from the screen on exit (WSL)"
- Status: Fixed in v0.7.0
- What changed:
  - Stream mode is the standard execution path; TUI/alt-screen only with `--ui tui`.
  - Docs updated to make this explicit:
    - `README.md`, `USAGE.md`
- Evidence:
  - CLI defaults and flags in `cmd/scan.go`

## Additional improvements included

- Doctor preflight runs by default on quick/deep scans (`--doctor=false` to skip):
  - `cmd/scan.go`
- ECR interface endpoint remediation and pricing estimate in markdown report:
  - `internal/analysis/endpoints.go`
  - `internal/report/report.go`
  - `internal/report/report_test.go`
  - `internal/analysis/endpoints_test.go`
- Consistent artifact naming and examples (`terminat-*`) in touched scripts/docs:
  - `ui/deep_scan.go`
  - `ui/deep_scan_stream.go`
  - `scripts/check-flowlogs-data.sh`
  - `scripts/setup-flowlogs-role.sh`
  - `AGENTS.md`
  - `USAGE.md`

## Verification snapshot

- Unit/integration tests:
  - `go test ./...` ✅
- E2E:
  - `test/scripts/run-e2e-test.sh` ✅
  - Report exported to `reports/` and includes NAT mode + ECR endpoint status + ECR remediation + pricing estimate.

## Upgrade notes

- No breaking CLI command changes.
- If you want previous full-screen behavior, run:
  - `terminat scan deep --ui tui`
  - `terminat scan demo --ui tui`

