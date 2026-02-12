package ui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/internal/core"
	"github.com/doitintl/terminator/internal/datahub"
	"github.com/doitintl/terminator/internal/report"
	"github.com/doitintl/terminator/pkg/types"
)

type streamDeepScanRunner struct {
	ctx                context.Context
	scanner            *core.Scanner
	region             string
	duration           int
	natIDs             []string
	vpcID              string
	autoApprove        bool
	autoCleanup        bool
	exportFormat       string
	outputFile         string
	datahubAPIKey      string
	datahubCustomerCtx string
	interactive        bool
	reader             *bufio.Reader
	startedAt          time.Time
	runID              string
	logGroupName       string
	outputWidth        int

	nats                 []types.NATGateway
	flowLogIDs           []string
	flowLogsStopped      bool
	estimatedScanCostGB  float64
	estimatedScanCostUSD float64
	recommendations      []analysis.Recommendation
	trafficStats         *analysis.TrafficStats
	costEstimate         *analysis.CostEstimate
	endpointAnalysis     *analysis.EndpointAnalysis
	allFindings          []types.Finding
	deepScannedVPC       string
}

func RunDeepScanStream(ctx context.Context, scanner *core.Scanner, region string, duration int, natIDs []string, vpcID string, autoApprove, autoCleanup bool, exportFormat, outputFile string, datahubAPIKey, datahubCustomerCtx string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := &streamDeepScanRunner{
		ctx:                ctx,
		scanner:            scanner,
		region:             region,
		duration:           duration,
		natIDs:             natIDs,
		vpcID:              vpcID,
		autoApprove:        autoApprove,
		autoCleanup:        autoCleanup,
		exportFormat:       strings.ToLower(strings.TrimSpace(exportFormat)),
		outputFile:         outputFile,
		datahubAPIKey:      datahub.ResolveAPIKey(datahubAPIKey),
		datahubCustomerCtx: datahub.ResolveCustomerContext(datahubCustomerCtx),
		interactive:        isTerminal(os.Stdin),
		reader:             bufio.NewReader(os.Stdin),
		startedAt:          time.Now(),
		runID:              fmt.Sprintf("terminat-%d", time.Now().Unix()),
		logGroupName:       fmt.Sprintf("/aws/vpc/flowlogs/terminat-%d", time.Now().Unix()),
		outputWidth:        detectOutputWidth(os.Stdout),
	}
	return r.run()
}

func (r *streamDeepScanRunner) run() error {
	r.logStage("scan", "Deep scan started (region=%s account=%s duration=%dm ui=stream)", r.region, r.scanner.GetAccountID(), r.duration)

	if !r.autoApprove && !r.interactive {
		return fmt.Errorf("--ui stream requires a TTY for prompts unless --auto-approve is set")
	}

	if err := r.discoverNATs(); err != nil {
		return err
	}

	if len(r.nats) > 1 && len(r.natIDs) == 0 && !r.autoApprove {
		selected, err := r.promptNATSelection()
		if err != nil {
			return err
		}
		r.nats = selected
	}

	if !r.autoApprove {
		approved, err := r.promptFlowLogsApproval()
		if err != nil {
			return err
		}
		if !approved {
			r.logStage("scan", "Cancelled by user before resource creation")
			return nil
		}
	}

	if err := r.createFlowLogs(); err != nil {
		return err
	}

	defer func() {
		if len(r.flowLogIDs) == 0 || r.flowLogsStopped {
			return
		}
		if err := r.stopFlowLogs(); err != nil {
			r.logStage("warn", "Failed to stop Flow Logs during deferred cleanup: %v", err)
		}
	}()

	if err := r.waitForFlowLogsStartup(); err != nil {
		return err
	}

	if err := r.collectTraffic(); err != nil {
		return err
	}

	if err := r.analyzeTraffic(); err != nil {
		return err
	}

	if err := r.stopFlowLogs(); err != nil {
		return err
	}

	if err := r.handleLogGroupCleanup(); err != nil {
		return err
	}

	r.renderFinalSummary()

	if err := r.exportIfRequested(); err != nil {
		return err
	}

	if err := r.sendDataHubIfConfigured(); err != nil {
		return err
	}

	r.logStage("scan", "Completed in %s", formatDuration(time.Since(r.startedAt)))
	return nil
}

