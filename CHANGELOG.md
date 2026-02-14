# Changelog

All notable changes to termiNATor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.0] - 2026-02-14

### Added
- Doctor preflight now runs by default for `scan quick` and `scan deep`, with opt-out via `--doctor=false`.
- Explicit ECR interface endpoint analysis in reports, including endpoint status, remediation commands, and regional price estimate (PrivateLink table).
- NAT topology section in markdown reports showing gateway mode (`zonal`/`regional`).
- New analysis tests for ECR interface endpoint remediation and pricing behavior in `internal/analysis/endpoints_test.go`.

### Changed
- Stream output is now the default UX; full-screen Bubble Tea UI is opt-in via `--ui tui`.
- E2E script now exports markdown reports into `reports/` and runs cleanup non-interactively (`--yes`) for automation.
- Export default filenames are now `terminat-report-*` for consistency.

### Fixed
- Fixed zero-price/missing-volume regressions by hardening aggregated query parsing and adding raw-message fallback analysis when aggregated parsing yields zero records (Issue #30).
- Added pre-analysis wait for non-`NODATA`/`SKIPDATA` flow-log events before running Insights query to reduce empty-result races (Issue #25).
- Prevented terminal output loss for default runs by avoiding alt-screen/TUI unless explicitly requested (Issue #32).
- Cleaned remaining docs/examples/scripts drift from `terminator` to `terminat` in touched operational paths.

### Verification
- `go test ./...` passes.
- E2E deep scan produced non-zero traffic records and markdown report persisted under `reports/`.

### Added
- Initial release of termiNATor
- Quick Scan: Instant VPC configuration analysis
- Deep Dive Scan: Real-time traffic analysis using VPC Flow Logs
- Traffic classification for S3 and DynamoDB services
- Cost estimation with monthly and annual savings projections
- Cleanup command for Flow Logs log groups
- Multi-region support with regional pricing
- Interactive terminal UI with progress tracking
- Comprehensive error handling and user guidance
- IAM role setup script for Flow Logs permissions

### Features

#### Quick Scan
- Discovers NAT Gateways in specified region
- Identifies missing VPC endpoints for S3 and DynamoDB
- Checks route table configurations
- Provides immediate recommendations
- No Flow Logs required

#### Deep Dive Scan
- Creates temporary VPC Flow Logs for traffic analysis
- 5-minute startup delay for Flow Logs initialization
- Configurable collection duration (5-60 minutes)
- Real-time traffic classification by destination service
- Downloads and caches AWS IP ranges
- Calculates data volumes per service
- Estimates current NAT Gateway costs
- Projects potential savings with VPC endpoints
- Retains log data for post-analysis review
- Automatic Flow Logs cleanup

#### Cost Analysis
- Regional NAT Gateway pricing ($0.045/GB in most regions)
- Monthly cost projections based on traffic samples
- Breakdown by service (S3, DynamoDB, Other)
- Annual savings calculations
- Clear disclaimers about estimate accuracy
- Highlights that Gateway VPC Endpoints are FREE

#### User Experience
- Verbose explanatory messages at each step
- Displays AWS account ID and region
- Shows detailed resource information (NAT Gateway, VPC, Subnet, ENI)
- Progress tracking with spinner animations
- Color-coded output for better readability
- Step-by-step guidance (Step 1/5, 2/5, etc.)
- Clear error messages with troubleshooting hints

### Technical Details
- Built with Go 1.21+
- Uses AWS SDK for Go v2
- Bubble Tea for terminal UI
- CloudWatch Logs Insights for traffic analysis
- Supports all AWS regions

### Documentation
- Comprehensive README with installation and usage
- Contributing guidelines
- IAM permissions documentation
- Troubleshooting guide
- Architecture overview

## [0.1.0] - TBD

Initial development release.
