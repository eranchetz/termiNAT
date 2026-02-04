package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "terminat",
	Short: "termiNATor - Terminate unnecessary NAT Gateway costs",
	Long: `termiNATor helps AWS customers identify and quantify avoidable NAT Gateway 
spend caused by workloads using NAT to reach AWS services when VPC endpoints 
could be used instead.`,
}

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
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