func (r *streamDeepScanRunner) discoverNATs() error {
	r.logStage("discover", "Discovering NAT Gateways")
	nats, err := r.scanner.DiscoverNATGateways(r.ctx)
	if err != nil {
		return err
	}

	if r.vpcID != "" {
		filtered := make([]types.NATGateway, 0, len(nats))
		for _, nat := range nats {
			if nat.VPCID == r.vpcID {
				filtered = append(filtered, nat)
			}
		}
		nats = filtered
	}

	if len(r.natIDs) > 0 {
		byID := make(map[string]types.NATGateway, len(nats))
		for _, nat := range nats {
			byID[nat.ID] = nat
		}
		filtered := make([]types.NATGateway, 0, len(r.natIDs))
		for _, id := range r.natIDs {
			if nat, ok := byID[id]; ok {
				filtered = append(filtered, nat)
			}
		}
		nats = filtered
	}

	if len(nats) == 0 {
		return fmt.Errorf("no NAT gateways found")
	}

	r.nats = nats
	r.recommendations = analysis.AnalyzeNATGatewaySetup(nats)

	natIDs := make([]string, 0, len(nats))
	for _, nat := range nats {
		natIDs = append(natIDs, nat.ID)
	}
	estGB, estCost, _ := r.scanner.EstimateFlowLogsCost(r.ctx, natIDs, r.duration)
	r.estimatedScanCostGB = estGB
	r.estimatedScanCostUSD = estCost

	r.logStage("discover", "Found %d NAT Gateway(s)", len(r.nats))
	for _, nat := range r.nats {
		mode := nat.AvailabilityMode
		if mode == "" {
			mode = "zonal"
		}
		r.logLine("  - %s (%s, vpc=%s)", nat.ID, mode, nat.VPCID)
	}
	return nil
}

func (r *streamDeepScanRunner) promptNATSelection() ([]types.NATGateway, error) {
	r.logLine("")
	r.logLine("Multiple NAT Gateways found. Select which to deep scan:")
	for i, nat := range r.nats {
		mode := nat.AvailabilityMode
		if mode == "" {
			mode = "zonal"
		}
		name := nat.Tags["Name"]
		if name == "" {
			r.logLine("  %d) %s (%s, vpc=%s)", i+1, nat.ID, mode, nat.VPCID)
			continue
		}
		r.logLine("  %d) %s (%s) (%s, vpc=%s)", i+1, nat.ID, name, mode, nat.VPCID)
	}

	r.logLine("Enter comma-separated indexes or press Enter for all")
	input, err := r.prompt("Selection [all]: ")
	if err != nil {
		return nil, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" || input == "all" {
		return r.nats, nil
	}

	seen := map[int]struct{}{}
	selected := make([]types.NATGateway, 0, len(r.nats))
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		idx, err := strconv.Atoi(part)
		if err != nil || idx < 1 || idx > len(r.nats) {
			return nil, fmt.Errorf("invalid NAT selection %q", part)
		}
		zeroBased := idx - 1
		if _, exists := seen[zeroBased]; exists {
			continue
		}
		seen[zeroBased] = struct{}{}
		selected = append(selected, r.nats[zeroBased])
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("at least one NAT Gateway must be selected")
	}
	r.logStage("discover", "Selected %d NAT Gateway(s) for deep scan", len(selected))
	return selected, nil
}

