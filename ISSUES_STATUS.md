# GitHub Issues Status

Last updated: 2026-02-01

## ✅ Closed Issues (11)

### Issue #1 - Regional NAT Gateway Support
**Status:** CLOSED  
**Implemented in:** v0.2.0  
**Features:**
- Detect NAT Gateway availability mode (regional vs zonal)
- Recommend Regional NAT Gateway for multi-AZ setups
- Show cost benefits and implementation steps

### Issue #2 - Regional Destinations and ECR Detection
**Status:** CLOSED (2026-02-01)  
**Implemented in:** v0.2.0  
**Features:**
- ECR traffic detection using EC2 IP ranges
- Display ECR traffic in summary and detailed views
- E2E tests include ECR traffic generation (~4-10%)

### Issue #3 - Interface VPC Endpoints Analysis
**Status:** CLOSED  
**Implemented in:** v0.3.0  
**Features:**
- Detect all Interface endpoints in VPC
- Calculate monthly cost per endpoint ($0.01/hour per AZ)
- Display costs in final report
- Tips for reviewing unused endpoints

### Issue #4 - Elastic IP Release Warning
**Status:** CLOSED  
**Resolution:** Documentation added to warn users about Elastic IP release when terminating NAT Gateway

### Issue #5 - AWS Profile Parameter
**Status:** CLOSED  
**Implemented in:** v0.2.0  
**Features:**
- Added `--profile` flag to scan commands
- Falls back to AWS_PROFILE environment variable
- Comprehensive authentication help guide

### Issue #6 - Binary Name Standardization
**Status:** CLOSED  
**Implemented in:** v0.3.0  
**Changes:**
- Renamed binary from `terminator` to `terminat`
- Updated cache directory to `~/.terminat/`
- Updated Flow Logs prefix to `/aws/vpc/flowlogs/terminat-*`

### Issue #7 - Version Flag
**Status:** CLOSED  
**Implemented in:** v0.1.0  
**Features:**
- Added `--version` flag to CLI

### Issue #8 - AWS_REGION Environment Variable
**Status:** CLOSED  
**Implemented in:** v0.2.0  
**Features:**
- Made `--region` flag optional when AWS_REGION is set
- Region resolution: flag → AWS_REGION env var

### Issue #9 - Test Scripts STACK_NAME Mismatch
**Status:** CLOSED  
**Implemented in:** v0.2.0  
**Fix:**
- deploy-test-infra.sh saves stack name to test/results/stack-name.txt
- All test scripts read from saved file

### Issue #10 - S3 Bucket Name Collision
**Status:** CLOSED  
**Implemented in:** v0.3.0  
**Fix:**
- Reuse stack name from previous deploy
- cleanup.sh preserves stack-name.txt
- Only generates new random suffix if no previous stack exists

### Issue #11 - Profile Region Auto-Detection
**Status:** CLOSED  
**Implemented in:** v0.3.0  
**Features:**
- Automatically read region from AWS profile config
- Shows message: "Using region X from profile Y"

## 🔄 Open Issues (2)

### Issue #12 - Multi-AZ Traffic Detection and Optimization
**Status:** OPEN  
**Created:** 2026-02-01  
**Priority:** High  
**Description:**
Analyze Flow Logs to detect cross-AZ traffic patterns and recommend optimization.

**Planned Features:**
- Detect cross-AZ traffic (source subnet AZ != NAT Gateway AZ)
- Calculate cross-AZ data transfer costs ($0.01/GB)
- Recommend multiple NAT Gateways (one per AZ)
- Recommend Regional NAT Gateway for simplified multi-AZ setup
- Show cost comparison between options

**Implementation Plan:**
- Analyze Flow Logs for cross-AZ patterns
- Add cost calculations for cross-AZ transfer
- Update recommendations in analyzer.go
- Display in deep_scan.go report

### Issue #13 - Flow Logs Cost Estimation
**Status:** OPEN  
**Created:** 2026-02-01  
**Priority:** Medium  
**Description:**
Estimate Flow Logs ingestion costs before creating them and warn users.

**Planned Features:**
- Query CloudWatch metrics (BytesOutToDestination) from past 24h
- Calculate estimated Flow Logs ingestion cost (~$0.50/GB)
- Show warning with cost estimate before scan
- Add to approval prompt in deep_scan.go
- Allow user to cancel before creating Flow Logs

**Implementation Plan:**
- Add CloudWatch metrics query for NAT Gateway traffic
- Calculate cost estimate based on duration
- Update approval prompt with cost warning
- Handle user cancellation gracefully

## 📊 Statistics

- **Total Issues:** 13
- **Closed:** 11 (85%)
- **Open:** 2 (15%)
- **Latest Release:** v0.3.0
- **Issues Closed in v0.3.0:** 4 (#3, #6, #10, #11)

## 🎯 Next Steps

1. **Issue #12** - Multi-AZ traffic detection (high priority)
2. **Issue #13** - Flow Logs cost estimation (medium priority)
3. Consider additional features from roadmap:
   - Historical cost analysis from CloudWatch metrics
   - Automated VPC endpoint creation
   - Multi-account scanning
   - JSON/CSV export for reporting

## 📝 Notes

- All closed issues have been implemented and tested
- E2E tests cover all major features
- Documentation updated for all changes
- Binary name standardized to `terminat` across all docs and code
