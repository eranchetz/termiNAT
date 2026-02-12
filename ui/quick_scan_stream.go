package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terminator/internal/core"
)

func RunQuickScanStream(ctx context.Context, scanner *core.Scanner) error {
	started := time.Now()
	quickLog("scan", "Quick scan started (region=%s account=%s ui=stream)", scanner.GetRegion(), scanner.GetAccountID())

	quickLog("discover", "Discovering NAT Gateways")
	nats, err := discoverNATsForQuickScan(ctx, scanner)
	if err != nil {
		return err
	}
	quickLog("discover", "Found %d NAT Gateway(s)", len(nats))

	quickLog("analyze", "Analyzing VPC endpoint configuration")
	findings, err := analyzeQuickFindings(ctx, scanner, nats)
	if err != nil {
		return err
	}
	quickLog("analyze", "Analysis complete: findings=%d", len(findings))

	fmt.Println()
	fmt.Println("========== QUICK SCAN REPORT ==========")
	fmt.Printf("NAT Gateways: %d\n", len(nats))
	for _, nat := range nats {
		mode := nat.AvailabilityMode
		if mode == "" {
			mode = "zonal"
		}
		fmt.Printf("  - %s (%s, %s, vpc=%s)\n", nat.ID, mode, nat.State, nat.VPCID)
	}

	fmt.Printf("\nFindings: %d\n", len(findings))
	if len(findings) == 0 {
		fmt.Println("  - No issues found. All VPCs have proper endpoint configuration.")
	} else {
		for _, finding := range findings {
			fmt.Printf("  - [%s] %s\n", finding.Severity, finding.Title)
			fmt.Printf("    %s\n", finding.Description)
			fmt.Printf("    Action: %s\n", finding.Action)
		}
	}

	quickLog("scan", "Completed in %s", formatDuration(time.Since(started)))
	return nil
}

func quickLog(stage, format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %-8s %s\n", ts, stage, msg)
}
