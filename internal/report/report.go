package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/pkg/types"
)

type Report struct {
	GeneratedAt      time.Time                  `json:"generated_at"`
	Region           string                     `json:"region"`
	AccountID        string                     `json:"account_id"`
	ScanDuration     int                        `json:"scan_duration_minutes"`
	NATGateways      []types.NATGateway         `json:"nat_gateways,omitempty"`
	TrafficStats     *analysis.TrafficStats     `json:"traffic_stats,omitempty"`
	CostEstimate     *analysis.CostEstimate     `json:"cost_estimate,omitempty"`
	EndpointAnalysis *analysis.EndpointAnalysis `json:"endpoint_analysis,omitempty"`
}

func New(region, accountID string, duration int, nats []types.NATGateway, stats *analysis.TrafficStats, cost *analysis.CostEstimate, endpoints *analysis.EndpointAnalysis) *Report {
	return &Report{
		GeneratedAt:      time.Now(),
		Region:           region,
		AccountID:        accountID,
		ScanDuration:     duration,
		NATGateways:      nats,
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

func (r *Report) estimateMonthlyECRDataGB() float64 {
	if r.TrafficStats == nil || r.TrafficStats.ECRBytes <= 0 {
		return 0
	}

	if r.CostEstimate != nil && r.CostEstimate.OtherPercentage() > 0 {
		return r.CostEstimate.OtherDataGB * (r.TrafficStats.ECRPercentage() / r.CostEstimate.OtherPercentage())
	}

	// Fallback if cost estimate is unavailable.
	sampleECRGB := float64(r.TrafficStats.ECRBytes) / (1024 * 1024 * 1024)
	return sampleECRGB * (43200.0 / float64(r.ScanDuration))
}

func (r *Report) estimateMonthlyECRNATCost() float64 {
	if r.CostEstimate == nil || r.TrafficStats == nil || r.TrafficStats.ECRBytes <= 0 || r.CostEstimate.OtherPercentage() <= 0 {
		return 0
	}
	return r.CostEstimate.OtherDataGB * r.CostEstimate.NATGatewayPricePerGB * (r.TrafficStats.ECRPercentage() / r.CostEstimate.OtherPercentage())
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

	if len(r.NATGateways) > 0 {
		b.WriteString("## NAT Gateway Topology\n\n")
		b.WriteString("| NAT Gateway | Mode | VPC | Subnet |\n")
		b.WriteString("|-------------|------|-----|--------|\n")
		for _, nat := range r.NATGateways {
			mode := nat.AvailabilityMode
			if mode == "" {
				mode = "zonal"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", nat.ID, mode, nat.VPCID, nat.SubnetID))
		}
		b.WriteString("\n")
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

		b.WriteString("### ECR Interface Endpoints (Paid)\n\n")
		ecrHourlyPerAZ, ecrDataPerGB := r.EndpointAnalysis.GetECRInterfaceEndpointPricing()
		b.WriteString(fmt.Sprintf("> Regional price estimate for `%s`: **$%.4f per AZ-hour** + **$%.4f per GB**\n\n",
			r.Region,
			ecrHourlyPerAZ,
			ecrDataPerGB))
		b.WriteString("> NOTE: These rates come from the scanner's per-region PrivateLink pricing table (defaults to $0.01 per AZ-hour and $0.01 per GB for most regions) and should be treated as estimates; confirm current AWS pricing before provisioning.\n\n")
		b.WriteString("| Service | Status | Endpoint ID |\n")
		b.WriteString("|---------|--------|-------------|\n")
		if r.EndpointAnalysis.ECRAPIEndpoint != nil {
			b.WriteString(fmt.Sprintf("| ECR API (`ecr.api`) | ✅ Configured | %s |\n", r.EndpointAnalysis.ECRAPIEndpoint.ID))
		} else {
			b.WriteString("| ECR API (`ecr.api`) | ⚠️ Missing (optional, paid) | - |\n")
		}
		if r.EndpointAnalysis.ECRDKREndpoint != nil {
			b.WriteString(fmt.Sprintf("| ECR DKR (`ecr.dkr`) | ✅ Configured | %s |\n", r.EndpointAnalysis.ECRDKREndpoint.ID))
		} else {
			b.WriteString("| ECR DKR (`ecr.dkr`) | ⚠️ Missing (optional, paid) | - |\n")
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
		if ecrCost := r.estimateMonthlyECRNATCost(); ecrCost > 0 {
			b.WriteString(fmt.Sprintf("| ECR Traffic Cost over NAT (no free endpoint) | $%.2f/month |\n", ecrCost))
		}
		if r.EndpointAnalysis != nil && r.EndpointAnalysis.HasMissingECRInterfaceEndpoints() {
			monthlyECRGB := r.estimateMonthlyECRDataGB()
			fixed, data, total, azCount, endpointCount := r.EndpointAnalysis.EstimateECRInterfaceEndpointMonthlyCost(monthlyECRGB)
			b.WriteString(fmt.Sprintf("| Estimated ECR Interface Endpoint Cost (%d endpoint(s), %d AZ) | $%.2f/month |\n", endpointCount, azCount, total))
			b.WriteString(fmt.Sprintf("|  └ Fixed hourly component | $%.2f/month |\n", fixed))
			b.WriteString(fmt.Sprintf("|  └ Data processing component (%.2f GB/month) | $%.2f/month |\n", monthlyECRGB, data))
		}
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
			if r.EndpointAnalysis.HasMissingECRInterfaceEndpoints() {
				b.WriteString("> For ECR interface endpoints, replace `<security-group-id>` with a security group that allows HTTPS (443) from your private workloads.\n\n")
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
