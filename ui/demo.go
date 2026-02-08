package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/pkg/types"
)

// RunDemoScan shows a sample report with realistic fake data, no AWS needed.
func RunDemoScan() error {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	m := &deepScanModel{
		spinner:   s,
		phase:     phaseDone,
		region:    "us-east-1",
		accountID: "123456789012",
		duration:  15,
		startTime: time.Now(),
		nats: []types.NATGateway{
			{ID: "nat-0a1b2c3d4e5f67890", VPCID: "vpc-0abc123def456789"},
		},
		trafficStats: &analysis.TrafficStats{
			S3Bytes:       34359738368, // 32 GB
			DynamoBytes:   10737418240, // 10 GB
			ECRBytes:      3221225472,  // 3 GB
			OtherBytes:    5368709120,  // 5 GB
			TotalBytes:    53687091200, // 50 GB
			S3Records:     42000,
			DynamoRecords: 18500,
			ECRRecords:    3200,
			OtherRecords:  6300,
			TotalRecords:  70000,
		},
		costEstimate: &analysis.CostEstimate{
			Region:               "us-east-1",
			TotalDataGB:          1440,
			S3DataGB:             921.6,
			DynamoDataGB:         288,
			OtherDataGB:          230.4,
			CurrentMonthlyCost:   64.80,
			S3SavingsMonthly:     41.47,
			DynamoSavingsMonthly: 12.96,
			TotalSavingsMonthly:  54.43,
			NATGatewayPricePerGB: 0.045,
		},
		endpointAnalysis: &analysis.EndpointAnalysis{
			VPCID:  "vpc-0abc123def456789",
			Region: "us-east-1",
			S3Endpoint: &types.VPCEndpoint{
				ID:          "vpce-0s3endpoint1234",
				ServiceName: "com.amazonaws.us-east-1.s3",
			},
			DynamoEndpoint:   nil,
			MissingEndpoints: []string{"com.amazonaws.us-east-1.dynamodb"},
			RouteTables: []types.RouteTable{
				{
					ID: "rtb-0abc123def456789",
					Routes: []types.Route{
						{DestinationCIDR: "0.0.0.0/0", TargetType: "nat-gateway", Target: "nat-0a1b2c3d4e5f67890"},
					},
				},
			},
		},
		recommendations: []analysis.Recommendation{
			{
				Priority:    "HIGH",
				Title:       "Create DynamoDB Gateway Endpoint",
				Description: "DynamoDB traffic is routing through NAT Gateway. A free Gateway Endpoint would eliminate $12.96/month in data processing charges.",
				Savings:     fmt.Sprintf("$%.2f/month ($%.2f/year)", 12.96, 155.52),
			},
		},
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
