# Changelog

All notable changes to termiNATor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
