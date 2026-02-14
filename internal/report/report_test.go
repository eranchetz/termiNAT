package report

import (
	"strings"
	"testing"

	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/pkg/types"
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
	r := New("us-east-1", "123456789012", 5, nil, stats, cost, nil)
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
	r := New("us-east-1", "123456789012", 5, nil, stats, cost, nil)
	md := r.ToMarkdown()

	if strings.Contains(md, "ECR Traffic Cost") {
		t.Error("ECR cost line should not appear when ECR bytes are 0")
	}
}

func TestMarkdownIncludesNATModeAndECREndpointRemediation(t *testing.T) {
	stats := &analysis.TrafficStats{
		ECRBytes:     1073741824,
		OtherBytes:   1073741824,
		TotalBytes:   2147483648,
		TotalRecords: 20,
	}
	cost := &analysis.CostEstimate{
		Region:               "us-east-1",
		TotalDataGB:          100,
		OtherDataGB:          100,
		CurrentMonthlyCost:   4.5,
		TotalSavingsMonthly:  0,
		NATGatewayPricePerGB: 0.045,
	}
	endpoints := analysis.AnalyzeEndpoints(
		"us-east-1",
		"vpc-123",
		nil,
		[]types.RouteTable{
			{
				ID:      "rtb-1",
				VPCID:   "vpc-123",
				Subnets: []string{"subnet-a", "subnet-b"},
				Routes: []types.Route{
					{DestinationCIDR: "0.0.0.0/0", TargetType: "nat-gateway"},
				},
			},
		},
	)
	nats := []types.NATGateway{
		{ID: "nat-1", VPCID: "vpc-123", SubnetID: "subnet-a", AvailabilityMode: "zonal"},
	}

	md := New("us-east-1", "123456789012", 5, nats, stats, cost, endpoints).ToMarkdown()

	if !strings.Contains(md, "## NAT Gateway Topology") || !strings.Contains(md, "| nat-1 | zonal |") {
		t.Error("markdown report missing NAT topology with gateway mode")
	}
	if !strings.Contains(md, "### ECR Interface Endpoints (Paid)") || !strings.Contains(md, "ecr.api") {
		t.Error("markdown report missing ECR interface endpoint status")
	}
	if !strings.Contains(md, "--vpc-endpoint-type Interface") || !strings.Contains(md, "<security-group-id>") {
		t.Error("markdown report missing ECR remediation command with security group placeholder")
	}
}
