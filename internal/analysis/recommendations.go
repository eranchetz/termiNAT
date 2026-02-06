package analysis

import (
	"fmt"
	"strings"

	pkgtypes "github.com/doitintl/terminator/pkg/types"
)

type Recommendation struct {
	Type        string // "regional-nat-gateway", "vpc-endpoint", etc.
	Priority    string // "high", "medium", "low"
	Title       string
	Description string
	Benefits    []string
	Commands    []string
	Savings     string
}

// AnalyzeNATGatewaySetup analyzes NAT Gateway configuration and provides recommendations
func AnalyzeNATGatewaySetup(nats []pkgtypes.NATGateway) []Recommendation {
	var recommendations []Recommendation

	// Group NAT Gateways by VPC
	vpcNATs := make(map[string][]pkgtypes.NATGateway)
	for _, nat := range nats {
		vpcNATs[nat.VPCID] = append(vpcNATs[nat.VPCID], nat)
	}

	// Check each VPC for multi-zonal NAT setup
	for vpcID, vpcNATList := range vpcNATs {
		zonalCount := 0
		regionalCount := 0

		for _, nat := range vpcNATList {
			if nat.AvailabilityMode == "regional" {
				regionalCount++
			} else {
				zonalCount++
			}
		}

		// Recommend Regional NAT if customer has multiple zonal NATs
		if zonalCount >= 2 && regionalCount == 0 {
			recommendations = append(recommendations, Recommendation{
				Type:     "regional-nat-gateway",
				Priority: "high",
				Title:    fmt.Sprintf("Consider Regional NAT Gateway for VPC %s", vpcID),
				Description: fmt.Sprintf(
					"You have %d zonal NAT Gateways in this VPC. AWS Regional NAT Gateway can simplify your architecture "+
						"by replacing multiple zonal NAT Gateways with a single regional resource that automatically spans all Availability Zones.",
					zonalCount,
				),
				Benefits: []string{
					"Simplified management - single NAT Gateway resource instead of multiple",
					"No public subnets required - improved security posture",
					"Automatic multi-AZ expansion - scales to new AZs automatically",
					"Eliminates cross-AZ data transfer costs ($0.01/GB) when properly configured",
					"Built-in high availability across all AZs",
					"Automatic IP scaling - up to 32 IPs per AZ for port exhaustion protection",
				},
				Commands: []string{
					fmt.Sprintf("# Create Regional NAT Gateway for VPC %s", vpcID),
					fmt.Sprintf("aws ec2 create-nat-gateway \\"),
					fmt.Sprintf("  --vpc-id %s \\", shellQuote(vpcID)),
					"  --availability-mode regional \\",
					"  --connectivity-type public",
					"",
					"# After creating Regional NAT Gateway:",
					"# 1. Update route tables to point to new Regional NAT Gateway",
					"# 2. Test connectivity from all AZs",
					"# 3. Delete old zonal NAT Gateways",
				},
				Savings: "Eliminates cross-AZ data transfer costs ($0.01/GB) and simplifies operations",
			})
		}
	}

	return recommendations
}

// FormatRecommendations formats recommendations for display
func FormatRecommendations(recommendations []Recommendation) string {
	if len(recommendations) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n🎯 RECOMMENDATIONS\n")
	b.WriteString(strings.Repeat("=", 80) + "\n\n")

	for i, rec := range recommendations {
		b.WriteString(fmt.Sprintf("%d. %s [%s priority]\n", i+1, rec.Title, strings.ToUpper(rec.Priority)))
		b.WriteString(fmt.Sprintf("\n%s\n\n", rec.Description))

		if len(rec.Benefits) > 0 {
			b.WriteString("Benefits:\n")
			for _, benefit := range rec.Benefits {
				b.WriteString(fmt.Sprintf("  ✓ %s\n", benefit))
			}
			b.WriteString("\n")
		}

		if rec.Savings != "" {
			b.WriteString(fmt.Sprintf("Potential Savings: %s\n\n", rec.Savings))
		}

		if len(rec.Commands) > 0 {
			b.WriteString("How to implement:\n")
			for _, cmd := range rec.Commands {
				b.WriteString(fmt.Sprintf("  %s\n", cmd))
			}
			b.WriteString("\n")
		}

		if i < len(recommendations)-1 {
			b.WriteString(strings.Repeat("-", 80) + "\n\n")
		}
	}

	return b.String()
}
