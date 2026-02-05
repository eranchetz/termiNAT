package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/doitintl/terminator/internal/analysis"
)

type Report struct {
	GeneratedAt      time.Time                  `json:"generated_at"`
	Region           string                     `json:"region"`
	AccountID        string                     `json:"account_id"`
	ScanDuration     int                        `json:"scan_duration_minutes"`
	TrafficStats     *analysis.TrafficStats     `json:"traffic_stats,omitempty"`
	CostEstimate     *analysis.CostEstimate     `json:"cost_estimate,omitempty"`
	EndpointAnalysis *analysis.EndpointAnalysis `json:"endpoint_analysis,omitempty"`
}

func New(region, accountID string, duration int, stats *analysis.TrafficStats, cost *analysis.CostEstimate, endpoints *analysis.EndpointAnalysis) *Report {
	return &Report{
		GeneratedAt:      time.Now(),
		Region:           region,
		AccountID:        accountID,
		ScanDuration:     duration,
		TrafficStats:     stats,
		CostEstimate:     cost,
		EndpointAnalysis: endpoints,
	}
}

func (r *Report) SaveJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (r *Report) SaveMarkdown(path string) error {
	return os.WriteFile(path, []byte(r.ToMarkdown()), 0644)
}

func (r *Report) ToMarkdown() string {
	var b strings.Builder

	b.WriteString("# termiNATor Deep Dive Report\n\n")
	b.WriteString(fmt.Sprintf("**Generated:** %s  \n", r.GeneratedAt.Format(time.RFC1123)))
	b.WriteString(fmt.Sprintf("**Region:** %s  \n", r.Region))
	b.WriteString(fmt.Sprintf("**Account:** %s  \n", r.AccountID))
	b.WriteString(fmt.Sprintf("**Sample Duration:** %d minutes\n\n", r.ScanDuration))

	// Executive Summary
	if r.CostEstimate != nil && r.CostEstimate.TotalSavingsMonthly > 0 {
		b.WriteString("## 💰 Executive Summary\n\n")
		b.WriteString(fmt.Sprintf("**Potential Monthly Savings: $%.2f** ($%.2f/year)\n\n",
			r.CostEstimate.TotalSavingsMonthly, r.CostEstimate.TotalSavingsMonthly*12))
		b.WriteString("> ⚠️ Estimates projected from traffic sample. Actual savings depend on real traffic patterns.\n\n")
	}

	// VPC Endpoint Status
	if r.EndpointAnalysis != nil {
		b.WriteString("## VPC Endpoint Configuration\n\n")
		b.WriteString(fmt.Sprintf("**VPC:** %s\n\n", r.EndpointAnalysis.VPCID))

		b.WriteString("### Gateway Endpoints\n\n")
		b.WriteString("| Service | Status | Endpoint ID |\n")
		b.WriteString("|---------|--------|-------------|\n")
		if r.EndpointAnalysis.S3Endpoint != nil {
			b.WriteString(fmt.Sprintf("| S3 | ✅ Configured | %s |\n", r.EndpointAnalysis.S3Endpoint.ID))
		} else {
			b.WriteString("| S3 | ❌ Missing | - |\n")
		}
		if r.EndpointAnalysis.DynamoEndpoint != nil {
			b.WriteString(fmt.Sprintf("| DynamoDB | ✅ Configured | %s |\n", r.EndpointAnalysis.DynamoEndpoint.ID))
		} else {
			b.WriteString("| DynamoDB | ❌ Missing | - |\n")
		}
		b.WriteString("\n")

		if len(r.EndpointAnalysis.MissingRoutes) > 0 {
			b.WriteString("### Missing Route Table Associations\n\n")
			for _, mr := range r.EndpointAnalysis.MissingRoutes {
				b.WriteString(fmt.Sprintf("- `%s`: missing %s route\n", mr.RouteTableID, mr.Service))
			}
			b.WriteString("\n")
		}
	}

	// Traffic Analysis
	if r.TrafficStats != nil && r.TrafficStats.TotalRecords > 0 {
		b.WriteString("## Collected Traffic Sample\n\n")
		b.WriteString(fmt.Sprintf("**Total:** %d records, %.2f GB\n\n",
			r.TrafficStats.TotalRecords, float64(r.TrafficStats.TotalBytes)/(1024*1024*1024)))

		b.WriteString("| Service | Data (GB) | Percentage |\n")
		b.WriteString("|---------|-----------|------------|\n")
		b.WriteString(fmt.Sprintf("| S3 | %.2f | %.1f%% |\n",
			float64(r.TrafficStats.S3Bytes)/(1024*1024*1024), r.TrafficStats.S3Percentage()))
		b.WriteString(fmt.Sprintf("| DynamoDB | %.2f | %.1f%% |\n",
			float64(r.TrafficStats.DynamoBytes)/(1024*1024*1024), r.TrafficStats.DynamoPercentage()))
		b.WriteString(fmt.Sprintf("| ECR | %.2f | %.1f%% |\n",
			float64(r.TrafficStats.ECRBytes)/(1024*1024*1024), r.TrafficStats.ECRPercentage()))
		b.WriteString(fmt.Sprintf("| Other | %.2f | %.1f%% |\n\n",
			float64(r.TrafficStats.OtherBytes)/(1024*1024*1024), r.TrafficStats.OtherPercentage()))
	}

	// Cost Estimate
	if r.CostEstimate != nil {
		b.WriteString("## Cost Estimate\n\n")
		b.WriteString(fmt.Sprintf("> Projected from %d-minute sample to monthly estimate\n\n", r.ScanDuration))
		b.WriteString(fmt.Sprintf("**NAT Gateway Rate:** $%.4f per GB\n\n", r.CostEstimate.NATGatewayPricePerGB))

		b.WriteString("| Metric | Amount |\n")
		b.WriteString("|--------|--------|\n")
		b.WriteString(fmt.Sprintf("| Current NAT Gateway Cost | $%.2f/month |\n", r.CostEstimate.CurrentMonthlyCost))
		b.WriteString(fmt.Sprintf("| S3 Endpoint Savings | $%.2f/month |\n", r.CostEstimate.S3SavingsMonthly))
		b.WriteString(fmt.Sprintf("| DynamoDB Endpoint Savings | $%.2f/month |\n", r.CostEstimate.DynamoSavingsMonthly))
		b.WriteString(fmt.Sprintf("| **Total Potential Savings** | **$%.2f/month** |\n\n", r.CostEstimate.TotalSavingsMonthly))
	}

	// Remediation
	if r.EndpointAnalysis != nil && r.EndpointAnalysis.HasIssues() {
		b.WriteString("## Remediation Steps\n\n")

		if cmds := r.EndpointAnalysis.GetCreateEndpointCommands(); len(cmds) > 0 {
			b.WriteString("### Create Missing VPC Endpoints\n\n")
			for _, cmd := range cmds {
				b.WriteString(fmt.Sprintf("```bash\n%s\n```\n\n", cmd))
			}
		}

		if cmds := r.EndpointAnalysis.GetAddRouteCommands(); len(cmds) > 0 {
			b.WriteString("### Add Missing Route Table Associations\n\n")
			for _, cmd := range cmds {
				b.WriteString(fmt.Sprintf("```bash\n%s\n```\n\n", cmd))
			}
		}
	}

	b.WriteString("---\n")
	b.WriteString("*Generated by [termiNATor](https://github.com/doitintl/terminator)*\n")

	return b.String()
}