func (r *streamDeepScanRunner) promptFlowLogsApproval() (bool, error) {
	r.logLine("")
	r.logLine("Resource creation summary:")
	r.logLine("  - Temporary VPC Flow Logs on selected NAT Gateways")
	r.logLine("  - CloudWatch Log Group: %s", r.logGroupName)
	if r.estimatedScanCostGB > 0 {
		r.logLine("  - Estimated ingestion: %.2f GB (~$%.2f)", r.estimatedScanCostGB, r.estimatedScanCostUSD)
	} else {
		r.logLine("  - Estimated ingestion cost: ~$0.50 per GB")
	}
	r.logLine("  - Total scan time estimate: %d minutes (%d startup + %d collection)", r.duration+5, 5, r.duration)
	return r.confirm("Proceed with scan?", true)
}

func (r *streamDeepScanRunner) createFlowLogs() error {
	r.logStage("setup", "Validating IAM role and creating Flow Logs resources")
	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/termiNATor-FlowLogsRole", r.scanner.GetAccountID())

	if err := r.scanner.ValidateFlowLogsRole(r.ctx, roleARN); err != nil {
		return err
	}
	if err := r.scanner.CreateLogGroup(r.ctx, r.logGroupName); err != nil {
		return fmt.Errorf("failed to create log group: %w", err)
	}

	for _, nat := range r.nats {
		flowLogID, err := r.scanner.CreateFlowLogs(r.ctx, nat, r.logGroupName, roleARN, r.runID)
		if err != nil {
			if len(r.flowLogIDs) > 0 {
				_ = r.scanner.DeleteFlowLogs(r.ctx, r.flowLogIDs)
			}
			_ = r.scanner.DeleteLogGroup(r.ctx, r.logGroupName)
			return fmt.Errorf("failed to create flow logs: %w", err)
		}
		r.flowLogIDs = append(r.flowLogIDs, flowLogID)
	}

	r.logStage("setup", "Created %d Flow Log(s) in %s", len(r.flowLogIDs), r.logGroupName)
	return nil
}

func (r *streamDeepScanRunner) waitForFlowLogsStartup() error {
	r.logStage("startup", "Waiting for Flow Logs to become ACTIVE")
	timeout := 10 * time.Minute
	pollInterval := 30 * time.Second
	deadline := time.Now().Add(timeout)
	started := time.Now()

	for time.Now().Before(deadline) {
		activeFlowLogs, err := r.scanner.CheckActiveFlowLogs(r.ctx, r.logGroupName)
		if err == nil && len(activeFlowLogs) > 0 {
			r.logStage("startup", "Flow Logs are ACTIVE after %s", formatDuration(time.Since(started)))
			return nil
		}
		r.logLine("  startup progress: elapsed=%s", formatDuration(time.Since(started)))

		select {
		case <-r.ctx.Done():
			return fmt.Errorf("scan cancelled during Flow Logs startup")
		case <-time.After(pollInterval):
		}
	}
	return fmt.Errorf("timeout waiting for Flow Logs to become ACTIVE after %s", timeout)
}

func (r *streamDeepScanRunner) collectTraffic() error {
	r.logStage("collect", "Collecting traffic for %d minute(s)", r.duration)
	total := time.Duration(r.duration) * time.Minute
	started := time.Now()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	timer := time.NewTimer(total)
	defer timer.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return fmt.Errorf("scan cancelled during traffic collection")
		case <-ticker.C:
			elapsed := time.Since(started)
			if elapsed > total {
				elapsed = total
			}
			remaining := total - elapsed
			if remaining < 0 {
				remaining = 0
			}
			progress := (elapsed.Seconds() / total.Seconds()) * 100
			r.logLine("  collection progress: %5.1f%% elapsed=%s remaining=%s", progress, formatDuration(elapsed), formatDuration(remaining))
		case <-timer.C:
			r.logStage("collect", "Traffic collection completed")
			return nil
		}
	}
}

