package analysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/doitintl/terminator/pkg/types"
)

// EndpointAnalysis contains VPC endpoint configuration analysis
type EndpointAnalysis struct {
	VPCID              string
	Region             string
	S3Endpoint         *types.VPCEndpoint
	DynamoEndpoint     *types.VPCEndpoint
	InterfaceEndpoints []types.VPCEndpoint
	RouteTables        []types.RouteTable
	MissingEndpoints   []string
	MissingRoutes      []MissingRoute
}

// InterfaceEndpointCost represents the cost of an interface endpoint
type InterfaceEndpointCost struct {
	Endpoint       types.VPCEndpoint
	HourlyCost     float64 // $0.01/hour per AZ
	MonthlyCost    float64
	AZCount        int
	ServiceName    string
	IsLikelyUnused bool // Based on heuristics
}

// MissingRoute represents a route table missing VPC endpoint route
type MissingRoute struct {
	RouteTableID   string
	RouteTableName string
	Service        string
	SubnetIDs      []string
}

// AnalyzeEndpoints checks VPC endpoint configuration
func AnalyzeEndpoints(region string, vpcID string, endpoints []types.VPCEndpoint, routeTables []types.RouteTable) *EndpointAnalysis {
	analysis := &EndpointAnalysis{
		VPCID:       vpcID,
		Region:      region,
		RouteTables: routeTables,
	}

	s3ServiceName := fmt.Sprintf("com.amazonaws.%s.s3", region)
	dynamoServiceName := fmt.Sprintf("com.amazonaws.%s.dynamodb", region)

	// Find existing endpoints
	for i := range endpoints {
		ep := &endpoints[i]
		if strings.Contains(ep.ServiceName, ".s3") && ep.Type == "Gateway" {
			analysis.S3Endpoint = ep
		}
		if strings.Contains(ep.ServiceName, ".dynamodb") && ep.Type == "Gateway" {
			analysis.DynamoEndpoint = ep
		}
		// Collect Interface endpoints
		if ep.Type == "Interface" {
			analysis.InterfaceEndpoints = append(analysis.InterfaceEndpoints, *ep)
		}
	}

	// Check for missing endpoints
	if analysis.S3Endpoint == nil {
		analysis.MissingEndpoints = append(analysis.MissingEndpoints, s3ServiceName)
	}
	if analysis.DynamoEndpoint == nil {
		analysis.MissingEndpoints = append(analysis.MissingEndpoints, dynamoServiceName)
	}

	// Check route tables for missing routes
	for _, rt := range routeTables {
		rtName := rt.Tags["Name"]
		if rtName == "" {
			rtName = rt.ID
		}

		// Check if this route table routes to NAT Gateway
		routesToNAT := false
		for _, route := range rt.Routes {
			if route.TargetType == "nat-gateway" {
				routesToNAT = true
				break
			}
		}

		if !routesToNAT {
			continue // Skip route tables that don't use NAT
		}

		// Check S3 endpoint route
		if analysis.S3Endpoint != nil {
			hasS3Route := false
			for _, rtID := range analysis.S3Endpoint.RouteTables {
				if rtID == rt.ID {
					hasS3Route = true
					break
				}
			}
			if !hasS3Route {
				analysis.MissingRoutes = append(analysis.MissingRoutes, MissingRoute{
					RouteTableID:   rt.ID,
					RouteTableName: rtName,
					Service:        "S3",
					SubnetIDs:      rt.Subnets,
				})
			}
		}

		// Check DynamoDB endpoint route
		if analysis.DynamoEndpoint != nil {
			hasDynamoRoute := false
			for _, rtID := range analysis.DynamoEndpoint.RouteTables {
				if rtID == rt.ID {
					hasDynamoRoute = true
					break
				}
			}
			if !hasDynamoRoute {
				analysis.MissingRoutes = append(analysis.MissingRoutes, MissingRoute{
					RouteTableID:   rt.ID,
					RouteTableName: rtName,
					Service:        "DynamoDB",
					SubnetIDs:      rt.Subnets,
				})
			}
		}
	}

	return analysis
}

