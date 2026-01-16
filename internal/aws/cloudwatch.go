package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// CloudWatchLogsClient wraps AWS CloudWatch Logs API calls
type CloudWatchLogsClient struct {
	client *cloudwatchlogs.Client
}

// NewCloudWatchLogsClient creates a new CloudWatch Logs client wrapper
func NewCloudWatchLogsClient(client *cloudwatchlogs.Client) *CloudWatchLogsClient {
	return &CloudWatchLogsClient{client: client}
}

// CreateLogGroup creates a CloudWatch Logs log group
func (c *CloudWatchLogsClient) CreateLogGroup(ctx context.Context, logGroupName string) error {
	input := &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: &logGroupName,
	}

	_, err := c.client.CreateLogGroup(ctx, input)
	if err != nil {
		// Ignore if already exists
		if _, ok := err.(*types.ResourceAlreadyExistsException); ok {
			return nil
		}
		return fmt.Errorf("failed to create log group: %w", err)
	}

	// Set retention to 1 day
	retentionInput := &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    &logGroupName,
		RetentionInDays: int32Ptr(1),
	}

	_, err = c.client.PutRetentionPolicy(ctx, retentionInput)
	if err != nil {
		return fmt.Errorf("failed to set retention policy: %w", err)
	}

	return nil
}

// DeleteLogGroup deletes a CloudWatch Logs log group
func (c *CloudWatchLogsClient) DeleteLogGroup(ctx context.Context, logGroupName string) error {
	input := &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: &logGroupName,
	}

	_, err := c.client.DeleteLogGroup(ctx, input)
	if err != nil {
		// Ignore if doesn't exist
		if _, ok := err.(*types.ResourceNotFoundException); ok {
			return nil
		}
		return fmt.Errorf("failed to delete log group: %w", err)
	}

	return nil
}

// LogGroupStats contains statistics about a log group
type LogGroupStats struct {
	StoredBytes int64
	LogStreams  int
}

// GetLogGroupStats retrieves statistics about a log group
func (c *CloudWatchLogsClient) GetLogGroupStats(ctx context.Context, logGroupName string) (*LogGroupStats, error) {
	resp, err := c.client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: &logGroupName,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.LogGroups) == 0 {
		return nil, fmt.Errorf("log group not found: %s", logGroupName)
	}

	lg := resp.LogGroups[0]

	streamsResp, err := c.client.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: &logGroupName,
	})
	if err != nil {
		return nil, err
	}

	return &LogGroupStats{
		StoredBytes: *lg.StoredBytes,
		LogStreams:  len(streamsResp.LogStreams),
	}, nil
}

// StartQuery starts a CloudWatch Logs Insights query
func (c *CloudWatchLogsClient) StartQuery(ctx context.Context, logGroupName string, startTime, endTime int64, queryString string) (string, error) {
	input := &cloudwatchlogs.StartQueryInput{
		LogGroupName: &logGroupName,
		StartTime:    &startTime,
		EndTime:      &endTime,
		QueryString:  &queryString,
	}

	result, err := c.client.StartQuery(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to start query: %w", err)
	}

	return *result.QueryId, nil
}

// WaitForQueryResults waits for query to complete and returns results
func (c *CloudWatchLogsClient) WaitForQueryResults(ctx context.Context, queryID string) ([][]types.ResultField, error) {
	for {
		result, err := c.client.GetQueryResults(ctx, &cloudwatchlogs.GetQueryResultsInput{
			QueryId: &queryID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get query results: %w", err)
		}

		switch result.Status {
		case types.QueryStatusComplete:
			return result.Results, nil
		case types.QueryStatusFailed, types.QueryStatusCancelled:
			return nil, fmt.Errorf("query failed with status: %s", result.Status)
		case types.QueryStatusRunning, types.QueryStatusScheduled:
			time.Sleep(2 * time.Second)
		}
	}
}

// QueryFlowLogs queries Flow Logs using CloudWatch Logs Insights
func (c *CloudWatchLogsClient) QueryFlowLogs(ctx context.Context, logGroupName string, startTime, endTime time.Time) (string, error) {
	// Query to extract traffic by destination
	query := `fields @timestamp, pkt_dstaddr, bytes
| filter action = "ACCEPT"
| stats sum(bytes) as total_bytes by pkt_dstaddr
| sort total_bytes desc`

	input := &cloudwatchlogs.StartQueryInput{
		LogGroupName: &logGroupName,
		StartTime:    int64Ptr(startTime.Unix()),
		EndTime:      int64Ptr(endTime.Unix()),
		QueryString:  &query,
	}

	result, err := c.client.StartQuery(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to start query: %w", err)
	}

	return *result.QueryId, nil
}

// GetQueryResults retrieves query results
func (c *CloudWatchLogsClient) GetQueryResults(ctx context.Context, queryID string) ([]map[string]string, bool, error) {
	input := &cloudwatchlogs.GetQueryResultsInput{
		QueryId: &queryID,
	}

	result, err := c.client.GetQueryResults(ctx, input)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get query results: %w", err)
	}

	// Check if query is still running
	if result.Status == types.QueryStatusRunning || result.Status == types.QueryStatusScheduled {
		return nil, false, nil
	}

	if result.Status == types.QueryStatusFailed || result.Status == types.QueryStatusCancelled {
		return nil, true, fmt.Errorf("query failed with status: %s", result.Status)
	}

	// Parse results
	var results []map[string]string
	for _, row := range result.Results {
		record := make(map[string]string)
		for _, field := range row {
			if field.Field != nil && field.Value != nil {
				record[*field.Field] = *field.Value
			}
		}
		results = append(results, record)
	}

	return results, true, nil
}

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}