func (r *streamDeepScanRunner) analyzeTraffic() error {
	r.logStage("analyze", "Querying Flow Logs and classifying traffic")
	endTime := time.Now().Unix()
	startTime := endTime - int64(r.duration*60) - 300

	stats, err := r.scanner.AnalyzeTraffic(r.ctx, r.logGroupName, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to analyze traffic: %w", err)
	}
	r.trafficStats = stats
	r.costEstimate = r.scanner.CalculateCosts(stats, r.duration)

	if len(r.nats) > 0 {
		r.deepScannedVPC = r.nats[0].VPCID
		r.endpointAnalysis, _ = r.scanner.AnalyzeVPCEndpoints(r.ctx, r.deepScannedVPC)
	}
	r.allFindings = analysis.AnalyzeAllVPCEndpoints(r.ctx, r.scanner, r.nats)

	r.logStage("analyze", "Analysis complete: records=%d total=%.2fGB", stats.TotalRecords, float64(stats.TotalBytes)/(1024*1024*1024))
	return nil
}

func (r *streamDeepScanRunner) stopFlowLogs() error {
	if len(r.flowLogIDs) == 0 || r.flowLogsStopped {
		return nil
	}
	r.logStage("cleanup", "Stopping Flow Logs")
	if err := r.scanner.DeleteFlowLogs(r.ctx, r.flowLogIDs); err != nil {
		return fmt.Errorf("failed to stop flow logs: %w", err)
	}
	r.flowLogsStopped = true
	r.logStage("cleanup", "Flow Logs stopped")
	return nil
}

func (r *streamDeepScanRunner) handleLogGroupCleanup() error {
	deleteLogGroup := r.autoCleanup
	if !r.autoApprove {
		answer, err := r.confirm(fmt.Sprintf("Delete CloudWatch Log Group %s?", r.logGroupName), true)
		if err != nil {
			return err
		}
		deleteLogGroup = answer
	}
	if !deleteLogGroup {
		r.logStage("cleanup", "Keeping log group: %s", r.logGroupName)
		return nil
	}

	if err := r.scanner.DeleteLogGroup(r.ctx, r.logGroupName); err != nil {
		return fmt.Errorf("failed to delete log group: %w", err)
	}
	r.logStage("cleanup", "Deleted log group: %s", r.logGroupName)
	return nil
}

