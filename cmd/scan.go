package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/doitintl/terminator/internal/core"
	"github.com/doitintl/terminator/ui"
	"github.com/spf13/cobra"
)

var (
	region       string
	profile      string
	duration     int
	natIDs       []string
	autoApprove  bool
	autoCleanup  bool
	exportFormat string
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

	// Common flags
	scanCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region (uses AWS_REGION env var if not specified)")
	scanCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile (uses AWS_PROFILE env var if not specified)")

	// Deep scan specific flags
	deepCmd.Flags().IntVarP(&duration, "duration", "d", 15, "Flow Log collection duration in minutes (max 60)")
	deepCmd.Flags().StringSliceVar(&natIDs, "nat-gateway-ids", []string{}, "Specific NAT Gateway IDs to analyze (optional)")
	deepCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip approval prompts (for automation)")
	deepCmd.Flags().BoolVar(&autoCleanup, "auto-cleanup", false, "Automatically delete log groups after scan")
	deepCmd.Flags().StringVarP(&exportFormat, "export", "e", "", "Export report format: markdown, json")
}

func getRegion(profile string) (string, error) {
	// Use flag value if provided
	if region != "" {
		return region, nil
	}

	// Fall back to AWS_REGION environment variable
	if envRegion := os.Getenv("AWS_REGION"); envRegion != "" {
		return envRegion, nil
	}

	// Try to get region from AWS config for the profile
	if profile != "" {
		cfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithSharedConfigProfile(profile),
		)
		if err == nil && cfg.Region != "" {
			fmt.Fprintf(os.Stderr, "ℹ️  Using region '%s' from profile '%s'\n", cfg.Region, profile)
			return cfg.Region, nil
		}
	}

	return "", fmt.Errorf("region not specified: use --region flag or set AWS_REGION environment variable")
}

func getProfile() string {
	// Use flag value if provided
	if profile != "" {
		return profile
	}

	// Fall back to AWS_PROFILE environment variable
	return os.Getenv("AWS_PROFILE")
}

func printAuthHelp(err error) {
	fmt.Fprintf(os.Stderr, "\n❌ Authentication failed: %v\n\n", err)
	fmt.Fprintln(os.Stderr, "🔐 AWS Authentication Guide:")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Option 1: AWS SSO")
	fmt.Fprintln(os.Stderr, "  aws sso login --profile your-profile")
	fmt.Fprintln(os.Stderr, "  terminat scan quick --region us-east-1 --profile your-profile")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Option 2: Environment Variables")
	fmt.Fprintln(os.Stderr, "  export AWS_PROFILE=your-profile")
	fmt.Fprintln(os.Stderr, "  export AWS_REGION=us-east-1")
	fmt.Fprintln(os.Stderr, "  terminat scan quick")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Option 3: AWS Access Keys")
	fmt.Fprintln(os.Stderr, "  export AWS_ACCESS_KEY_ID=your-key")
	fmt.Fprintln(os.Stderr, "  export AWS_SECRET_ACCESS_KEY=your-secret")
	fmt.Fprintln(os.Stderr, "  terminat scan quick --region us-east-1")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Option 4: AWS CLI Configuration")
	fmt.Fprintln(os.Stderr, "  aws configure")
	fmt.Fprintln(os.Stderr, "  terminat scan quick --region us-east-1")
	fmt.Fprintln(os.Stderr, "")
}

func runQuickScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get profile from flag or environment (optional)
	selectedProfile := getProfile()

	// Get region from flag, environment, or profile config
	selectedRegion, err := getRegion(selectedProfile)
	if err != nil {
		return err
	}

	// Create scanner - this validates credentials
	scanner, err := core.NewScanner(ctx, selectedRegion, selectedProfile)
	if err != nil {
		printAuthHelp(err)
		return fmt.Errorf("failed to create scanner")
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

	// Get profile from flag or environment (optional)
	selectedProfile := getProfile()

	// Get region from flag, environment, or profile config
	selectedRegion, err := getRegion(selectedProfile)
	if err != nil {
		return err
	}

	// Create scanner - this validates credentials
	scanner, err := core.NewScanner(ctx, selectedRegion, selectedProfile)
	if err != nil {
		printAuthHelp(err)
		return fmt.Errorf("failed to create scanner")
	}

	// Run deep scan with UI
	return ui.RunDeepScan(ctx, scanner, selectedRegion, duration, natIDs, autoApprove, autoCleanup, exportFormat)
}