// HasIssues returns true if there are missing endpoints or routes
func (a *EndpointAnalysis) HasIssues() bool {
	return len(a.MissingEndpoints) > 0 || len(a.MissingRoutes) > 0
}

// String returns a formatted summary
func (a *EndpointAnalysis) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("VPC: %s\n\n", a.VPCID))

	// Endpoint status
	b.WriteString("VPC Gateway Endpoints:\n")
	if a.S3Endpoint != nil {
		b.WriteString(fmt.Sprintf("  ✓ S3: %s (associated with %d route tables)\n",
			a.S3Endpoint.ID, len(a.S3Endpoint.RouteTables)))
	} else {
		b.WriteString("  ✗ S3: NOT CONFIGURED\n")
	}
	if a.DynamoEndpoint != nil {
		b.WriteString(fmt.Sprintf("  ✓ DynamoDB: %s (associated with %d route tables)\n",
			a.DynamoEndpoint.ID, len(a.DynamoEndpoint.RouteTables)))
	} else {
		b.WriteString("  ✗ DynamoDB: NOT CONFIGURED\n")
	}

	// Missing routes
	if len(a.MissingRoutes) > 0 {
		b.WriteString("\nRoute Tables Missing VPC Endpoint Routes:\n")
		for _, mr := range a.MissingRoutes {
			b.WriteString(fmt.Sprintf("  • %s (%s): missing %s endpoint route\n",
				mr.RouteTableName, mr.RouteTableID, mr.Service))
		}
	}

	return b.String()
}

// GetCreateEndpointCommands returns AWS CLI commands to create missing endpoints
func (a *EndpointAnalysis) GetCreateEndpointCommands() []string {
	var commands []string

	// Get all route table IDs that route to NAT
	var rtIDs []string
	for _, rt := range a.RouteTables {
		for _, route := range rt.Routes {
			if route.TargetType == "nat-gateway" {
				rtIDs = append(rtIDs, rt.ID)
				break
			}
		}
	}
	rtIDsStr := strings.Join(rtIDs, " ")

	for _, svc := range a.MissingEndpoints {
		cmd := fmt.Sprintf("aws ec2 create-vpc-endpoint \\\n  --vpc-id %s \\\n  --service-name %s \\\n  --route-table-ids %s",
			a.VPCID, svc, rtIDsStr)
		commands = append(commands, cmd)
	}

	return commands
}

// GetAddRouteCommands returns AWS CLI commands to add missing routes
func (a *EndpointAnalysis) GetAddRouteCommands() []string {
	var commands []string

	for _, mr := range a.MissingRoutes {
		var endpointID string
		if mr.Service == "S3" && a.S3Endpoint != nil {
			endpointID = a.S3Endpoint.ID
		} else if mr.Service == "DynamoDB" && a.DynamoEndpoint != nil {
			endpointID = a.DynamoEndpoint.ID
		} else {
			continue
		}

		cmd := fmt.Sprintf("aws ec2 modify-vpc-endpoint \\\n  --vpc-endpoint-id %s \\\n  --add-route-table-ids %s",
			endpointID, mr.RouteTableID)
		commands = append(commands, cmd)
	}

	return commands
}

// GetInterfaceEndpointCosts calculates costs for all Interface endpoints
// Interface endpoints cost $0.01/hour per AZ + $0.01/GB data processed
func (a *EndpointAnalysis) GetInterfaceEndpointCosts() []InterfaceEndpointCost {
	var costs []InterfaceEndpointCost

	for _, ep := range a.InterfaceEndpoints {
		// Extract service name from full service name
		// e.g., "com.amazonaws.us-east-1.ec2" -> "ec2"
		parts := strings.Split(ep.ServiceName, ".")
		serviceName := parts[len(parts)-1]

		// Assume 1 AZ per endpoint (conservative estimate)
		// In reality, we'd need to check subnet associations
		azCount := 1

		hourlyCost := 0.01 * float64(azCount)
		monthlyCost := hourlyCost * 24 * 30

		costs = append(costs, InterfaceEndpointCost{
			Endpoint:    ep,
			HourlyCost:  hourlyCost,
			MonthlyCost: monthlyCost,
			AZCount:     azCount,
			ServiceName: serviceName,
		})
	}

	return costs
}

