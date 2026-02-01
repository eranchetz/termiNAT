package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/doitintl/terminator/internal/core"
	"github.com/doitintl/terminator/ui"
	"github.com/spf13/cobra"
)

var (
	region      string
	duration    int
	natIDs      []string
	autoApprove bool
	autoCleanup bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for NAT Gateway optimization opportunities",
	Long:  `Scan AWS infrastructure to identify services using NAT Gateway that could use VPC endpoints instead.`,
}

var quickCmd = &cobra.Command{
	Use:   "quick",
	Short: "Quick scan without Flow Logs (configuration-only)",
	Long: `Performs a quick configuration scan to identify missing or misconfigured 
VPC endpoints without enabling Flow Logs. Fast and cost-free.`,
	RunE: runQuickScan,
}

var deepCmd = &cobra.Command{
	Use:   "deep",
	Short: "Deep dive analysis with Flow Logs",
	Long: `Enables short-lived Flow Logs to quantify actual NAT traffic to AWS services 
and calculate potential savings. Requires Flow Log permissions.`,
	RunE: runDeepScan,
}

func init() {
	scanCmd.AddCommand(quickCmd)
	scanCmd.AddCommand(deepCmd)

	// Common flags - region is optional if AWS_REGION is set
	scanCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region (uses AWS_REGION env var if not specified)")

	// Deep scan specific flags
	deepCmd.Flags().IntVarP(&duration, "duration", "d", 15, "Flow Log collection duration in minutes (max 60)")
	deepCmd.Flags().StringSliceVar(&natIDs, "nat-gateway-ids", []string{}, "Specific NAT Gateway IDs to analyze (optional)")
	deepCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip approval prompts (for automation)")
	deepCmd.Flags().BoolVar(&autoCleanup, "auto-cleanup", false, "Automatically delete log groups after scan")
}

func getRegion() (string, error) {
	// Use flag value if provided
	if region != "" {
		return region, nil
	}

	// Fall back to AWS_REGION environment variable
	if envRegion := os.Getenv("AWS_REGION"); envRegion != "" {
		return envRegion, nil
	}

	return "", fmt.Errorf("region not specified: use --region flag or set AWS_REGION environment variable")
}

func runQuickScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get region from flag or environment
	selectedRegion, err := getRegion()
	if err != nil {
		return err
	}

	// Create scanner
	scanner, err := core.NewScanner(ctx, selectedRegion)
	if err != nil {
		return fmt.Errorf("failed to create scanner: %w", err)
	}

	// Run quick scan with UI
	return ui.RunQuickScan(ctx, scanner)
}

func runDeepScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate duration
	if duration < 5 || duration > 60 {
		return fmt.Errorf("duration must be between 5 and 60 minutes")
	}

	// Get region from flag or environment
	selectedRegion, err := getRegion()
	if err != nil {
		return err
	}

	// Create scanner
	scanner, err := core.NewScanner(ctx, selectedRegion)
	if err != nil {
		return fmt.Errorf("failed to create scanner: %w", err)
	}

	// Run deep scan with UI
	return ui.RunDeepScan(ctx, scanner, selectedRegion, duration, natIDs, autoApprove, autoCleanup)
}
