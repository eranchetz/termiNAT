package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/doitintl/terminator/internal/core"
	"github.com/doitintl/terminator/pkg/types"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D7D7D"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)
)

type quickScanModel struct {
	scanner  *core.Scanner
	ctx      context.Context
	spinner  spinner.Model
	step     string
	nats     []types.NATGateway
	findings []types.Finding
	err      error
	done     bool
}

type scanStepMsg struct {
	step string
}

type natsDiscoveredMsg struct {
	nats []types.NATGateway
}

type findingsMsg struct {
	findings []types.Finding
}

type scanErrorMsg struct {
	err error
}

type scanCompleteMsg struct{}

func RunQuickScan(ctx context.Context, scanner *core.Scanner) error {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	m := quickScanModel{
		scanner: scanner,
		ctx:     ctx,
		spinner: s,
		step:    "Initializing...",
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func (m quickScanModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.discoverNATs,
	)
}

func (m quickScanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		if m.done && (msg.String() == "enter" || msg.String() == " ") {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanStepMsg:
		m.step = msg.step
		return m, nil

	case natsDiscoveredMsg:
		m.nats = msg.nats
		return m, m.analyzeConfiguration

	case findingsMsg:
		m.findings = msg.findings
		return m, m.complete

	case scanErrorMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit

	case scanCompleteMsg:
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m quickScanModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("termiNATor - Quick Scan"))
	b.WriteString("\n\n")

	if !m.done {
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), stepStyle.Render(m.step)))
	} else {
		b.WriteString(successStyle.Render("✓ Scan Complete\n\n"))
		b.WriteString(m.renderResults())
		b.WriteString("\n\n")
		b.WriteString(infoStyle.Render("Press Enter to exit"))
	}

	return b.String()
}

func (m quickScanModel) renderResults() string {
	var b strings.Builder

	b.WriteString(stepStyle.Render(fmt.Sprintf("Found %d NAT Gateway(s)\n\n", len(m.nats))))

	for _, nat := range m.nats {
		b.WriteString(fmt.Sprintf("  • %s (%s, %s)\n", nat.ID, nat.AvailabilityMode, nat.State))
		b.WriteString(fmt.Sprintf("    VPC: %s\n", nat.VPCID))
	}

	b.WriteString("\n")
	b.WriteString(stepStyle.Render(fmt.Sprintf("Findings: %d\n\n", len(m.findings))))

	if len(m.findings) == 0 {
		b.WriteString(successStyle.Render("  ✓ No issues found! All VPCs have proper endpoint configuration.\n"))
	} else {
		for _, finding := range m.findings {
			severity := finding.Severity
			if severity == "high" {
				severity = errorStyle.Render("HIGH")
			}
			b.WriteString(fmt.Sprintf("  [%s] %s\n", severity, finding.Title))
			b.WriteString(fmt.Sprintf("    %s\n", finding.Description))
			b.WriteString(fmt.Sprintf("    Action: %s\n\n", finding.Action))
		}
	}

	return b.String()
}

func (m quickScanModel) discoverNATs() tea.Msg {
	m.step = "Discovering NAT Gateways..."

	nats, err := m.scanner.DiscoverNATGateways(m.ctx)
	if err != nil {
		return scanErrorMsg{err: err}
	}

	return natsDiscoveredMsg{nats: nats}
}

