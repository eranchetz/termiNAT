package ui

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/doitintl/terminator/internal/report"
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

type reportData = report.ViewData
type epCostDisplay = report.ViewInterfaceEndpointCost
type sourceIPDisplay = report.ViewSourceIP

func sectionHeader(title string) string {
	line := strings.Repeat("─", 60)
	return stepStyle.Render(line) + "\n" + stepStyle.Render(title) + "\n" + stepStyle.Render(line) + "\n"
}

func (m *deepScanModel) currentReport() *report.Report {
	if m.report != nil {
		return m.report
	}
	return report.NewDetailed(m.region, m.accountID, m.duration, m.nats, m.trafficStats, m.costEstimate, m.endpointAnalysis, m.endpointAnalyses, m.allFindings, m.recommendations, m.selectedVPCIDs, m.logGroupName)
}

func (m *deepScanModel) renderFinalReport() string {
	return m.renderReportBody() + "\n" + m.renderFooter()
}

func (m *deepScanModel) renderReportBody() string {
	data := m.currentReport().ViewData()
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
