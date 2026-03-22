package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/pkg/types"
)

type Report struct {
	GeneratedAt      time.Time                             `json:"generated_at"`
	Region           string                                `json:"region"`
	AccountID        string                                `json:"account_id"`
	ScanDuration     int                                   `json:"scan_duration_minutes"`
	NATGateways      []types.NATGateway                    `json:"nat_gateways,omitempty"`
	TrafficStats     *analysis.TrafficStats                `json:"traffic_stats,omitempty"`
	CostEstimate     *analysis.CostEstimate                `json:"cost_estimate,omitempty"`
	EndpointAnalysis *analysis.EndpointAnalysis            `json:"endpoint_analysis,omitempty"`
	EndpointAnalyses map[string]*analysis.EndpointAnalysis `json:"endpoint_analyses,omitempty"`
	AllFindings      []types.Finding                       `json:"all_findings,omitempty"`
	Recommendations  []analysis.Recommendation             `json:"recommendations,omitempty"`
	DeepScannedVPC   string                                `json:"deep_scanned_vpc,omitempty"`
	SelectedVPCIDs   []string                              `json:"selected_vpc_ids,omitempty"`
	LogGroupName     string                                `json:"log_group_name,omitempty"`
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

// NewDetailed creates a report with the full result payload used by the scan UIs.
func NewDetailed(region, accountID string, duration int, nats []types.NATGateway, stats *analysis.TrafficStats, cost *analysis.CostEstimate, primaryEndpoint *analysis.EndpointAnalysis, endpointAnalyses map[string]*analysis.EndpointAnalysis, findings []types.Finding, recommendations []analysis.Recommendation, selectedVPCIDs []string, logGroupName string) *Report {
	r := New(region, accountID, duration, nats, stats, cost, primaryEndpoint)
	r.EndpointAnalysis = primaryEndpoint
	r.EndpointAnalyses = cloneEndpointAnalyses(endpointAnalyses)
	r.AllFindings = findings
	r.Recommendations = recommendations
	r.SelectedVPCIDs = normalizeIDs(selectedVPCIDs)
	if len(r.SelectedVPCIDs) == 0 && primaryEndpoint != nil && primaryEndpoint.VPCID != "" {
		r.SelectedVPCIDs = []string{primaryEndpoint.VPCID}
	}
	if len(r.SelectedVPCIDs) > 0 {
		r.DeepScannedVPC = r.SelectedVPCIDs[0]
	} else if primaryEndpoint != nil {
		r.DeepScannedVPC = primaryEndpoint.VPCID
	}
	r.LogGroupName = logGroupName
	return r
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

func normalizeIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	ids := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	return ids
}

func cloneEndpointAnalyses(input map[string]*analysis.EndpointAnalysis) map[string]*analysis.EndpointAnalysis {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]*analysis.EndpointAnalysis, len(input))
	for vpcID, analysis := range input {
		if analysis == nil {
			continue
		}
		out[vpcID] = analysis
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func indentText(text, indent string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// ViewInterfaceEndpointCost is the template-friendly view of an interface endpoint cost.
type ViewInterfaceEndpointCost struct {
	VPCID       string
	ServiceName string
	DisplayName string
	MonthlyCost float64
}

// ViewSourceIP is the template-friendly view of a source IP breakdown.
type ViewSourceIP struct {
	IP      string
	GB      float64
	Records int
}

// VPCNATGroup keeps NAT gateways ordered by VPC for stable rendering.
type VPCNATGroup struct {
	VPCID            string
	NATs             []types.NATGateway
	EndpointAnalysis *analysis.EndpointAnalysis
	Selected         bool
}

// ViewData is the canonical presentation model for deep-scan output.
type ViewData struct {
	VPCNATs          map[string][]types.NATGateway
	OrderedVPCNATs   []VPCNATGroup
	DeepScannedVPC   string
	SelectedVPCIDs   []string
	AllFindings      []types.Finding
	EndpointAnalysis *analysis.EndpointAnalysis
	TrafficStats     *analysis.TrafficStats
	CostEstimate     *analysis.CostEstimate
	Recommendations  []analysis.Recommendation
	Duration         int
	LogGroupName     string

	HasTraffic                         bool
	HasRemediation                     bool
	HasInterfaceEndpoints              bool
	MissingRoutes                      []analysis.MissingRoute
	InterfaceEndpointCosts             []ViewInterfaceEndpointCost
	TotalInterfaceEndpointCost         float64
	TotalTrafficGB                     float64
	S3GB, DynamoGB, ECRGB, OtherGB     float64
	S3Pct, DynamoPct, ECRPct, OtherPct float64
	TopSourceIPs                       []ViewSourceIP
	MoreSources                        int
	ECRCost                            float64
	AnnualSavings                      float64
	CreateEndpointCmds                 []string
	AddRouteCmds                       []string
}

// ViewData converts the report into a presentation model used by the UIs.
func (r *Report) ViewData() ViewData {
	data := ViewData{
		VPCNATs:          make(map[string][]types.NATGateway),
		DeepScannedVPC:   r.DeepScannedVPC,
		SelectedVPCIDs:   normalizeIDs(r.SelectedVPCIDs),
		AllFindings:      r.AllFindings,
		EndpointAnalysis: r.EndpointAnalysis,
		TrafficStats:     r.TrafficStats,
		CostEstimate:     r.CostEstimate,
		Recommendations:  r.Recommendations,
		Duration:         r.ScanDuration,
		LogGroupName:     r.LogGroupName,
	}

	for _, nat := range r.NATGateways {
		data.VPCNATs[nat.VPCID] = append(data.VPCNATs[nat.VPCID], nat)
	}
	if len(data.SelectedVPCIDs) == 0 {
		if data.DeepScannedVPC != "" {
			data.SelectedVPCIDs = []string{data.DeepScannedVPC}
		} else if data.EndpointAnalysis != nil && data.EndpointAnalysis.VPCID != "" {
			data.SelectedVPCIDs = []string{data.EndpointAnalysis.VPCID}
		}
	}
	selectedSet := make(map[string]struct{}, len(data.SelectedVPCIDs))
	for _, vpcID := range data.SelectedVPCIDs {
		selectedSet[vpcID] = struct{}{}
	}
	if len(data.VPCNATs) > 0 {
		vpcIDs := make([]string, 0, len(data.VPCNATs))
		for vpcID := range data.VPCNATs {
			vpcIDs = append(vpcIDs, vpcID)
		}
		sort.Strings(vpcIDs)
		if data.EndpointAnalysis == nil && len(r.EndpointAnalyses) > 0 {
			if primary, ok := r.EndpointAnalyses[data.DeepScannedVPC]; ok {
				data.EndpointAnalysis = primary
			}
		}
		for _, vpcID := range vpcIDs {
			analysis := r.EndpointAnalyses[vpcID]
			if analysis == nil && r.EndpointAnalysis != nil && r.EndpointAnalysis.VPCID == vpcID {
				analysis = r.EndpointAnalysis
			}
			if analysis == nil && data.EndpointAnalysis != nil && data.EndpointAnalysis.VPCID == vpcID {
				analysis = data.EndpointAnalysis
			}
			data.OrderedVPCNATs = append(data.OrderedVPCNATs, VPCNATGroup{
				VPCID:            vpcID,
				NATs:             data.VPCNATs[vpcID],
				EndpointAnalysis: analysis,
				Selected:         len(selectedSet) == 0 || containsString(data.SelectedVPCIDs, vpcID),
			})
		}
	}

	var aggregatedAnalyses []*analysis.EndpointAnalysis
	for _, group := range data.OrderedVPCNATs {
		if group.EndpointAnalysis == nil {
			continue
		}
		aggregatedAnalyses = append(aggregatedAnalyses, group.EndpointAnalysis)
		if group.EndpointAnalysis.HasIssues() {
			data.HasRemediation = true
			data.CreateEndpointCmds = append(data.CreateEndpointCmds, group.EndpointAnalysis.GetCreateEndpointCommands()...)
			data.AddRouteCmds = append(data.AddRouteCmds, group.EndpointAnalysis.GetAddRouteCommands()...)
		}
		if group.EndpointAnalysis.HasInterfaceEndpoints() {
			data.HasInterfaceEndpoints = true
			data.TotalInterfaceEndpointCost += group.EndpointAnalysis.GetTotalInterfaceEndpointMonthlyCost()
			for _, c := range group.EndpointAnalysis.GetInterfaceEndpointCosts() {
				name := c.Endpoint.Tags["Name"]
				if name == "" {
					name = c.Endpoint.ID
				}
				data.InterfaceEndpointCosts = append(data.InterfaceEndpointCosts, ViewInterfaceEndpointCost{
					VPCID:       group.VPCID,
					ServiceName: c.ServiceName,
					DisplayName: name,
					MonthlyCost: c.MonthlyCost,
				})
			}
		}
	}
	if len(aggregatedAnalyses) > 0 {
		data.EndpointAnalysis = aggregatedAnalyses[0]
	}
	if data.EndpointAnalysis != nil {
		for _, analysis := range aggregatedAnalyses {
			data.MissingRoutes = append(data.MissingRoutes, analysis.MissingRoutes...)
		}
	}

	if r.TrafficStats != nil && r.TrafficStats.TotalRecords > 0 {
		data.HasTraffic = true
		data.TotalTrafficGB = float64(r.TrafficStats.TotalBytes) / (1024 * 1024 * 1024)
		data.S3GB = float64(r.TrafficStats.S3Bytes) / (1024 * 1024 * 1024)
		data.DynamoGB = float64(r.TrafficStats.DynamoBytes) / (1024 * 1024 * 1024)
		data.ECRGB = float64(r.TrafficStats.ECRBytes) / (1024 * 1024 * 1024)
		data.OtherGB = float64(r.TrafficStats.OtherBytes) / (1024 * 1024 * 1024)
		data.S3Pct = r.TrafficStats.S3Percentage()
		data.DynamoPct = r.TrafficStats.DynamoPercentage()
		data.ECRPct = r.TrafficStats.ECRPercentage()
		data.OtherPct = r.TrafficStats.OtherPercentage()

		top := r.TrafficStats.TopSourceIPs(10)
		for _, e := range top {
			data.TopSourceIPs = append(data.TopSourceIPs, ViewSourceIP{
				IP:      e.IP,
				GB:      float64(e.Stats.Bytes) / (1024 * 1024 * 1024),
				Records: e.Stats.Records,
			})
		}
		if len(r.TrafficStats.SourceIPs) > 10 {
			data.MoreSources = len(r.TrafficStats.SourceIPs) - 10
		}
	}

	if r.CostEstimate != nil {
		data.AnnualSavings = r.CostEstimate.TotalSavingsMonthly * 12
		if r.TrafficStats != nil && r.TrafficStats.ECRBytes > 0 && r.CostEstimate.OtherPercentage() > 0 {
			data.ECRCost = r.CostEstimate.OtherDataGB * r.CostEstimate.NATGatewayPricePerGB * (r.TrafficStats.ECRPercentage() / r.CostEstimate.OtherPercentage())
		}
	}

	return data
}

// SummaryText returns a plain-text summary that is shared by the stream UI and the CLI demo.
func (r *Report) SummaryText() string {
	data := r.ViewData()
	var b strings.Builder

	b.WriteString("========== DEEP SCAN REPORT ==========\n")
	if len(data.SelectedVPCIDs) > 0 {
		b.WriteString(fmt.Sprintf("Selected VPCs: %s\n", strings.Join(data.SelectedVPCIDs, ", ")))
	} else if r.DeepScannedVPC != "" {
		b.WriteString(fmt.Sprintf("Deep scanned VPC: %s\n", r.DeepScannedVPC))
	}
	if r.LogGroupName != "" {
		b.WriteString(fmt.Sprintf("Log group: %s\n", r.LogGroupName))
	}
	b.WriteString("\nNAT Gateways\n")
	if len(data.OrderedVPCNATs) == 0 {
		b.WriteString("  - none discovered\n")
	} else {
		for _, group := range data.OrderedVPCNATs {
			if group.Selected {
				b.WriteString(fmt.Sprintf("  VPC %s [selected]\n", group.VPCID))
			} else {
				b.WriteString(fmt.Sprintf("  VPC %s [config check only]\n", group.VPCID))
			}
			for _, nat := range group.NATs {
				mode := nat.AvailabilityMode
				if mode == "" {
					mode = "zonal"
				}
				b.WriteString(fmt.Sprintf("    - %s (%s)\n", nat.ID, mode))
			}
		}
	}

	b.WriteString("\nEndpoint Findings\n")
	if len(data.OrderedVPCNATs) == 0 {
		b.WriteString("  - no VPCs selected\n")
	} else {
		for _, group := range data.OrderedVPCNATs {
			if group.EndpointAnalysis == nil {
				continue
			}
			b.WriteString(fmt.Sprintf("  VPC %s\n", group.VPCID))
			b.WriteString(indentText(group.EndpointAnalysis.String(), "    "))
		}
	}

	if len(data.AllFindings) > 0 {
		b.WriteString("\nFindings\n")
		for _, finding := range data.AllFindings {
			severity := strings.ToUpper(strings.TrimSpace(finding.Severity))
			if severity == "" {
				severity = "INFO"
			}
			b.WriteString(fmt.Sprintf("  - [%s] %s\n", severity, finding.Title))
			if finding.Description != "" {
				b.WriteString(fmt.Sprintf("     %s\n", finding.Description))
			}
			if finding.Action != "" {
				b.WriteString(fmt.Sprintf("     Action: %s\n", finding.Action))
			}
		}
	}

	b.WriteString("\nTraffic Sample\n")
	if data.HasTraffic {
		b.WriteString(fmt.Sprintf("  - Duration: %d minute(s)\n", data.Duration))
		b.WriteString(fmt.Sprintf("  - Total: %d records, %.2f GB\n", r.TrafficStats.TotalRecords, data.TotalTrafficGB))
		b.WriteString(fmt.Sprintf("  - S3: %.2f GB (%.1f%%)\n", data.S3GB, data.S3Pct))
		b.WriteString(fmt.Sprintf("  - DynamoDB: %.2f GB (%.1f%%)\n", data.DynamoGB, data.DynamoPct))
		b.WriteString(fmt.Sprintf("  - ECR: %.2f GB (%.1f%%)\n", data.ECRGB, data.ECRPct))
		b.WriteString(fmt.Sprintf("  - Other: %.2f GB (%.1f%%)\n", data.OtherGB, data.OtherPct))
	} else {
		b.WriteString("  - No traffic records were collected in this run\n")
	}

	if r.CostEstimate != nil {
		b.WriteString("\nCost Estimate (projected from sample)\n")
		b.WriteString(fmt.Sprintf("  - NAT data processing rate: $%.4f per GB\n", r.CostEstimate.NATGatewayPricePerGB))
		b.WriteString(fmt.Sprintf("  - NAT Gateway Data Processing Cost: $%.2f/month\n", r.CostEstimate.CurrentMonthlyCost))
		b.WriteString(fmt.Sprintf("  - S3 savings potential: $%.2f/month\n", r.CostEstimate.S3SavingsMonthly))
		b.WriteString(fmt.Sprintf("  - DynamoDB savings potential: $%.2f/month\n", r.CostEstimate.DynamoSavingsMonthly))
		if data.ECRCost > 0 {
			b.WriteString(fmt.Sprintf("  - ECR traffic cost (no free endpoint): $%.2f/month\n", data.ECRCost))
		}
		b.WriteString(fmt.Sprintf("  - Total savings potential: $%.2f/month ($%.2f/year)\n", r.CostEstimate.TotalSavingsMonthly, data.AnnualSavings))
	}

	if data.HasRemediation {
		b.WriteString("\nRemediation Commands\n")
		for _, cmd := range data.CreateEndpointCmds {
			b.WriteString(fmt.Sprintf("  %s\n", cmd))
		}
		for _, cmd := range data.AddRouteCmds {
			b.WriteString(fmt.Sprintf("  %s\n", cmd))
		}
	}

	if len(data.Recommendations) > 0 {
		b.WriteString("\nRecommendations\n")
		for i, rec := range data.Recommendations {
			b.WriteString(fmt.Sprintf("  %d. %s [%s]\n", i+1, rec.Title, strings.ToUpper(rec.Priority)))
			b.WriteString(fmt.Sprintf("     %s\n", rec.Description))
			if rec.Savings != "" {
				b.WriteString(fmt.Sprintf("     Savings: %s\n", rec.Savings))
			}
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// estimateMonthlyECRDataGB returns the projected ECR traffic volume for the month.
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
	data := r.ViewData()

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
	if len(data.OrderedVPCNATs) > 0 {
		b.WriteString("## VPC Endpoint Configuration\n\n")
		for _, group := range data.OrderedVPCNATs {
			b.WriteString(fmt.Sprintf("### VPC: %s\n\n", group.VPCID))
			if group.EndpointAnalysis == nil {
				b.WriteString("_No endpoint analysis available for this VPC._\n\n")
				continue
			}
			b.WriteString("```text\n")
			b.WriteString(group.EndpointAnalysis.String())
			b.WriteString("```\n\n")
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
		b.WriteString(fmt.Sprintf("| NAT Gateway Data Processing Cost | $%.2f/month |\n", r.CostEstimate.CurrentMonthlyCost))
		b.WriteString(fmt.Sprintf("| S3 Endpoint Savings | $%.2f/month |\n", r.CostEstimate.S3SavingsMonthly))
		b.WriteString(fmt.Sprintf("| DynamoDB Endpoint Savings | $%.2f/month |\n", r.CostEstimate.DynamoSavingsMonthly))
		if data.HasInterfaceEndpoints {
			b.WriteString(fmt.Sprintf("| Total Interface Endpoint Cost | $%.2f/month |\n", data.TotalInterfaceEndpointCost))
		}
		if ecrCost := r.estimateMonthlyECRNATCost(); ecrCost > 0 {
			b.WriteString(fmt.Sprintf("| ECR Traffic Cost over NAT (no free endpoint) | $%.2f/month |\n", ecrCost))
		}
		b.WriteString(fmt.Sprintf("| **Total Potential Savings** | **$%.2f/month** |\n\n", r.CostEstimate.TotalSavingsMonthly))

		if data.HasInterfaceEndpoints {
			b.WriteString("### Interface Endpoint Cost Details\n\n")
			b.WriteString("| VPC | Service | Endpoint | Monthly Cost |\n")
			b.WriteString("|-----|---------|----------|--------------|\n")
			for _, c := range data.InterfaceEndpointCosts {
				b.WriteString(fmt.Sprintf("| %s | %s | %s | $%.2f |\n", c.VPCID, c.ServiceName, c.DisplayName, c.MonthlyCost))
			}
			b.WriteString("\n")
		}
	}

	// Remediation
	if data.HasRemediation {
		b.WriteString("## Remediation Steps\n\n")

		if cmds := data.CreateEndpointCmds; len(cmds) > 0 {
			b.WriteString("### Create Missing VPC Endpoints\n\n")
			for _, cmd := range cmds {
				b.WriteString(fmt.Sprintf("```bash\n%s\n```\n\n", cmd))
			}
		}

		if cmds := data.AddRouteCmds; len(cmds) > 0 {
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
