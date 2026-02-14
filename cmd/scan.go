package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/doitintl/terminator/internal/core"
	"github.com/doitintl/terminator/ui"
	"github.com/spf13/cobra"
)

var (
	region                 string
	profile                string
	duration               int
	natIDs                 []string
	vpcID                  string
	quickDoctor            bool
	deepDoctor             bool
	deepUIMode             string
	quickUIMode            string
	demoUIMode             string
	autoApprove            bool
	autoCleanup            bool
	exportFormat           string
	outputFile             string
	datahubAPIKey          string
	datahubCustomerContext string
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for NAT Gateway optimization opportunities",
	Long:  `Scan AWS infrastructure to identify services using NAT Gateway that could use VPC endpoints instead.`,
}

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Show a sample report with realistic fake data (no AWS needed)",
	RunE:  runDemoScan,
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
and calculate potential savings. Requires Flow Log permissions.

Examples:
  # Basic deep scan
  terminat scan deep --region us-east-1

  # Export report to markdown
  terminat scan deep --region us-east-1 --export markdown

  # Export to custom file
  terminat scan deep --region us-east-1 --export json --output report.json

  # Fully automated scan with export
  terminat scan deep --region us-east-1 --auto-approve --auto-cleanup --export markdown`,
	RunE: runDeepScan,
}

func init() {
	scanCmd.AddCommand(quickCmd)
	scanCmd.AddCommand(deepCmd)
	scanCmd.AddCommand(demoCmd)

	// Common flags
	scanCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region (uses AWS_REGION env var if not specified)")
	scanCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile (uses AWS_PROFILE env var if not specified)")

	// Deep scan specific flags
	deepCmd.Flags().IntVarP(&duration, "duration", "d", 15, "Flow Log collection duration in minutes (max 60)")
	deepCmd.Flags().StringSliceVar(&natIDs, "nat-gateway-ids", []string{}, "Specific NAT Gateway IDs to analyze (optional)")
	deepCmd.Flags().StringVar(&vpcID, "vpc-id", "", "Filter NAT Gateways by VPC ID (optional)")
	deepCmd.Flags().BoolVar(&deepDoctor, "doctor", true, "Run doctor preflight checks before scan")
	quickCmd.Flags().BoolVar(&quickDoctor, "doctor", true, "Run doctor preflight checks before scan")
	deepCmd.Flags().StringVar(&deepUIMode, "ui", "stream", "UI mode [stream|tui]")
	quickCmd.Flags().StringVar(&quickUIMode, "ui", "stream", "UI mode [stream|tui]")
	demoCmd.Flags().StringVar(&demoUIMode, "ui", "stream", "UI mode [stream|tui]")
	deepCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip approval prompts (for automation)")
	deepCmd.Flags().BoolVar(&autoCleanup, "auto-cleanup", false, "Automatically delete log groups after scan")
	deepCmd.Flags().StringVarP(&exportFormat, "export", "e", "", "Export report format [json|markdown]")
	deepCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path for export (requires --export)")
	deepCmd.Flags().StringVar(&datahubAPIKey, "doit-datahub-api-key", "", "DoiT DataHub API key (or set DOIT_DATAHUB_API_KEY)")
	deepCmd.Flags().StringVar(&datahubCustomerContext, "doit-customer-context", "", "DoiT customer context (optional, for multi-tenant API keys)")
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
	if !isValidUIMode(quickUIMode) {
		return fmt.Errorf("invalid --ui value %q (valid: stream, tui)", quickUIMode)
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

	if quickDoctor {
		if err := runDoctorPreflight(ctx, scanner, selectedRegion, selectedProfile, false); err != nil {
			return err
		}
	}

	// Run quick scan with UI
	return ui.RunQuickScan(ctx, scanner, quickUIMode)
}

func runDeepScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if !isValidUIMode(deepUIMode) {
		return fmt.Errorf("invalid --ui value %q (valid: stream, tui)", deepUIMode)
	}

	// Validate duration
	if duration < 5 || duration > 60 {
		return fmt.Errorf("duration must be between 5 and 60 minutes")
	}

	// Validate --output requires --export
	if outputFile != "" && exportFormat == "" {
		return fmt.Errorf("--output requires --export flag (e.g., --export markdown --output report.md)")
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

	if deepDoctor {
		if err := runDoctorPreflight(ctx, scanner, selectedRegion, selectedProfile, true); err != nil {
			return err
		}
	}

	// Run deep scan with UI
	return ui.RunDeepScan(ctx, scanner, selectedRegion, duration, natIDs, vpcID, deepUIMode, autoApprove, autoCleanup, exportFormat, outputFile, datahubAPIKey, datahubCustomerContext)
}

func runDemoScan(cmd *cobra.Command, args []string) error {
	if !isValidUIMode(demoUIMode) {
		return fmt.Errorf("invalid --ui value %q (valid: stream, tui)", demoUIMode)
	}
	return ui.RunDemoScan(demoUIMode)
}

func isValidUIMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "stream", "tui":
		return true
	default:
		return false
	}
}

func runDoctorPreflight(ctx context.Context, scanner *core.Scanner, selectedRegion, selectedProfile string, requiresFlowLogsRole bool) error {
	fmt.Fprintln(os.Stderr, "🩺 Running doctor preflight checks...")
	fmt.Fprintf(os.Stderr, "✓ Region: %s\n", selectedRegion)
	if selectedProfile != "" {
		fmt.Fprintf(os.Stderr, "✓ AWS profile: %s\n", selectedProfile)
	} else {
		fmt.Fprintln(os.Stderr, "✓ AWS profile: default credential chain")
	}
	fmt.Fprintf(os.Stderr, "✓ AWS authentication: account %s\n", scanner.GetAccountID())

	if requiresFlowLogsRole {
		roleARN := fmt.Sprintf("arn:aws:iam::%s:role/termiNATor-FlowLogsRole", scanner.GetAccountID())
		if err := scanner.ValidateFlowLogsRole(ctx, roleARN); err != nil {
			return fmt.Errorf("doctor failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "✓ Flow Logs role: %s\n", roleARN)
	}

	fmt.Fprintln(os.Stderr, "✓ Doctor preflight passed")
	fmt.Fprintln(os.Stderr, "")
	return nil
}