// GetTotalInterfaceEndpointMonthlyCost returns total monthly cost of all Interface endpoints
func (a *EndpointAnalysis) GetTotalInterfaceEndpointMonthlyCost() float64 {
	costs := a.GetInterfaceEndpointCosts()
	total := 0.0
	for _, c := range costs {
		total += c.MonthlyCost
	}
	return total
}

// HasInterfaceEndpoints returns true if there are Interface endpoints
func (a *EndpointAnalysis) HasInterfaceEndpoints() bool {
	return len(a.InterfaceEndpoints) > 0
}

// AnalyzeAllVPCEndpoints runs quick scan analysis on all VPCs with NAT Gateways
// Returns findings for all VPCs
func AnalyzeAllVPCEndpoints(ctx context.Context, scanner interface {
	DiscoverVPCEndpoints(ctx context.Context, vpcID string) ([]types.VPCEndpoint, error)
	DiscoverRouteTables(ctx context.Context, vpcID string) ([]types.RouteTable, error)
}, nats []types.NATGateway) []types.Finding {
	var findings []types.Finding

	// Group NATs by VPC
	vpcNATs := make(map[string][]types.NATGateway)
	for _, nat := range nats {
		vpcNATs[nat.VPCID] = append(vpcNATs[nat.VPCID], nat)
	}

	// Check each VPC for missing endpoints
	for vpcID := range vpcNATs {
		endpoints, err := scanner.DiscoverVPCEndpoints(ctx, vpcID)
		if err != nil {
			continue
		}

		routeTables, err := scanner.DiscoverRouteTables(ctx, vpcID)
		if err != nil {
			continue
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
			// Check route table associations
			natRouteTables := getRouteTablesWithNAT(routeTables)
			missingAssociations := findMissingAssociations(natRouteTables, s3EndpointRTs)
			if len(missingAssociations) > 0 {
				findings = append(findings, types.Finding{
					Type:        "misconfigured-endpoint",
					Severity:    "high",
					Title:       "S3 Gateway Endpoint Missing Route Table Associations",
					Description: fmt.Sprintf("VPC %s: S3 endpoint not associated with %d route table(s)", vpcID, len(missingAssociations)),
					VPCID:       vpcID,
					Service:     "S3",
					Action:      fmt.Sprintf("Associate S3 endpoint with: %s", strings.Join(missingAssociations, ", ")),
					Impact:      "S3 traffic from some subnets still goes through NAT Gateway",
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
			natRouteTables := getRouteTablesWithNAT(routeTables)
			missingAssociations := findMissingAssociations(natRouteTables, ddbEndpointRTs)
			if len(missingAssociations) > 0 {
				findings = append(findings, types.Finding{
					Type:        "misconfigured-endpoint",
					Severity:    "high",
					Title:       "DynamoDB Gateway Endpoint Missing Route Table Associations",
					Description: fmt.Sprintf("VPC %s: DynamoDB endpoint not associated with %d route table(s)", vpcID, len(missingAssociations)),
					VPCID:       vpcID,
					Service:     "DynamoDB",
					Action:      fmt.Sprintf("Associate DynamoDB endpoint with: %s", strings.Join(missingAssociations, ", ")),
					Impact:      "DynamoDB traffic from some subnets still goes through NAT Gateway",
				})
			}
		}
	}

	return findings
}

func getRouteTablesWithNAT(routeTables []types.RouteTable) []string {
	var result []string
	for _, rt := range routeTables {
		for _, route := range rt.Routes {
			if route.TargetType == "nat-gateway" && route.DestinationCIDR == "0.0.0.0/0" {
				result = append(result, rt.ID)
				break
			}
		}
	}
	return result
}

func findMissingAssociations(natRouteTables, endpointRTs []string) []string {
	var missing []string
	for _, rtID := range natRouteTables {
		found := false
		for _, epRT := range endpointRTs {
			if epRT == rtID {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, rtID)
		}
	}
	return missing
}
