package report

import (
	"strings"
	"testing"

	"github.com/doitintl/terminator/internal/analysis"
)

func TestMarkdownContainsECRCost(t *testing.T) {
	stats := &analysis.TrafficStats{
		S3Bytes:      1073741824,
		DynamoBytes:  536870912,
		ECRBytes:     107374182,
		OtherBytes:   214748365,
		TotalBytes:   1932734783,
		TotalRecords: 100,
	}
	cost := &analysis.CostEstimate{
		TotalDataGB:          1.8,
		S3DataGB:             1.0,
		DynamoDataGB:         0.5,
		OtherDataGB:          0.3,
		CurrentMonthlyCost:   0.081,
		S3SavingsMonthly:     0.045,
		DynamoSavingsMonthly: 0.0225,
		TotalSavingsMonthly:  0.0675,
		NATGatewayPricePerGB: 0.045,
	}
	r := New("us-east-1", "123456789012", 5, stats, cost, nil)
	md := r.ToMarkdown()

	if !strings.Contains(md, "ECR Traffic Cost") {
		t.Error("markdown report missing ECR cost line")
	}
	if !strings.Contains(md, "| ECR |") {
		t.Error("markdown report missing ECR traffic row")
	}
}

func TestMarkdownOmitsECRWhenZero(t *testing.T) {
	stats := &analysis.TrafficStats{
		S3Bytes:    1073741824,
		TotalBytes: 1073741824,
	}
	cost := &analysis.CostEstimate{
		TotalDataGB:          1.0,
		S3DataGB:             1.0,
		NATGatewayPricePerGB: 0.045,
	}
	r := New("us-east-1", "123456789012", 5, stats, cost, nil)
	md := r.ToMarkdown()

	if strings.Contains(md, "ECR Traffic Cost") {
		t.Error("ECR cost line should not appear when ECR bytes are 0")
	}
}