func (r *streamDeepScanRunner) renderFinalSummary() {
	r.logLine("")
	r.logLine("========== DEEP SCAN REPORT ==========")

	r.logLine("NAT Gateways")
	for _, nat := range r.nats {
		mode := nat.AvailabilityMode
		if mode == "" {
			mode = "zonal"
		}
		r.logLine("  - %s (%s, vpc=%s)", nat.ID, mode, nat.VPCID)
	}

	if len(r.allFindings) == 0 {
		r.logLine("\nEndpoint Findings")
		r.logLine("  - No endpoint issues found across scanned VPCs")
	} else {
		r.logLine("\nEndpoint Findings (%d)", len(r.allFindings))
		for _, finding := range r.allFindings {
			r.logLine("  - [%s] %s", strings.ToUpper(finding.Severity), finding.Title)
			r.logLine("    %s", finding.Description)
			r.logLine("    Action: %s", finding.Action)
		}
	}

	if r.trafficStats != nil && r.trafficStats.TotalRecords > 0 {
		totalGB := float64(r.trafficStats.TotalBytes) / (1024 * 1024 * 1024)
		r.logLine("\nTraffic Sample")
		r.logLine("  - Duration: %d minute(s)", r.duration)
		r.logLine("  - Total: %d records, %.2f GB", r.trafficStats.TotalRecords, totalGB)
		r.logLine("  - S3: %.2f GB (%.1f%%)", float64(r.trafficStats.S3Bytes)/(1024*1024*1024), r.trafficStats.S3Percentage())
		r.logLine("  - DynamoDB: %.2f GB (%.1f%%)", float64(r.trafficStats.DynamoBytes)/(1024*1024*1024), r.trafficStats.DynamoPercentage())
		r.logLine("  - ECR: %.2f GB (%.1f%%)", float64(r.trafficStats.ECRBytes)/(1024*1024*1024), r.trafficStats.ECRPercentage())
		r.logLine("  - Other: %.2f GB (%.1f%%)", float64(r.trafficStats.OtherBytes)/(1024*1024*1024), r.trafficStats.OtherPercentage())
	} else {
		r.logLine("\nTraffic Sample")
		r.logLine("  - No traffic records were collected in this run")
	}

	if r.costEstimate != nil {
		r.logLine("\nCost Estimate (projected from sample)")
		r.logLine("  - NAT data processing rate: $%.4f per GB", r.costEstimate.NATGatewayPricePerGB)
		r.logLine("  - Current NAT cost: $%.2f/month", r.costEstimate.CurrentMonthlyCost)
		r.logLine("  - S3 savings potential: $%.2f/month", r.costEstimate.S3SavingsMonthly)
		r.logLine("  - DynamoDB savings potential: $%.2f/month", r.costEstimate.DynamoSavingsMonthly)
		r.logLine("  - Total savings potential: $%.2f/month ($%.2f/year)", r.costEstimate.TotalSavingsMonthly, r.costEstimate.TotalSavingsMonthly*12)
	}

	if r.endpointAnalysis != nil && r.endpointAnalysis.HasIssues() {
		r.logLine("\nRemediation Commands")
		for _, cmd := range r.endpointAnalysis.GetCreateEndpointCommands() {
			r.logLine("  %s", cmd)
		}
		for _, cmd := range r.endpointAnalysis.GetAddRouteCommands() {
			r.logLine("  %s", cmd)
		}
	}

	if len(r.recommendations) > 0 {
		r.logLine("\nRecommendations")
		for i, rec := range r.recommendations {
			r.logLine("  %d. %s [%s]", i+1, rec.Title, strings.ToUpper(rec.Priority))
			r.logLine("     %s", rec.Description)
			if rec.Savings != "" {
				r.logLine("     Savings: %s", rec.Savings)
			}
		}
	}
}

func (r *streamDeepScanRunner) exportIfRequested() error {
	if r.exportFormat == "" {
		return nil
	}

	rep := report.New(r.region, r.scanner.GetAccountID(), r.duration, r.trafficStats, r.costEstimate, r.endpointAnalysis)
	filename := r.outputFile
	if filename == "" {
		timestamp := time.Now().Format("20060102-150405")
		ext := ".md"
		if r.exportFormat == "json" {
			ext = ".json"
		}
		filename = fmt.Sprintf("terminator-report-%s%s", timestamp, ext)
	}

	var err error
	switch r.exportFormat {
	case "markdown":
		err = rep.SaveMarkdown(filename)
	case "json":
		err = rep.SaveJSON(filename)
	default:
		return fmt.Errorf("unsupported export format: %s", r.exportFormat)
	}
	if err != nil {
		return err
	}

	absPath, _ := filepath.Abs(filename)
	if absPath == "" {
		absPath = filename
	}
	r.logStage("export", "Saved %s report: %s", r.exportFormat, absPath)
	return nil
}

func (r *streamDeepScanRunner) sendDataHubIfConfigured() error {
	if r.datahubAPIKey == "" {
		return nil
	}

	r.logStage("datahub", "Sending events to DoiT DataHub")
	events := datahub.BuildEvents(r.scanner.GetAccountID(), r.region, r.nats, r.trafficStats, r.costEstimate, r.endpointAnalysis)
	if err := datahub.Send(r.datahubAPIKey, r.datahubCustomerCtx, events); err != nil {
		return err
	}
	r.logStage("datahub", "Sent %d event(s)", len(events))
	return nil
}

func (r *streamDeepScanRunner) confirm(prompt string, defaultYes bool) (bool, error) {
	defaultText := "y/N"
	if defaultYes {
		defaultText = "Y/n"
	}

	answer, err := r.prompt(fmt.Sprintf("%s [%s]: ", prompt, defaultText))
	if err != nil {
		return false, err
	}
	normalized := strings.ToLower(strings.TrimSpace(answer))
	if normalized == "" {
		return defaultYes, nil
	}
	return normalized == "y" || normalized == "yes", nil
}