func (m quickScanModel) analyzeConfiguration() tea.Msg {
	m.step = "Analyzing VPC endpoint configuration..."

	var findings []types.Finding

	// Group NATs by VPC
	vpcNATs := make(map[string][]types.NATGateway)
	for _, nat := range m.nats {
		vpcNATs[nat.VPCID] = append(vpcNATs[nat.VPCID], nat)
	}

	// Check each VPC for missing endpoints
	for vpcID := range vpcNATs {
		endpoints, err := m.scanner.DiscoverVPCEndpoints(m.ctx, vpcID)
		if err != nil {
			return scanErrorMsg{err: err}
		}

		routeTables, err := m.scanner.DiscoverRouteTables(m.ctx, vpcID)
		if err != nil {
			return scanErrorMsg{err: err}
		}

		// Check for S3 gateway endpoint
		hasS3Gateway := false
		s3EndpointRTs := []string{}
		for _, ep := range endpoints {
			if strings.Contains(ep.ServiceName, ".s3") && ep.Type == "Gateway" {
				hasS3Gateway = true
				s3EndpointRTs = ep.RouteTables
				break
			}
		}

		if !hasS3Gateway {
			findings = append(findings, types.Finding{
				Type:        "missing-endpoint",
				Severity:    "high",
				Title:       "Missing S3 Gateway Endpoint",
				Description: fmt.Sprintf("VPC %s has NAT Gateway(s) but no S3 Gateway endpoint", vpcID),
				VPCID:       vpcID,
				Service:     "S3",
				Action:      "Create S3 Gateway VPC endpoint and associate with private route tables",
				Impact:      "All S3 traffic is going through NAT Gateway, incurring $0.045/GB data processing charges",
			})
		} else {
			// Check if endpoint is associated with route tables that route to NAT
			natRouteTables := []string{}
			for _, rt := range routeTables {
				for _, route := range rt.Routes {
					if route.TargetType == "nat-gateway" && route.DestinationCIDR == "0.0.0.0/0" {
						natRouteTables = append(natRouteTables, rt.ID)
						break
					}
				}
			}

			// Check if S3 endpoint is associated with NAT route tables
			missingAssociations := []string{}
			for _, rtID := range natRouteTables {
				found := false
				for _, epRT := range s3EndpointRTs {
					if epRT == rtID {
						found = true
						break
					}
				}
				if !found {
					missingAssociations = append(missingAssociations, rtID)
				}
			}

			if len(missingAssociations) > 0 {
				findings = append(findings, types.Finding{
					Type:        "misconfigured-endpoint",
					Severity:    "high",
					Title:       "S3 Gateway Endpoint Not Associated with NAT Route Tables",
					Description: fmt.Sprintf("VPC %s has S3 endpoint but it's not associated with %d route table(s) that route to NAT", vpcID, len(missingAssociations)),
					VPCID:       vpcID,
					Service:     "S3",
					Action:      fmt.Sprintf("Associate S3 endpoint with route tables: %s", strings.Join(missingAssociations, ", ")),
					Impact:      "S3 traffic from some subnets is still going through NAT Gateway",
				})
			}
		}

		// Check for DynamoDB gateway endpoint
		hasDDBGateway := false
		ddbEndpointRTs := []string{}
		for _, ep := range endpoints {
			if strings.Contains(ep.ServiceName, ".dynamodb") && ep.Type == "Gateway" {
				hasDDBGateway = true
				ddbEndpointRTs = ep.RouteTables
				break
			}
		}

		if !hasDDBGateway {
			findings = append(findings, types.Finding{
				Type:        "missing-endpoint",
				Severity:    "high",
				Title:       "Missing DynamoDB Gateway Endpoint",
				Description: fmt.Sprintf("VPC %s has NAT Gateway(s) but no DynamoDB Gateway endpoint", vpcID),
				VPCID:       vpcID,
				Service:     "DynamoDB",
				Action:      "Create DynamoDB Gateway VPC endpoint and associate with private route tables",
				Impact:      "All DynamoDB traffic is going through NAT Gateway, incurring $0.045/GB data processing charges",
			})
		} else {
			// Similar check for DynamoDB endpoint associations
			natRouteTables := []string{}
			for _, rt := range routeTables {
				for _, route := range rt.Routes {
					if route.TargetType == "nat-gateway" && route.DestinationCIDR == "0.0.0.0/0" {
						natRouteTables = append(natRouteTables, rt.ID)
						break
					}
				}
			}

			missingAssociations := []string{}
			for _, rtID := range natRouteTables {
				found := false
				for _, epRT := range ddbEndpointRTs {
					if epRT == rtID {
						found = true
						break
					}
				}
				if !found {
					missingAssociations = append(missingAssociations, rtID)
				}
			}

			if len(missingAssociations) > 0 {
				findings = append(findings, types.Finding{
					Type:        "misconfigured-endpoint",
					Severity:    "high",
					Title:       "DynamoDB Gateway Endpoint Not Associated with NAT Route Tables",
					Description: fmt.Sprintf("VPC %s has DynamoDB endpoint but it's not associated with %d route table(s) that route to NAT", vpcID, len(missingAssociations)),
					VPCID:       vpcID,
					Service:     "DynamoDB",
					Action:      fmt.Sprintf("Associate DynamoDB endpoint with route tables: %s", strings.Join(missingAssociations, ", ")),
					Impact:      "DynamoDB traffic from some subnets is still going through NAT Gateway",
				})
			}
		}
	}

	return findingsMsg{findings: findings}
}

func (m quickScanModel) complete() tea.Msg {
	return scanCompleteMsg{}
}
