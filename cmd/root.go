package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const Version = "0.1.3"

var rootCmd = &cobra.Command{
	Use:     "terminator",
	Short:   "termiNATor - Terminate unnecessary NAT Gateway costs",
	Version: Version,
	Long: `termiNATor helps AWS customers identify and quantify avoidable NAT Gateway 
spend caused by workloads using NAT to reach AWS services when VPC endpoints 
could be used instead.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