func (r *streamDeepScanRunner) prompt(q string) (string, error) {
	fmt.Print(q)
	input, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (r *streamDeepScanRunner) logStage(stage, format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	prefix := fmt.Sprintf("[%s] %-8s ", ts, stage)
	r.printWrapped(prefix, fmt.Sprintf(format, args...))
}

func (r *streamDeepScanRunner) logLine(format string, args ...any) {
	r.printWrapped("", fmt.Sprintf(format, args...))
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func detectOutputWidth(file *os.File) int {
	const defaultWidth = 100
	const minWidth = 60
	const maxWidth = 160

	if file != nil {
		w, _, err := term.GetSize(file.Fd())
		if err == nil && w > 0 {
			if w < minWidth {
				return minWidth
			}
			if w > maxWidth {
				return maxWidth
			}
			return w
		}
	}

	if env := strings.TrimSpace(os.Getenv("COLUMNS")); env != "" {
		if w, err := strconv.Atoi(env); err == nil && w > 0 {
			if w < minWidth {
				return minWidth
			}
			if w > maxWidth {
				return maxWidth
			}
			return w
		}
	}
	return defaultWidth
}

func (r *streamDeepScanRunner) printWrapped(prefix, text string) {
	width := r.outputWidth
	if width <= 0 {
		width = 100
	}

	for _, rawLine := range strings.Split(text, "\n") {
		for i, line := range wrapLine(rawLine, maxInt(20, width-visibleLen(prefix))) {
			if i == 0 {
				fmt.Printf("%s%s\n", prefix, line)
				continue
			}
			fmt.Printf("%s%s\n", strings.Repeat(" ", visibleLen(prefix)), line)
		}
	}
}

func wrapLine(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}

	leading := leadingWhitespace(s)
	content := strings.TrimLeft(s, " \t")
	if content == "" {
		return []string{s}
	}

	if visibleLen(s) <= width {
		return []string{s}
	}

	firstPrefix := leading
	nextPrefix := leading + "  "
	firstLimit := maxInt(10, width-visibleLen(firstPrefix))
	nextLimit := maxInt(10, width-visibleLen(nextPrefix))
	words := strings.Fields(content)
	if len(words) == 0 {
		return []string{s}
	}

	lines := make([]string, 0, 4)
	currentPrefix := firstPrefix
	limit := firstLimit
	current := currentPrefix
	currentLen := visibleLen(currentPrefix)

	flush := func() {
		lines = append(lines, current)
		currentPrefix = nextPrefix
		limit = nextLimit
		current = currentPrefix
		currentLen = visibleLen(currentPrefix)
	}

	for _, word := range words {
		parts := splitWord(word, limit)
		for _, part := range parts {
			partLen := visibleLen(part)
			space := 0
			if currentLen > visibleLen(currentPrefix) {
				space = 1
			}

			if currentLen+space+partLen > limit+visibleLen(currentPrefix) {
				flush()
				space = 0
			}

			if space == 1 {
				current += " "
				currentLen++
			}
			current += part
			currentLen += partLen
		}
	}

	if currentLen > visibleLen(currentPrefix) {
		lines = append(lines, current)
	}
	return lines
}

func splitWord(word string, maxLen int) []string {
	if maxLen <= 0 || visibleLen(word) <= maxLen {
		return []string{word}
	}
	r := []rune(word)
	out := make([]string, 0, (len(r)/maxLen)+1)
	for len(r) > maxLen {
		out = append(out, string(r[:maxLen]))
		r = r[maxLen:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
	}
	return out
}

func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) {
		if s[i] != ' ' && s[i] != '\t' {
			break
		}
		i++
	}
	return s[:i]
}

func visibleLen(s string) int {
	return len([]rune(s))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
