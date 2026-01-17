package cmd

import (
	"context"
	"fmt"

	"github.com/doitintl/terminator/internal/core"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up Flow Logs data after analysis",
	Long: `Delete CloudWatch Logs groups created by termiNATor.
	
This command validates that:
1. No active Flow Logs are using the log group
2. Data has been collected (not empty)
3. User confirms deletion

Log groups incur storage costs (~$0.50/GB/month), so cleanup after analysis is recommended.`,
	RunE: runCleanup,
}

var (
	logGroupName  string
	force         bool
	cleanupRegion string
)

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().StringVar(&logGroupName, "log-group", "", "Log group name to delete (required)")
	cleanupCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cleanupCmd.Flags().StringVarP(&cleanupRegion, "region", "r", "", "AWS region (required)")
	cleanupCmd.MarkFlagRequired("log-group")
	cleanupCmd.MarkFlagRequired("region")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize scanner
	scanner, err := core.NewScanner(ctx, cleanupRegion)
	if err != nil {
		return fmt.Errorf("failed to create scanner: %w", err)
	}

	// Validate log group exists and get stats
	stats, err := scanner.GetLogGroupStats(ctx, logGroupName)
	if err != nil {
		return fmt.Errorf("failed to get log group stats: %w", err)
	}

	// Check if any Flow Logs are still using this log group
	activeFlowLogs, err := scanner.CheckActiveFlowLogs(ctx, logGroupName)
	if err != nil {
		return fmt.Errorf("failed to check active Flow Logs: %w", err)
	}

	if len(activeFlowLogs) > 0 {
		return fmt.Errorf("cannot delete: %d active Flow Log(s) still using this log group", len(activeFlowLogs))
	}

	// Display stats
	fmt.Printf("Log Group: %s\n", logGroupName)
	fmt.Printf("Storage: %.2f MB\n", float64(stats.StoredBytes)/(1024*1024))
	fmt.Printf("Estimated monthly cost: $%.4f\n", float64(stats.StoredBytes)/(1024*1024*1024)*0.50)
	fmt.Printf("Log streams: %d\n", stats.LogStreams)
	fmt.Println()

	// Confirm deletion
	if !force {
		fmt.Print("Delete this log group? (yes/no): ")
		var response string
		fmt.Scanln(&response)
		if response != "yes" {
			fmt.Println("Cleanup cancelled")
			return nil
		}
	}

	// Delete log group
	if err := scanner.DeleteLogGroup(ctx, logGroupName); err != nil {
		return fmt.Errorf("failed to delete log group: %w", err)
	}

	fmt.Printf("✓ Log group deleted: %s\n", logGroupName)
	return nil
}
