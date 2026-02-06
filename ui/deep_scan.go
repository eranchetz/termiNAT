package ui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/internal/core"
	"github.com/doitintl/terminator/internal/report"
	"github.com/doitintl/terminator/pkg/types"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var tips = []string{
	"💡 VPC Gateway Endpoints for S3 and DynamoDB are FREE - no hourly or data charges",
	"💡 NAT Gateway charges $0.045/GB for data processing - this adds up quickly!",
	"💡 Flow Logs capture all network traffic metadata without impacting performance",
	"💡 You can run multiple scans at different times to understand traffic patterns",
	"💡 Consider running scans during peak hours for more representative data",
}

type phase int

const (
	phaseInit phase = iota
	phaseDiscovering
	phaseAwaitingApproval
	phaseCreatingResources
	phaseWaitingStartup
	phaseCollecting
	phaseAnalyzing
	phaseAwaitingCleanup
	phaseDone
)

type deepScanModel struct {
	scanner              *core.Scanner
	ctx                  context.Context
	duration             int
	natIDs               []string
	autoApprove          bool
	autoCleanup          bool
	spinner              spinner.Model
	phase                phase
	step                 string
	nats                 []types.NATGateway
	flowLogIDs           []string
	logGroupName         string
	runID                string
	trafficStats         *analysis.TrafficStats
	costEstimate         *analysis.CostEstimate
	endpointAnalysis     *analysis.EndpointAnalysis
	allFindings          []types.Finding // Quick scan findings for ALL VPCs
	deepScannedVPC       string          // VPC that was deep scanned
	recommendations      []analysis.Recommendation
	region               string
	accountID            string
	estimatedScanCostGB  float64
	estimatedScanCostUSD float64
	err                  error
	done                 bool
	startTime            time.Time
	phaseStartTime       time.Time
	tipIndex             int
	flowLogsStopped      bool
	exportMsg            string
	exportFormat         string
	outputFile           string
}

type tickMsg time.Time
type deepNatsDiscoveredMsg struct {
	nats            []types.NATGateway
	recommendations []analysis.Recommendation
	estGB           float64
	estCost         float64
}
type flowLogsCreatedMsg struct{ flowLogIDs []string }
type collectionCompleteMsg struct{}
type trafficAnalyzedMsg struct {
	stats            *analysis.TrafficStats
	cost             *analysis.CostEstimate
	endpointAnalysis *analysis.EndpointAnalysis
	allFindings      []types.Finding
	deepScannedVPC   string
}
type flowLogsStoppedMsg struct{}
type deepScanErrorMsg struct{ err error }
type deepScanCompleteMsg struct{}

func RunDeepScan(ctx context.Context, scanner *core.Scanner, region string, duration int, natIDs []string, autoApprove, autoCleanup bool, exportFormat, outputFile string) error {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	m := &deepScanModel{
		scanner:      scanner,
		ctx:          ctx,
		duration:     duration,
		natIDs:       natIDs,
		autoApprove:  autoApprove,
		autoCleanup:  autoCleanup,
		spinner:      s,
		phase:        phaseInit,
		region:       region,
		accountID:    scanner.GetAccountID(),
		runID:        fmt.Sprintf("terminat-%d", time.Now().Unix()),
		logGroupName: fmt.Sprintf("/aws/vpc/flowlogs/terminat-%d", time.Now().Unix()),
		startTime:    time.Now(),
		exportFormat: exportFormat,
		outputFile:   outputFile,
	}

	// Set up signal handler for cleanup on interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  Interrupt received - cleaning up Flow Logs...")
		m.cleanupFlowLogs()
		os.Exit(1)
	}()
	defer signal.Stop(sigChan)

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		m.cleanupFlowLogs()
		return err
	}
	return nil
}

func (m *deepScanModel) cleanupFlowLogs() {
	if len(m.flowLogIDs) > 0 && !m.flowLogsStopped {
		fmt.Printf("🧹 Stopping Flow Logs: %v\n", m.flowLogIDs)
		if err := m.scanner.DeleteFlowLogs(m.ctx, m.flowLogIDs); err != nil {
			fmt.Printf("⚠️  Warning: Failed to delete Flow Logs: %v\n", err)
		} else {
			fmt.Println("✓ Flow Logs stopped successfully")
		}
		m.flowLogsStopped = true
	}
}

func (m *deepScanModel) exportReport(format string) {
	r := report.New(m.region, m.accountID, m.duration, m.trafficStats, m.costEstimate, m.endpointAnalysis)

	var filename string
	var err error

	// Use custom filename if provided, otherwise generate timestamped name
	if m.outputFile != "" {
		filename = m.outputFile
	} else {
		timestamp := time.Now().Format("20060102-150405")
		ext := ".md"
		if format == "json" {
			ext = ".json"
		}
		filename = fmt.Sprintf("terminator-report-%s%s", timestamp, ext)
	}

	switch format {
	case "markdown":
		err = r.SaveMarkdown(filename)
	case "json":
		err = r.SaveJSON(filename)
	}

	// Get absolute path for clear output
	absPath, _ := filepath.Abs(filename)
	if absPath == "" {
		absPath = filename
	}

	if err != nil {
		m.exportMsg = fmt.Sprintf("❌ Export failed: %v", err)
	} else {
		m.exportMsg = fmt.Sprintf("✅ Report saved: %s", absPath)
	}
}

func (m *deepScanModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tick(), m.discoverNATs)
}

func (m *deepScanModel) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *deepScanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cleanupFlowLogs()
			return m, tea.Quit
		case "y", "Y":
			if m.phase == phaseAwaitingApproval {
				m.phase = phaseCreatingResources
				return m, m.createFlowLogs
			}
			if m.phase == phaseAwaitingCleanup {
				return m, m.deleteLogGroup
			}
		case "n", "N":
			if m.phase == phaseAwaitingApproval {
				m.done = true
				return m, tea.Quit
			}
			if m.phase == phaseAwaitingCleanup {
				// Auto-export if --export flag was provided
				if m.exportFormat != "" {
					m.exportReport(m.exportFormat)
				}
				m.phase = phaseDone
				return m, nil
			}
		case "m", "M":
			if m.phase == phaseDone {
				m.exportReport("markdown")
				return m, nil
			}
		case "j", "J":
			if m.phase == phaseDone {
				m.exportReport("json")
				return m, nil
			}
		case "enter", " ":
			if m.phase == phaseDone {
				return m, tea.Quit
			}
		}

	case tickMsg:
		m.tipIndex = (int(time.Since(m.startTime).Seconds()) / 5) % len(tips)
		return m, m.tick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case deepNatsDiscoveredMsg:
		m.nats = msg.nats
		m.recommendations = msg.recommendations
		m.estimatedScanCostGB = msg.estGB
		m.estimatedScanCostUSD = msg.estCost
		if m.autoApprove {
			m.phase = phaseCreatingResources
			return m, m.createFlowLogs
		}
		m.phase = phaseAwaitingApproval
		return m, nil

	case flowLogsCreatedMsg:
		m.flowLogIDs = msg.flowLogIDs
		m.phase = phaseWaitingStartup
		m.phaseStartTime = time.Now()
		return m, m.waitForStartup

	case collectionCompleteMsg:
		m.phase = phaseAnalyzing
		return m, m.analyzeTraffic

	case trafficAnalyzedMsg:
		m.trafficStats = msg.stats
		m.costEstimate = msg.cost
		m.endpointAnalysis = msg.endpointAnalysis
		m.allFindings = msg.allFindings
		m.deepScannedVPC = msg.deepScannedVPC
		return m, m.stopFlowLogs

	case flowLogsStoppedMsg:
		m.flowLogsStopped = true
		if m.autoApprove {
			if m.exportFormat != "" {
				m.exportReport(m.exportFormat)
			}
			if m.autoCleanup {
				return m, m.deleteLogGroup
			}
			m.done = true
			m.phase = phaseDone
			return m, tea.Quit
		}
		m.phase = phaseAwaitingCleanup
		return m, nil

	case deepScanErrorMsg:
		m.err = msg.err
		m.done = true
		m.cleanupFlowLogs()
		return m, tea.Quit

	case deepScanCompleteMsg:
		// Auto-export if --export flag was provided
		if m.exportFormat != "" {
			m.exportReport(m.exportFormat)
		}
		m.phase = phaseDone
		return m, nil
	}
	return m, nil
}

func (m *deepScanModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("termiNATor - Deep Dive Scan"))
	b.WriteString("\n\n")
	b.WriteString(infoStyle.Render(fmt.Sprintf("Region: %s  |  Account: %s  |  Elapsed: %s\n\n",
		m.region, m.accountID, formatDuration(time.Since(m.startTime)))))

	switch m.phase {
	case phaseInit, phaseDiscovering:
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), "Discovering NAT Gateways..."))
	case phaseAwaitingApproval:
		b.WriteString(m.renderApprovalPrompt())
	case phaseCreatingResources:
		b.WriteString(fmt.Sprintf("%s Creating Flow Logs and CloudWatch resources...\n", m.spinner.View()))
	case phaseWaitingStartup, phaseCollecting:
		b.WriteString(m.renderProgress())
	case phaseAnalyzing:
		b.WriteString(fmt.Sprintf("%s Analyzing traffic patterns...\n", m.spinner.View()))
	case phaseAwaitingCleanup:
		b.WriteString(m.renderCleanupPrompt())
	case phaseDone:
		b.WriteString(m.renderFinalReport())
	}

	return b.String()
}

func (m *deepScanModel) renderApprovalPrompt() string {
	var b strings.Builder
	b.WriteString(warningStyle.Render("⚠️  RESOURCE CREATION APPROVAL REQUIRED\n\n"))
	b.WriteString("The following AWS resources will be created:\n\n")

	b.WriteString(stepStyle.Render("1. VPC Flow Logs (temporary)\n"))
	for _, nat := range m.nats {
		mode := nat.AvailabilityMode
		if mode == "" {
			mode = "zonal"
		}
		b.WriteString(fmt.Sprintf("   • NAT Gateway: %s (%s, VPC: %s)\n", nat.ID, mode, nat.VPCID))
	}
	b.WriteString(infoStyle.Render("   → Flow Logs will be AUTOMATICALLY STOPPED after analysis\n"))

	b.WriteString(stepStyle.Render("\n2. CloudWatch Log Group\n"))
	b.WriteString(fmt.Sprintf("   • %s\n", m.logGroupName))
	b.WriteString(infoStyle.Render("   → You'll be asked whether to keep or delete after scan\n"))

	b.WriteString(stepStyle.Render("\n📊 Estimated Costs:\n"))
	if m.estimatedScanCostGB > 0 {
		b.WriteString(fmt.Sprintf("   • Estimated flow log data: ~%.2f GB (based on current NAT throughput)\n", m.estimatedScanCostGB))
		b.WriteString(fmt.Sprintf("   • Flow Logs ingestion (~$0.50/GB): ~$%.2f\n", m.estimatedScanCostUSD))
		b.WriteString(fmt.Sprintf("   • CloudWatch storage (~$0.03/GB/month): ~$%.4f/month\n", m.estimatedScanCostGB*0.03))
	} else {
		b.WriteString("   • Flow Logs ingestion: ~$0.50 per GB\n")
		b.WriteString("   • CloudWatch Logs storage: ~$0.03 per GB/month\n")
		b.WriteString("   • For a 5-minute scan, typical cost: < $0.10\n")
	}

	b.WriteString(stepStyle.Render(fmt.Sprintf("\n⏱️  Total scan time: %d minutes\n", m.duration+5)))
	b.WriteString("   • 5 min startup delay (Flow Logs initialization)\n")
	b.WriteString(fmt.Sprintf("   • %d min traffic collection\n\n", m.duration))

	b.WriteString(highlightStyle.Render("Proceed with scan? [Y/n] "))
	return b.String()
}

func (m *deepScanModel) renderProgress() string {
	var b strings.Builder
	elapsed := time.Since(m.phaseStartTime)
	var remaining time.Duration
	var phaseName string

	if m.phase == phaseWaitingStartup {
		phaseName = "Flow Logs Initialization"
		remaining = 5*time.Minute - elapsed
		if remaining < 0 {
			remaining = 0
		}
	} else {
		phaseName = "Traffic Collection"
		remaining = time.Duration(m.duration)*time.Minute - elapsed
		if remaining < 0 {
			remaining = 0
		}
	}

	var total float64
	if m.phase == phaseWaitingStartup {
		total = 5 * 60
	} else {
		total = float64(m.duration * 60)
	}
	progress := elapsed.Seconds() / total
	if progress > 1 {
		progress = 1
	}
	barWidth := 40
	filled := int(progress * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	b.WriteString(fmt.Sprintf("%s %s\n\n", m.spinner.View(), stepStyle.Render(phaseName)))
	b.WriteString(fmt.Sprintf("  [%s] %.0f%%\n\n", bar, progress*100))
	b.WriteString(fmt.Sprintf("  ⏱️  Elapsed: %s  |  Remaining: %s\n\n", formatDuration(elapsed), formatDuration(remaining)))

	b.WriteString(infoStyle.Render("Monitoring:\n"))
	for _, nat := range m.nats {
		b.WriteString(fmt.Sprintf("  • %s (%s)\n", nat.ID, nat.VPCID))
	}
	b.WriteString("\n")
	b.WriteString(tipStyle.Render(tips[m.tipIndex]))
	b.WriteString("\n")

	return b.String()
}

func (m *deepScanModel) renderCleanupPrompt() string {
	var b strings.Builder
	b.WriteString(successStyle.Render("✓ Flow Logs STOPPED\n\n"))

	b.WriteString(warningStyle.Render("CloudWatch Log Group Cleanup\n\n"))
	b.WriteString(fmt.Sprintf("Log Group: %s\n\n", m.logGroupName))
	b.WriteString("This log group contains the collected traffic data.\n")
	b.WriteString("• Keep it to analyze traffic patterns in CloudWatch Logs Insights\n")
	b.WriteString("• Delete it to avoid storage costs (~$0.03/GB/month)\n\n")

	b.WriteString(highlightStyle.Render("Delete CloudWatch Log Group? [Y/n] "))
	return b.String()
}

func (m *deepScanModel) renderFinalReport() string {
	var b strings.Builder
	b.WriteString(successStyle.Render("✓ Deep Dive Scan Complete\n"))
	b.WriteString(successStyle.Render("✓ Flow Logs STOPPED\n\n"))

	// NAT Gateway Overview - show which were deep scanned vs config-only
	b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
	b.WriteString(stepStyle.Render("                    NAT GATEWAY OVERVIEW\n"))
	b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))

	// Group NATs by VPC
	vpcNATs := make(map[string][]types.NATGateway)
	for _, nat := range m.nats {
		vpcNATs[nat.VPCID] = append(vpcNATs[nat.VPCID], nat)
	}

	for vpcID, nats := range vpcNATs {
		isDeepScanned := vpcID == m.deepScannedVPC
		if isDeepScanned {
			b.WriteString(highlightStyle.Render(fmt.Sprintf("📊 VPC: %s [DEEP SCANNED - Traffic Analyzed]\n", vpcID)))
		} else {
			b.WriteString(infoStyle.Render(fmt.Sprintf("📋 VPC: %s [Config Check Only]\n", vpcID)))
		}
		for _, nat := range nats {
			mode := nat.AvailabilityMode
			if mode == "" {
				mode = "zonal"
			}
			b.WriteString(fmt.Sprintf("   • %s (%s)\n", nat.ID, mode))
		}
		b.WriteString("\n")
	}

	// Show findings for ALL VPCs (quick scan results)
	if len(m.allFindings) > 0 {
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
		b.WriteString(stepStyle.Render("              VPC ENDPOINT ISSUES (All VPCs)\n"))
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))

		b.WriteString(warningStyle.Render(fmt.Sprintf("⚠️  Found %d issue(s) across all VPCs:\n\n", len(m.allFindings))))
		for _, finding := range m.allFindings {
			b.WriteString(fmt.Sprintf("  [%s] %s\n", strings.ToUpper(finding.Severity), finding.Title))
			b.WriteString(fmt.Sprintf("      %s\n", finding.Description))
			b.WriteString(infoStyle.Render(fmt.Sprintf("      → %s\n\n", finding.Action)))
		}
	} else {
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
		b.WriteString(stepStyle.Render("              VPC ENDPOINT STATUS (All VPCs)\n"))
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))
		b.WriteString(successStyle.Render("✓ All VPCs have proper endpoint configuration!\n\n"))
	}

	// VPC Endpoint Analysis Section for deep scanned VPC (detailed)
	if m.endpointAnalysis != nil {
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
		b.WriteString(stepStyle.Render("         DETAILED ENDPOINT CONFIG (Deep Scanned VPC)\n"))
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))

		b.WriteString(fmt.Sprintf("VPC: %s\n\n", m.endpointAnalysis.VPCID))

		// Endpoint status
		b.WriteString(stepStyle.Render("Gateway Endpoints:\n"))
		if m.endpointAnalysis.S3Endpoint != nil {
			b.WriteString(fmt.Sprintf("  ✓ S3: %s (%d route tables)\n",
				m.endpointAnalysis.S3Endpoint.ID, len(m.endpointAnalysis.S3Endpoint.RouteTables)))
		} else {
			b.WriteString(warningStyle.Render("  ✗ S3: NOT CONFIGURED\n"))
		}
		if m.endpointAnalysis.DynamoEndpoint != nil {
			b.WriteString(fmt.Sprintf("  ✓ DynamoDB: %s (%d route tables)\n",
				m.endpointAnalysis.DynamoEndpoint.ID, len(m.endpointAnalysis.DynamoEndpoint.RouteTables)))
		} else {
			b.WriteString(warningStyle.Render("  ✗ DynamoDB: NOT CONFIGURED\n"))
		}
		b.WriteString("\n")

		// Missing routes
		if len(m.endpointAnalysis.MissingRoutes) > 0 {
			b.WriteString(warningStyle.Render("Route Tables Missing Endpoint Routes:\n"))
			for _, mr := range m.endpointAnalysis.MissingRoutes {
				b.WriteString(fmt.Sprintf("  • %s: missing %s route\n", mr.RouteTableID, mr.Service))
			}
			b.WriteString("\n")
		}

		// Interface Endpoints
		if m.endpointAnalysis.HasInterfaceEndpoints() {
			b.WriteString(stepStyle.Render("Interface Endpoints:\n"))
			costs := m.endpointAnalysis.GetInterfaceEndpointCosts()
			for _, c := range costs {
				name := c.Endpoint.Tags["Name"]
				if name == "" {
					name = c.Endpoint.ID
				}
				b.WriteString(fmt.Sprintf("  • %s (%s): %s/month\n", c.ServiceName, name, formatCurrency(c.MonthlyCost)))
			}
			totalCost := m.endpointAnalysis.GetTotalInterfaceEndpointMonthlyCost()
			b.WriteString(fmt.Sprintf("\n  Total Interface Endpoint Cost: %s/month\n", formatCurrency(totalCost)))
			b.WriteString(infoStyle.Render("  💡 Interface endpoints cost $0.01/hour + $0.01/GB data processed\n"))
			b.WriteString(infoStyle.Render("  💡 Review unused endpoints to reduce costs\n"))
			b.WriteString("\n")
		}
	}

	// Traffic Analysis
	if m.trafficStats != nil && m.trafficStats.TotalRecords > 0 {
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
		b.WriteString(stepStyle.Render("                 COLLECTED TRAFFIC SAMPLE\n"))
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))

		b.WriteString(infoStyle.Render(fmt.Sprintf("Sample period: %d minutes\n\n", m.duration)))
		b.WriteString(fmt.Sprintf("Total Traffic: %d records, %.2f GB\n\n",
			m.trafficStats.TotalRecords, float64(m.trafficStats.TotalBytes)/(1024*1024*1024)))

		b.WriteString(stepStyle.Render("Traffic by Service:\n"))
		b.WriteString(fmt.Sprintf("  %-12s %10s %10s\n", "Service", "Data", "Percentage"))
		b.WriteString(fmt.Sprintf("  %-12s %10s %10s\n", "───────────", "─────────", "──────────"))
		b.WriteString(fmt.Sprintf("  %-12s %9.2f GB %9.1f%%\n", "S3",
			float64(m.trafficStats.S3Bytes)/(1024*1024*1024), m.trafficStats.S3Percentage()))
		b.WriteString(fmt.Sprintf("  %-12s %9.2f GB %9.1f%%\n", "DynamoDB",
			float64(m.trafficStats.DynamoBytes)/(1024*1024*1024), m.trafficStats.DynamoPercentage()))
		b.WriteString(fmt.Sprintf("  %-12s %9.2f GB %9.1f%%\n", "ECR",
			float64(m.trafficStats.ECRBytes)/(1024*1024*1024), m.trafficStats.ECRPercentage()))
		b.WriteString(fmt.Sprintf("  %-12s %9.2f GB %9.1f%%\n\n", "Other",
			float64(m.trafficStats.OtherBytes)/(1024*1024*1024), m.trafficStats.OtherPercentage()))

		if len(m.trafficStats.SourceIPs) > 0 {
			b.WriteString(stepStyle.Render("Top Source IPs:\n"))
			for _, entry := range m.trafficStats.TopSourceIPs(10) {
				b.WriteString(fmt.Sprintf("  • %s: %.2f GB (%d records)\n", entry.IP,
					float64(entry.Stats.Bytes)/(1024*1024*1024), entry.Stats.Records))
			}
			if len(m.trafficStats.SourceIPs) > 10 {
				b.WriteString(fmt.Sprintf("  ... and %d more sources\n", len(m.trafficStats.SourceIPs)-10))
			}
			b.WriteString("\n")
		}
	}

	// Cost Estimate
	if m.costEstimate != nil {
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
		b.WriteString(stepStyle.Render("                      COST ESTIMATE\n"))
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))

		b.WriteString(warningStyle.Render(fmt.Sprintf("⚠️  Projected from %d-minute sample to monthly estimate\n\n", m.duration)))
		b.WriteString(fmt.Sprintf("NAT Gateway Data Processing: $%.4f per GB\n\n", m.costEstimate.NATGatewayPricePerGB))
		b.WriteString(stepStyle.Render("Projected Monthly Costs:\n"))
		b.WriteString(fmt.Sprintf("  Current NAT Gateway cost:     %s/month\n", formatCurrency(m.costEstimate.CurrentMonthlyCost)))
		b.WriteString(fmt.Sprintf("  Potential S3 savings:         %s/month\n", formatCurrency(m.costEstimate.S3SavingsMonthly)))
		b.WriteString(fmt.Sprintf("  Potential DynamoDB savings:   %s/month\n", formatCurrency(m.costEstimate.DynamoSavingsMonthly)))
		b.WriteString("  ─────────────────────────────────────────\n")
		b.WriteString(highlightStyle.Render(fmt.Sprintf("  TOTAL POTENTIAL SAVINGS:      %s/month (%s/year)\n\n",
			formatCurrency(m.costEstimate.TotalSavingsMonthly), formatCurrency(m.costEstimate.TotalSavingsMonthly*12))))

		b.WriteString(infoStyle.Render("Note: Actual costs depend on real traffic patterns. Run longer\n"))
		b.WriteString(infoStyle.Render("scans during peak hours for more accurate estimates.\n\n"))
	}

	// Remediation Steps
	if m.endpointAnalysis != nil && m.endpointAnalysis.HasIssues() {
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
		b.WriteString(stepStyle.Render("                    REMEDIATION STEPS\n"))
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))

		// Create missing endpoints
		if cmds := m.endpointAnalysis.GetCreateEndpointCommands(); len(cmds) > 0 {
			b.WriteString(stepStyle.Render("📦 Create Missing VPC Endpoints:\n\n"))
			for _, cmd := range cmds {
				b.WriteString(fmt.Sprintf("%s\n\n", cmd))
			}
		}

		// Add missing routes
		if cmds := m.endpointAnalysis.GetAddRouteCommands(); len(cmds) > 0 {
			b.WriteString(stepStyle.Render("🔗 Add Missing Route Table Associations:\n\n"))
			for _, cmd := range cmds {
				b.WriteString(fmt.Sprintf("%s\n\n", cmd))
			}
		}
	}

	// Recommendations
	if len(m.recommendations) > 0 {
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
		b.WriteString(stepStyle.Render("                      RECOMMENDATIONS\n"))
		b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))

		for i, rec := range m.recommendations {
			b.WriteString(highlightStyle.Render(fmt.Sprintf("%d. %s [%s priority]\n\n", i+1, rec.Title, strings.ToUpper(rec.Priority))))
			b.WriteString(fmt.Sprintf("%s\n\n", rec.Description))

			if len(rec.Benefits) > 0 {
				b.WriteString(stepStyle.Render("Benefits:\n"))
				for _, benefit := range rec.Benefits {
					b.WriteString(fmt.Sprintf("  ✓ %s\n", benefit))
				}
				b.WriteString("\n")
			}

			if rec.Savings != "" {
				b.WriteString(highlightStyle.Render(fmt.Sprintf("💰 %s\n\n", rec.Savings)))
			}

			if len(rec.Commands) > 0 {
				b.WriteString(stepStyle.Render("How to implement:\n"))
				for _, cmd := range rec.Commands {
					if strings.HasPrefix(cmd, "#") {
						b.WriteString(infoStyle.Render(fmt.Sprintf("%s\n", cmd)))
					} else {
						b.WriteString(fmt.Sprintf("%s\n", cmd))
					}
				}
				b.WriteString("\n")
			}

			if i < len(m.recommendations)-1 {
				b.WriteString(strings.Repeat("-", 63) + "\n\n")
			}
		}
	}

	b.WriteString(warningStyle.Render("⚠️  DISCLAIMERS:\n"))
	b.WriteString("   • Cost estimates based on traffic sample collected\n")
	b.WriteString("   • Actual costs may vary based on traffic patterns\n")
	b.WriteString("   • Gateway VPC Endpoints for S3 and DynamoDB are FREE\n\n")

	// Export options
	b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n"))
	b.WriteString(stepStyle.Render("                       EXPORT OPTIONS\n"))
	b.WriteString(stepStyle.Render("═══════════════════════════════════════════════════════════════\n\n"))
	b.WriteString("  [M] Export as Markdown    [J] Export as JSON    [Enter] Exit\n")
	if m.exportMsg != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n", m.exportMsg))
	}

	return b.String()
}

