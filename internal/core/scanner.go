package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
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
	iamClient *iam.Client
	cwClient  *cloudwatch.Client
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
		iamClient: iam.NewFromConfig(cfg),
		cwClient:  cloudwatch.NewFromConfig(cfg),
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

// ValidateFlowLogsRole checks if the IAM role for Flow Logs exists
func (s *Scanner) ValidateFlowLogsRole(ctx context.Context, roleARN string) error {
	// Extract role name from ARN (arn:aws:iam::123456789012:role/RoleName)
	parts := strings.Split(roleARN, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid role ARN format: %s", roleARN)
	}
	roleName := parts[len(parts)-1]

	// Check if role exists
	roleResp, err := s.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &roleName,
	})
	if err != nil {
		return fmt.Errorf("IAM role '%s' not found. Run: ./scripts/setup-flowlogs-role.sh", roleName)
	}

	// Verify trust policy allows vpc-flow-logs.amazonaws.com
	trustPolicy := *roleResp.Role.AssumeRolePolicyDocument
	if !strings.Contains(trustPolicy, "vpc-flow-logs.amazonaws.com") {
		return fmt.Errorf("IAM role '%s' trust policy does not allow vpc-flow-logs.amazonaws.com. Run: ./scripts/setup-flowlogs-role.sh", roleName)
	}

	// Check for CloudWatch Logs permissions (both attached and inline policies)
	hasCloudWatchPolicy := false

	// Check attached policies
	policiesResp, err := s.iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return fmt.Errorf("failed to list role policies: %w", err)
	}

	for _, policy := range policiesResp.AttachedPolicies {
		if strings.Contains(*policy.PolicyName, "CloudWatchLogs") ||
			strings.Contains(*policy.PolicyName, "FlowLogs") {
			hasCloudWatchPolicy = true
			break
		}
	}

	// Check inline policies if not found in attached
	if !hasCloudWatchPolicy {
		inlinePoliciesResp, err := s.iamClient.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
			RoleName: &roleName,
		})
		if err != nil {
			return fmt.Errorf("failed to list inline policies: %w", err)
		}

		for _, policyName := range inlinePoliciesResp.PolicyNames {
			if strings.Contains(policyName, "CloudWatchLogs") ||
				strings.Contains(policyName, "FlowLogs") {
				hasCloudWatchPolicy = true
				break
			}
		}
	}

	if !hasCloudWatchPolicy {
		return fmt.Errorf("IAM role '%s' missing CloudWatch Logs permissions. Run: ./scripts/setup-flowlogs-role.sh", roleName)
	}

	return nil
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

// AnalyzeTraffic analyzes Flow Logs and classifies traffic using aggregated CloudWatch query
func (s *Scanner) AnalyzeTraffic(ctx context.Context, logGroupName string, startTime, endTime int64) (*analysis.TrafficStats, error) {
	// Use aggregated query to avoid OOM on large datasets
	query := `fields @timestamp, pkt_dstaddr, bytes
| filter action = "ACCEPT"
| stats sum(bytes) as total_bytes by pkt_dstaddr
| sort total_bytes desc`

	queryID, err := s.cwlClient.StartQuery(ctx, logGroupName, startTime, endTime, query)
	if err != nil {
		return nil, fmt.Errorf("failed to start query: %w", err)
	}

	// Wait for query to complete and get results
	results, err := s.cwlClient.WaitForQueryResults(ctx, queryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get query results: %w", err)
	}

	// Diagnostic: check if query returned any results
	if len(results) == 0 {
		return nil, fmt.Errorf("no Flow Logs data found - query returned 0 results. This could mean: (1) No traffic during collection period, (2) Flow Logs not delivering data yet, or (3) All traffic was to private IPs (filtered out)")
	}

	// Process aggregated results
	analyzer, err := analysis.NewTrafficAnalyzer()
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer: %w", err)
	}

	return analyzer.AnalyzeAggregatedResults(results)
}

// CalculateCosts calculates cost estimates based on traffic analysis
func (s *Scanner) CalculateCosts(stats *analysis.TrafficStats, collectionMinutes int) *analysis.CostEstimate {
	return analysis.CalculateCosts(s.region, stats, collectionMinutes)
}

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

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }
