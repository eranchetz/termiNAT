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
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/internal/core"
	"github.com/doitintl/terminator/internal/datahub"
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
	phaseSelectingNATs
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
	vpcID                string
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
	datahubAPIKey        string
	datahubCustomerCtx   string
	datahubMsg           string
	datahubInputBuf      string
	datahubPhase         int // 0=none, 1=prompting-key, 2=prompting-context, 3=prompting-save, 4=sending
	viewport             viewport.Model
	viewportReady        bool
	natCursor            int
	natSelected          map[int]bool
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
type datahubResultMsg struct{ err error }

func RunDeepScan(ctx context.Context, scanner *core.Scanner, region string, duration int, natIDs []string, vpcID, uiMode string, autoApprove, autoCleanup bool, exportFormat, outputFile string, datahubAPIKey, datahubCustomerCtx string) error {
	switch strings.ToLower(strings.TrimSpace(uiMode)) {
	case "", "stream":
		return RunDeepScanStream(ctx, scanner, region, duration, natIDs, vpcID, autoApprove, autoCleanup, exportFormat, outputFile, datahubAPIKey, datahubCustomerCtx)
	case "tui":
		return runDeepScanTUI(ctx, scanner, region, duration, natIDs, vpcID, autoApprove, autoCleanup, exportFormat, outputFile, datahubAPIKey, datahubCustomerCtx)
	default:
		return fmt.Errorf("invalid --ui value %q (valid: stream, tui)", uiMode)
	}
}

func runDeepScanTUI(ctx context.Context, scanner *core.Scanner, region string, duration int, natIDs []string, vpcID string, autoApprove, autoCleanup bool, exportFormat, outputFile string, datahubAPIKey, datahubCustomerCtx string) error {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	m := &deepScanModel{
		scanner:            scanner,
		ctx:                ctx,
		duration:           duration,
		natIDs:             natIDs,
		vpcID:              vpcID,
		autoApprove:        autoApprove,
		autoCleanup:        autoCleanup,
		spinner:            s,
		phase:              phaseInit,
		region:             region,
		accountID:          scanner.GetAccountID(),
		runID:              fmt.Sprintf("terminat-%d", time.Now().Unix()),
		logGroupName:       fmt.Sprintf("/aws/vpc/flowlogs/terminat-%d", time.Now().Unix()),
		startTime:          time.Now(),
		exportFormat:       exportFormat,
		outputFile:         outputFile,
		datahubAPIKey:      datahub.ResolveAPIKey(datahubAPIKey),
		datahubCustomerCtx: datahub.ResolveCustomerContext(datahubCustomerCtx),
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

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
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

func (m *deepScanModel) sendToDataHub() tea.Msg {
	events := datahub.BuildEvents(m.accountID, m.region, m.nats, m.trafficStats, m.costEstimate, m.endpointAnalysis)
	err := datahub.Send(m.datahubAPIKey, m.datahubCustomerCtx, events)
	return datahubResultMsg{err: err}
}

func (m *deepScanModel) enterPhaseDone() {
	m.phase = phaseDone
	if m.viewportReady {
		m.viewport.SetContent(m.renderReportBody())
		m.viewport.GotoTop()
	}
}

func (m *deepScanModel) Init() tea.Cmd {
	if m.phase == phaseDone {
		return nil
	}
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
		}

		// NAT selection phase key handlers
		if m.phase == phaseSelectingNATs {
			switch msg.String() {
			case "up", "k":
				if m.natCursor > 0 {
					m.natCursor--
				}
			case "down", "j":
				if m.natCursor < len(m.nats)-1 {
					m.natCursor++
				}
			case " ":
				m.natSelected[m.natCursor] = !m.natSelected[m.natCursor]
			case "a":
				allSelected := true
				for i := range m.nats {
					if !m.natSelected[i] {
						allSelected = false
						break
					}
				}
				for i := range m.nats {
					m.natSelected[i] = !allSelected
				}
			case "enter":
				selected := []types.NATGateway{}
				for i, nat := range m.nats {
					if m.natSelected[i] {
						selected = append(selected, nat)
					}
				}
				if len(selected) == 0 {
					return m, nil // don't proceed with nothing selected
				}
				m.nats = selected
				m.phase = phaseAwaitingApproval
			}
			return m, nil
		}

		switch msg.String() {
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
				m.enterPhaseDone()
				return m, nil
			}
		case "m", "M":
			if m.phase == phaseDone && m.datahubPhase == 0 {
				m.exportReport("markdown")
				return m, nil
			}
		case "j", "J":
			if m.phase == phaseDone && m.datahubPhase == 0 {
				m.exportReport("json")
				return m, nil
			}
		case "d", "D":
			if m.phase == phaseDone && m.datahubPhase == 0 {
				if m.datahubAPIKey != "" {
					m.datahubPhase = 4
					return m, m.sendToDataHub
				}
				m.datahubPhase = 1
				m.datahubInputBuf = ""
				m.datahubMsg = ""
				return m, nil
			}
		case "enter", " ":
			if m.phase == phaseDone && m.datahubPhase == 0 {
				return m, tea.Quit
			}
			if m.datahubPhase == 1 {
				if m.datahubInputBuf == "" {
					m.datahubPhase = 0
					m.datahubMsg = "  ✗ No API key provided"
					return m, nil
				}
				m.datahubAPIKey = m.datahubInputBuf
				m.datahubInputBuf = ""
				m.datahubPhase = 2
				return m, nil
			}
			if m.datahubPhase == 2 {
				m.datahubCustomerCtx = m.datahubInputBuf
				m.datahubInputBuf = ""
				m.datahubPhase = 4
				return m, m.sendToDataHub
			}
			if m.datahubPhase == 3 {
				m.datahubPhase = 0
				return m, nil
			}
		case "backspace":
			if m.datahubPhase == 1 || m.datahubPhase == 2 {
				if len(m.datahubInputBuf) > 0 {
					m.datahubInputBuf = m.datahubInputBuf[:len(m.datahubInputBuf)-1]
				}
				return m, nil
			}
		default:
			if m.datahubPhase == 1 || m.datahubPhase == 2 {
				if len(msg.String()) == 1 {
					m.datahubInputBuf += msg.String()
				}
				return m, nil
			}
			if m.datahubPhase == 3 {
				if msg.String() == "n" || msg.String() == "N" {
					m.datahubPhase = 0
					return m, nil
				}
				if msg.String() == "y" || msg.String() == "Y" {
					_ = datahub.SaveConfig(datahub.Config{APIKey: m.datahubAPIKey, CustomerContext: m.datahubCustomerCtx})
					m.datahubMsg += "\n  ✓ Saved to ~/.terminat/config.toml"
					m.datahubPhase = 0
					return m, nil
				}
			}
		}
		// Forward to viewport for scrolling
		if m.phase == phaseDone && m.viewportReady && m.datahubPhase == 0 {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		footerHeight := 6
		m.viewport = viewport.New(msg.Width, msg.Height-footerHeight)
		m.viewportReady = true
		if m.phase == phaseDone {
			m.viewport.SetContent(m.renderReportBody())
		}
		return m, nil

	case tea.MouseMsg:
		if m.phase == phaseDone && m.viewportReady {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
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
		// Show interactive selection when multiple NATs and no explicit filter
		if len(m.nats) > 1 && len(m.natIDs) == 0 {
			m.natSelected = make(map[int]bool)
			for i := range m.nats {
				m.natSelected[i] = true // select all by default
			}
			m.phase = phaseSelectingNATs
			return m, nil
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
		if m.exportFormat != "" {
			m.exportReport(m.exportFormat)
		}
		m.enterPhaseDone()
		return m, nil

	case datahubResultMsg:
		if msg.err != nil {
			m.datahubMsg = fmt.Sprintf("  ✗ DataHub error: %v", msg.err)
			m.datahubPhase = 0
		} else {
			m.datahubMsg = "  ✓ Sent to DoiT DataHub"
			cfg := datahub.LoadConfig()
			if cfg.APIKey != m.datahubAPIKey {
				m.datahubPhase = 3
			} else {
				m.datahubPhase = 0
			}
		}
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
	case phaseSelectingNATs:
		b.WriteString(m.renderNATSelection())
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
		if m.viewportReady {
			b.WriteString(m.viewport.View())
			b.WriteString("\n")
			b.WriteString(m.renderFooter())
		} else {
			b.WriteString(m.renderFinalReport())
		}
	}

	return b.String()
}

func (m *deepScanModel) renderNATSelection() string {
	var b strings.Builder
	b.WriteString(stepStyle.Render("Select NAT Gateways to deep scan:") + "\n\n")

	// Group by VPC for display
	for i, nat := range m.nats {
		cursor := "  "
		if i == m.natCursor {
			cursor = highlightStyle.Render("> ")
		}
		check := "[ ]"
		if m.natSelected[i] {
			check = successStyle.Render("[✓]")
		}
		mode := nat.AvailabilityMode
		if mode == "" {
			mode = "zonal"
		}
		name := nat.Tags["Name"]
		label := fmt.Sprintf("%s (%s, VPC: %s)", nat.ID, mode, nat.VPCID)
		if name != "" {
			label = fmt.Sprintf("%s - %s (%s, VPC: %s)", nat.ID, name, mode, nat.VPCID)
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, check, label))
	}

	b.WriteString("\n")
	b.WriteString(tipStyle.Render("↑/↓ move  ␣ toggle  a select all  enter confirm") + "\n")
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

func (m *deepScanModel) discoverNATs() tea.Msg {
	nats, err := m.scanner.DiscoverNATGateways(m.ctx)
	if err != nil {
		return deepScanErrorMsg{err: err}
	}

	// Filter by --vpc-id
	if m.vpcID != "" {
		filtered := []types.NATGateway{}
		for _, nat := range nats {
			if nat.VPCID == m.vpcID {
				filtered = append(filtered, nat)
			}
		}
		nats = filtered
	}

	// Filter by --nat-gateway-ids
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
	return p.Sprintf("$%.2f", amount)
}

var (
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Bold(true)
	highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
	tipStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true)
)