func (m *deepScanModel) discoverNATs() tea.Msg {
	nats, err := m.scanner.DiscoverNATGateways(m.ctx)
	if err != nil {
		return deepScanErrorMsg{err: err}
	}

	if len(m.natIDs) > 0 {
		filtered := []types.NATGateway{}
		for _, nat := range nats {
			for _, id := range m.natIDs {
				if nat.ID == id {
					filtered = append(filtered, nat)
					break
				}
			}
		}
		nats = filtered
	}

	if len(nats) == 0 {
		return deepScanErrorMsg{err: fmt.Errorf("no NAT gateways found")}
	}

	// Generate recommendations based on NAT Gateway setup
	recommendations := analysis.AnalyzeNATGatewaySetup(nats)

	// Estimate scan cost from recent NAT throughput
	var natIDs []string
	for _, nat := range nats {
		natIDs = append(natIDs, nat.ID)
	}
	estGB, estCost, _ := m.scanner.EstimateFlowLogsCost(m.ctx, natIDs, m.duration)

	return deepNatsDiscoveredMsg{nats: nats, recommendations: recommendations, estGB: estGB, estCost: estCost}
}

func (m *deepScanModel) createFlowLogs() tea.Msg {
	// Use dynamic account ID from scanner
	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/termiNATor-FlowLogsRole", m.accountID)

	// Validate IAM role exists before proceeding
	if err := m.scanner.ValidateFlowLogsRole(m.ctx, roleARN); err != nil {
		return deepScanErrorMsg{err: err}
	}

	if err := m.scanner.CreateLogGroup(m.ctx, m.logGroupName); err != nil {
		return deepScanErrorMsg{err: fmt.Errorf("failed to create log group: %w", err)}
	}

	var flowLogIDs []string
	for _, nat := range m.nats {
		flowLogID, err := m.scanner.CreateFlowLogs(m.ctx, nat, m.logGroupName, roleARN, m.runID)
		if err != nil {
			if len(flowLogIDs) > 0 {
				m.scanner.DeleteFlowLogs(m.ctx, flowLogIDs)
			}
			m.scanner.DeleteLogGroup(m.ctx, m.logGroupName)
			return deepScanErrorMsg{err: fmt.Errorf("failed to create flow logs: %w", err)}
		}
		flowLogIDs = append(flowLogIDs, flowLogID)
	}
	return flowLogsCreatedMsg{flowLogIDs: flowLogIDs}
}

