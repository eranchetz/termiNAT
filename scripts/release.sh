#!/bin/bash
set -e

VERSION="${1:-v1.0.0}"
REPO="eranchetz/termiNAT"

echo "🚀 Building termiNATor $VERSION"
echo "================================"
echo ""

# Create dist directory
mkdir -p dist

# Build for Linux (amd64)
echo "📦 Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o dist/terminator-linux-amd64 -ldflags "-X main.Version=$VERSION" .
chmod +x dist/terminator-linux-amd64
echo "✓ Linux build complete"

# Build for Linux (arm64)
echo "📦 Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -o dist/terminator-linux-arm64 -ldflags "-X main.Version=$VERSION" .
chmod +x dist/terminator-linux-arm64
echo "✓ Linux ARM64 build complete"

# Build for macOS (amd64 - Intel)
echo "📦 Building for macOS (amd64 - Intel)..."
GOOS=darwin GOARCH=amd64 go build -o dist/terminator-darwin-amd64 -ldflags "-X main.Version=$VERSION" .
chmod +x dist/terminator-darwin-amd64
echo "✓ macOS Intel build complete"

# Build for macOS (arm64 - Apple Silicon)
echo "📦 Building for macOS (arm64 - Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o dist/terminator-darwin-arm64 -ldflags "-X main.Version=$VERSION" .
chmod +x dist/terminator-darwin-arm64
echo "✓ macOS Apple Silicon build complete"

echo ""
echo "✅ All builds complete!"
echo ""
echo "📊 Build artifacts:"
ls -lh dist/

echo ""
echo "🏷️  Creating GitHub release $VERSION..."

# Create release notes
cat > dist/release-notes.md << EOF
# termiNATor $VERSION

Analyze AWS NAT Gateway traffic to identify cost savings by detecting services that should use free VPC Gateway Endpoints.

## Features

- **Quick Scan**: Instant VPC endpoint analysis (no resources created)
- **Deep Dive Scan**: Real-time traffic analysis with VPC Flow Logs
- **Traffic Classification**: Automatically identifies S3, DynamoDB, and other services
- **Cost Estimation**: Calculate potential monthly and annual savings
- **VPC Endpoint Detection**: Identify missing endpoints and routes
- **Automated Cleanup**: Flow Logs stopped automatically after scan

## Installation

### Linux (amd64)
\`\`\`bash
curl -L https://github.com/$REPO/releases/download/$VERSION/terminator-linux-amd64 -o terminator
chmod +x terminator
sudo mv terminator /usr/local/bin/
\`\`\`

### Linux (arm64)
\`\`\`bash
curl -L https://github.com/$REPO/releases/download/$VERSION/terminator-linux-arm64 -o terminator
chmod +x terminator
sudo mv terminator /usr/local/bin/
\`\`\`

### macOS (Intel)
\`\`\`bash
curl -L https://github.com/$REPO/releases/download/$VERSION/terminator-darwin-amd64 -o terminator
chmod +x terminator
sudo mv terminator /usr/local/bin/
\`\`\`

### macOS (Apple Silicon)
\`\`\`bash
curl -L https://github.com/$REPO/releases/download/$VERSION/terminator-darwin-arm64 -o terminator
chmod +x terminator
sudo mv terminator /usr/local/bin/
\`\`\`

## Quick Start

\`\`\`bash
# Configure AWS credentials
export AWS_PROFILE=your-profile
export AWS_REGION=us-east-1

# Run quick scan (instant, no resources created)
terminator scan quick --region us-east-1

# Run deep dive scan (analyzes actual traffic, ~10 minutes)
terminator scan deep --region us-east-1 --duration 5
\`\`\`

## Documentation

- [Usage Guide](https://github.com/$REPO/blob/main/USAGE.md) - Complete guide for production use
- [E2E Testing Guide](https://github.com/$REPO/blob/main/E2E_TESTING.md) - Run automated tests
- [Agent Guide](https://github.com/$REPO/blob/main/AGENTS.md) - For AI agents and contributors

## What's New in $VERSION

- Initial release
- Quick scan for instant VPC endpoint analysis
- Deep dive scan with VPC Flow Logs integration
- Traffic classification for S3, DynamoDB, and other services
- Cost estimation with monthly and annual projections
- VPC endpoint detection and remediation commands
- Automated Flow Logs cleanup
- Comprehensive documentation and E2E testing infrastructure

## Requirements

- AWS CLI configured with valid credentials
- IAM permissions for EC2 read access (Quick Scan)
- Additional permissions for Flow Logs and CloudWatch (Deep Dive Scan)

## Support

- Issues: https://github.com/$REPO/issues
- Documentation: https://github.com/$REPO
EOF

# Create GitHub release
gh release create "$VERSION" \
  --repo "$REPO" \
  --title "termiNATor $VERSION" \
  --notes-file dist/release-notes.md \
  dist/terminator-linux-amd64 \
  dist/terminator-linux-arm64 \
  dist/terminator-darwin-amd64 \
  dist/terminator-darwin-arm64

echo ""
echo "✅ Release $VERSION created successfully!"
echo ""
echo "🔗 View release: https://github.com/$REPO/releases/tag/$VERSION"
