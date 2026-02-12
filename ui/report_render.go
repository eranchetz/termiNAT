package ui

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/pkg/types"
)

//go:embed templates/report.tmpl
var reportTmplFS embed.FS

var reportTmpl = template.Must(template.New("report.tmpl").Funcs(tmplFuncs).ParseFS(reportTmplFS, "templates/report.tmpl"))

var tmplFuncs = template.FuncMap{
	"green":     func(s string) string { return stepStyle.Render(s) },
	"warn":      func(s string) string { return warningStyle.Render(s) },
	"success":   func(s string) string { return successStyle.Render(s) },
	"highlight": func(s string) string { return highlightStyle.Render(s) },
	"dim":       func(s string) string { return infoStyle.Render(s) },
	"header":    sectionHeader,
	"currency":  formatCurrency,
	"upper":     strings.ToUpper,
	"hasPrefix": strings.HasPrefix,
	"inc":       func(i int) int { return i + 1 },
	"indent": func(cmd string) string {
		var b strings.Builder
		for i, line := range strings.Split(cmd, "\n") {
			if i == 0 {
				b.WriteString("  " + line + "\n")
			} else {
				b.WriteString("    " + line + "\n")
			}
		}
		return b.String()
	},
}

func sectionHeader(title string) string {
	line := strings.Repeat("─", 60)
	return stepStyle.Render(line) + "\n" + stepStyle.Render(title) + "\n" + stepStyle.Render(line) + "\n"
}

// reportData holds all data needed by the report template.
type reportData struct {
	VPCNATs          map[string][]types.NATGateway
	DeepScannedVPC   string
	AllFindings      []types.Finding
	EndpointAnalysis *analysis.EndpointAnalysis
	TrafficStats     *analysis.TrafficStats
	CostEstimate     *analysis.CostEstimate
	Recommendations  []analysis.Recommendation
	Duration         int
	LogGroupName     string

	// Computed fields
	HasTraffic                         bool
	HasRemediation                     bool
	HasInterfaceEndpoints              bool
	MissingRoutes                      []analysis.MissingRoute
	InterfaceEndpointCosts             []epCostDisplay
	TotalInterfaceEndpointCost         float64
	TotalTrafficGB                     float64
	S3GB, DynamoGB, ECRGB, OtherGB     float64
	S3Pct, DynamoPct, ECRPct, OtherPct float64
	TopSourceIPs                       []sourceIPDisplay
	MoreSources                        int
	ECRCost                            float64
	AnnualSavings                      float64
	CreateEndpointCmds                 []string
	AddRouteCmds                       []string
}

type epCostDisplay struct {
	ServiceName string
	DisplayName string
	MonthlyCost float64
}

type sourceIPDisplay struct {
	IP      string
	GB      float64
	Records int
}