func (m *deepScanModel) waitForStartup() tea.Msg {
	// Poll for Flow Logs to become ACTIVE instead of fixed sleep
	timeout := 10 * time.Minute
	pollInterval := 30 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		activeFlowLogs, err := m.scanner.CheckActiveFlowLogs(m.ctx, m.logGroupName)
		if err == nil && len(activeFlowLogs) > 0 {
			// Flow Logs are active, proceed to collection
			m.phase = phaseCollecting
			m.phaseStartTime = time.Now()
			time.Sleep(time.Duration(m.duration) * time.Minute)
			return collectionCompleteMsg{}
		}

		// Check if context was cancelled
		select {
		case <-m.ctx.Done():
			return deepScanErrorMsg{err: fmt.Errorf("scan cancelled during Flow Logs startup")}
		case <-time.After(pollInterval):
			// Continue polling
		}
	}

	return deepScanErrorMsg{err: fmt.Errorf("timeout waiting for Flow Logs to become ACTIVE after %v", timeout)}
}

func (m *deepScanModel) analyzeTraffic() tea.Msg {
	endTime := time.Now().Unix()
	startTime := endTime - int64(m.duration*60) - 300

	stats, err := m.scanner.AnalyzeTraffic(m.ctx, m.logGroupName, startTime, endTime)
	if err != nil {
		return deepScanErrorMsg{err: fmt.Errorf("failed to analyze traffic: %w", err)}
	}

	costEstimate := m.scanner.CalculateCosts(stats, m.duration)

	// Analyze VPC endpoints for the deep scanned VPC
	var endpointAnalysis *analysis.EndpointAnalysis
	var deepScannedVPC string
	if len(m.nats) > 0 {
		deepScannedVPC = m.nats[0].VPCID
		endpointAnalysis, _ = m.scanner.AnalyzeVPCEndpoints(m.ctx, deepScannedVPC)
	}

	// Run quick scan analysis on ALL VPCs (not just the deep scanned one)
	allFindings := analysis.AnalyzeAllVPCEndpoints(m.ctx, m.scanner, m.nats)

	return trafficAnalyzedMsg{
		stats:            stats,
		cost:             costEstimate,
		endpointAnalysis: endpointAnalysis,
		allFindings:      allFindings,
		deepScannedVPC:   deepScannedVPC,
	}
}

func (m *deepScanModel) stopFlowLogs() tea.Msg {
	if len(m.flowLogIDs) > 0 {
		if err := m.scanner.DeleteFlowLogs(m.ctx, m.flowLogIDs); err != nil {
			return deepScanErrorMsg{err: fmt.Errorf("failed to stop flow logs: %w", err)}
		}
	}
	return flowLogsStoppedMsg{}
}

func (m *deepScanModel) deleteLogGroup() tea.Msg {
	if err := m.scanner.DeleteLogGroup(m.ctx, m.logGroupName); err != nil {
		return deepScanErrorMsg{err: fmt.Errorf("failed to delete log group: %w", err)}
	}
	return deepScanCompleteMsg{}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	return fmt.Sprintf("%02d:%02d", m, s)
}

// formatCurrency formats a float as currency with commas (e.g., $1,234.56)
func formatCurrency(amount float64) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("$%,.2f", amount)
}

var (
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Bold(true)
	highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
	tipStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true)
)
