package analysis

import (
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