func (m *deepScanModel) buildReportData() reportData {
	d := reportData{
		VPCNATs:          make(map[string][]types.NATGateway),
		DeepScannedVPC:   m.deepScannedVPC,
		AllFindings:      m.allFindings,
		EndpointAnalysis: m.endpointAnalysis,
		TrafficStats:     m.trafficStats,
		CostEstimate:     m.costEstimate,
		Recommendations:  m.recommendations,
		Duration:         m.duration,
		LogGroupName:     m.logGroupName,
	}

	for _, nat := range m.nats {
		d.VPCNATs[nat.VPCID] = append(d.VPCNATs[nat.VPCID], nat)
	}

	if m.endpointAnalysis != nil {
		d.MissingRoutes = m.endpointAnalysis.MissingRoutes
		d.HasInterfaceEndpoints = m.endpointAnalysis.HasInterfaceEndpoints()
		d.HasRemediation = m.endpointAnalysis.HasIssues()
		if d.HasRemediation {
			d.CreateEndpointCmds = m.endpointAnalysis.GetCreateEndpointCommands()
			d.AddRouteCmds = m.endpointAnalysis.GetAddRouteCommands()
		}
		if d.HasInterfaceEndpoints {
			d.TotalInterfaceEndpointCost = m.endpointAnalysis.GetTotalInterfaceEndpointMonthlyCost()
			for _, c := range m.endpointAnalysis.GetInterfaceEndpointCosts() {
				name := c.Endpoint.Tags["Name"]
				if name == "" {
					name = c.Endpoint.ID
				}
				d.InterfaceEndpointCosts = append(d.InterfaceEndpointCosts, epCostDisplay{
					ServiceName: c.ServiceName,
					DisplayName: name,
					MonthlyCost: c.MonthlyCost,
				})
			}
		}
	}

	if m.trafficStats != nil && m.trafficStats.TotalRecords > 0 {
		d.HasTraffic = true
		d.TotalTrafficGB = float64(m.trafficStats.TotalBytes) / (1024 * 1024 * 1024)
		d.S3GB = float64(m.trafficStats.S3Bytes) / (1024 * 1024 * 1024)
		d.DynamoGB = float64(m.trafficStats.DynamoBytes) / (1024 * 1024 * 1024)
		d.ECRGB = float64(m.trafficStats.ECRBytes) / (1024 * 1024 * 1024)
		d.OtherGB = float64(m.trafficStats.OtherBytes) / (1024 * 1024 * 1024)
		d.S3Pct = m.trafficStats.S3Percentage()
		d.DynamoPct = m.trafficStats.DynamoPercentage()
		d.ECRPct = m.trafficStats.ECRPercentage()
		d.OtherPct = m.trafficStats.OtherPercentage()

		top := m.trafficStats.TopSourceIPs(10)
		for _, e := range top {
			d.TopSourceIPs = append(d.TopSourceIPs, sourceIPDisplay{
				IP:      e.IP,
				GB:      float64(e.Stats.Bytes) / (1024 * 1024 * 1024),
				Records: e.Stats.Records,
			})
		}
		if len(m.trafficStats.SourceIPs) > 10 {
			d.MoreSources = len(m.trafficStats.SourceIPs) - 10
		}
	}

	if m.costEstimate != nil {
		d.AnnualSavings = m.costEstimate.TotalSavingsMonthly * 12
		if m.trafficStats != nil && m.trafficStats.ECRBytes > 0 && m.costEstimate.OtherPercentage() > 0 {
			d.ECRCost = m.costEstimate.OtherDataGB * m.costEstimate.NATGatewayPricePerGB * (m.trafficStats.ECRPercentage() / m.costEstimate.OtherPercentage())
		}
	}

	return d
}

func (m *deepScanModel) renderFinalReport() string {
	return m.renderReportBody() + "\n" + m.renderFooter()
}

func (m *deepScanModel) renderReportBody() string {
	data := m.buildReportData()
	var buf bytes.Buffer
	if err := reportTmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Error rendering report: %v", err)
	}
	return buf.String()
}

func (m *deepScanModel) renderFooter() string {
	var b strings.Builder
	b.WriteString("  [M] Markdown  [J] JSON  [D] DoiT DataHub  [↑↓] Scroll  [Enter] Exit\n")
	if m.exportMsg != "" {
		b.WriteString(fmt.Sprintf("  %s\n", m.exportMsg))
	}
	if m.datahubMsg != "" {
		b.WriteString(fmt.Sprintf("%s\n", m.datahubMsg))
	}
	switch m.datahubPhase {
	case 1:
		b.WriteString(fmt.Sprintf("  Enter DoiT DataHub API key: %s█\n", m.datahubInputBuf))
	case 2:
		b.WriteString(fmt.Sprintf("  Customer context (optional, Enter to skip): %s█\n", m.datahubInputBuf))
	case 3:
		b.WriteString("  Save API key for future use? [Y/n] ")
	case 4:
		b.WriteString("  ⏳ Sending to DoiT DataHub...\n")
	}
	return b.String()
}
