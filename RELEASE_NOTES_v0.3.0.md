# Release Notes - v0.3.0

## 🎉 What's New

### Interface VPC Endpoints Cost Analysis (Issue #3)
termiNATor now analyzes Interface VPC Endpoints and calculates their monthly costs to help you identify unused or underutilized endpoints.

**Features:**
- Automatically detects all Interface endpoints in your VPC
- Calculates monthly cost per endpoint ($0.01/hour per AZ)
- Displays endpoint costs in the final report
- Provides tips for reviewing and removing unused endpoints

**Example Output:**
```
Interface VPC Endpoints:
  • com.amazonaws.us-east-1.ec2 (vpce-xxx): $7.20/month
  • com.amazonaws.us-east-1.lambda (vpce-yyy): $7.20/month
  
Total Interface Endpoint Cost: $14.40/month
```

### AWS Profile Region Auto-Detection (Issue #11)
When using `--profile` without `--region`, termiNATor now automatically reads the region from your AWS profile configuration (`~/.aws/config`).

**Before:**
```bash
terminat scan quick --profile my-profile --region us-east-1
```

**Now:**
```bash
terminat scan quick --profile my-profile
# Using region us-east-1 from profile my-profile
```

## 🐛 Bug Fixes

### Binary Renamed to 'terminat' (Issue #6)
The CLI binary has been renamed from `terminator` to `terminat` to match the brand name and avoid conflicts.

**Changes:**
- Binary name: `terminator` → `terminat`
- Cache directory: `~/.terminator/` → `~/.terminat/`
- Flow Logs prefix: `/aws/vpc/flowlogs/terminator-*` → `/aws/vpc/flowlogs/terminat-*`
- All documentation updated

**Migration:**
```bash
# Old cache will be ignored, new cache will be created at ~/.terminat/
# No action needed - just rebuild and use the new binary name
go build -o terminat .
```

### S3 Bucket Name Collision Fix (Issue #10)
E2E test infrastructure now reuses stack names to avoid S3 bucket name collisions when redeploying.

**Changes:**
- `deploy-test-infra.sh` saves stack name to `test/results/stack-name.txt`
- `cleanup.sh` preserves stack name file for reuse
- Only generates new random suffix if no previous stack exists
- Clear messaging about stack name reuse

## 📚 Documentation

- Standardized binary name to `terminat` across all documentation
- Updated README, USAGE.md, and E2E_TESTING.md
- Updated all code examples and commands

## 🔧 Technical Details

**Commits since v0.2.0:**
- `dfc1871` - fix: Reuse stack name to avoid S3 bucket name collision (Issue #10)
- `b135b76` - feat: Read region from AWS profile config (Issue #11)
- `6acd51a` - docs: Standardize binary name to 'terminat' in all documentation
- `1259479` - fix: Rename binary from 'terminator' to 'terminat' (Issue #6)
- `d3a8508` - feat: Add Interface VPC Endpoints analysis (Issue #3)

## 📦 Installation

```bash
# Build from source
git clone https://github.com/doitintl/terminator.git
cd terminator
git checkout v0.3.0
go build -o terminat .

# Or download pre-built binary from releases page
```

## 🚀 Upgrade Notes

If upgrading from v0.2.0 or earlier:

1. **Binary name changed**: Use `terminat` instead of `terminator`
2. **Cache location changed**: Old cache at `~/.terminator/` will be ignored, new cache created at `~/.terminat/`
3. **No breaking changes**: All commands and flags remain the same

## 🙏 Contributors

Thanks to all contributors who helped with this release!

---

**Full Changelog**: https://github.com/doitintl/terminator/compare/v0.2.0...v0.3.0
