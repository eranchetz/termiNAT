package core

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/internal/aws"
	"github.com/doitintl/terminator/pkg/types"
)

// Scanner orchestrates the NAT Gateway analysis
type Scanner struct {
	region    string
	accountID string
	ec2Client *aws.EC2Client
	cwlClient *aws.CloudWatchLogsClient
}

// NewScanner creates a new scanner instance
func NewScanner(ctx context.Context, region, profile string) (*Scanner, error) {
	// Build config options with fast IMDS timeout
	configOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithEC2IMDSClientEnableState(imds.ClientDisabled), // Disable IMDS for fast failure on non-EC2
	}

	// Add profile if specified
	if profile != "" {
		configOpts = append(configOpts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Validate credentials by calling STS - this fails fast if not authenticated
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	accountID := ""
	if identity.Account != nil {
		accountID = *identity.Account
	}

	return &Scanner{
		region:    region,
		accountID: accountID,
		ec2Client: aws.NewEC2Client(ec2.NewFromConfig(cfg)),
		cwlClient: aws.NewCloudWatchLogsClient(cloudwatchlogs.NewFromConfig(cfg)),
	}, nil
}

// GetAccountID returns the AWS account ID
func (s *Scanner) GetAccountID() string {
	return s.accountID
}

// GetRegion returns the AWS region
func (s *Scanner) GetRegion() string {
	return s.region
}

// DiscoverNATGateways finds all NAT Gateways in the region
func (s *Scanner) DiscoverNATGateways(ctx context.Context) ([]types.NATGateway, error) {
	return s.ec2Client.DiscoverNATGateways(ctx)
}

// DiscoverVPCEndpoints finds all VPC endpoints
func (s *Scanner) DiscoverVPCEndpoints(ctx context.Context, vpcID string) ([]types.VPCEndpoint, error) {
	return s.ec2Client.DiscoverVPCEndpoints(ctx, vpcID)
}

// DiscoverRouteTables finds route tables for a VPC
func (s *Scanner) DiscoverRouteTables(ctx context.Context, vpcID string) ([]types.RouteTable, error) {
	return s.ec2Client.DiscoverRouteTables(ctx, vpcID)
}

// AnalyzeVPCEndpoints analyzes VPC endpoint configuration for a VPC
func (s *Scanner) AnalyzeVPCEndpoints(ctx context.Context, vpcID string) (*analysis.EndpointAnalysis, error) {
	endpoints, err := s.DiscoverVPCEndpoints(ctx, vpcID)
	if err != nil {
		return nil, fmt.Errorf("failed to discover VPC endpoints: %w", err)
	}

	routeTables, err := s.DiscoverRouteTables(ctx, vpcID)
	if err != nil {
		return nil, fmt.Errorf("failed to discover route tables: %w", err)
	}

	return analysis.AnalyzeEndpoints(s.region, vpcID, endpoints, routeTables), nil
}

// CreateFlowLogs creates Flow Logs for a NAT Gateway
func (s *Scanner) CreateFlowLogs(ctx context.Context, nat types.NATGateway, logGroupName string, deliveryRoleArn string, runID string) (string, error) {
	return s.ec2Client.CreateFlowLogs(ctx, nat, logGroupName, deliveryRoleArn, runID)
}

// DeleteFlowLogs deletes Flow Logs
func (s *Scanner) DeleteFlowLogs(ctx context.Context, flowLogIDs []string) error {
	return s.ec2Client.DeleteFlowLogs(ctx, flowLogIDs)
}

// CreateLogGroup creates a CloudWatch Logs log group
func (s *Scanner) CreateLogGroup(ctx context.Context, logGroupName string) error {
	return s.cwlClient.CreateLogGroup(ctx, logGroupName)
}

// DeleteLogGroup deletes a CloudWatch Logs log group
func (s *Scanner) DeleteLogGroup(ctx context.Context, logGroupName string) error {
	return s.cwlClient.DeleteLogGroup(ctx, logGroupName)
}

// GetLogGroupStats retrieves statistics about a log group
func (s *Scanner) GetLogGroupStats(ctx context.Context, logGroupName string) (*aws.LogGroupStats, error) {
	return s.cwlClient.GetLogGroupStats(ctx, logGroupName)
}

// CheckActiveFlowLogs checks if any Flow Logs are actively using a log group
func (s *Scanner) CheckActiveFlowLogs(ctx context.Context, logGroupName string) ([]string, error) {
	return s.ec2Client.CheckActiveFlowLogs(ctx, logGroupName)
}

// AnalyzeTraffic analyzes Flow Logs and classifies traffic
func (s *Scanner) AnalyzeTraffic(ctx context.Context, logGroupName string, startTime, endTime int64) (*analysis.TrafficStats, error) {
	// Query Flow Logs
	queryID, err := s.cwlClient.StartQuery(ctx, logGroupName, startTime, endTime, "fields @message | filter @message like /ACCEPT/")
	if err != nil {
		return nil, fmt.Errorf("failed to start query: %w", err)
	}

	// Wait for query to complete and get results
	results, err := s.cwlClient.WaitForQueryResults(ctx, queryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get query results: %w", err)
	}

	// Extract log lines
	var logLines []string
	for _, result := range results {
		for _, field := range result {
			if *field.Field == "@message" {
				logLines = append(logLines, *field.Value)
			}
		}
	}

	// Analyze traffic
	analyzer, err := analysis.NewTrafficAnalyzer()
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer: %w", err)
	}

	return analyzer.AnalyzeFlowLogs(logLines)
}

// CalculateCosts calculates cost estimates based on traffic analysis
func (s *Scanner) CalculateCosts(stats *analysis.TrafficStats, collectionMinutes int) *analysis.CostEstimate {
	return analysis.CalculateCosts(s.region, stats, collectionMinutes)
}
