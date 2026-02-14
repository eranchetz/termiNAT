package analysis

import (
	"math"
	"strings"
	"testing"

	"github.com/doitintl/terminator/pkg/types"
)

func TestGetCreateEndpointCommandsSkipsECRWhenConfigured(t *testing.T) {
	endpoints := []types.VPCEndpoint{
		{
			ID:          "vpce-s3",
			VPCID:       "vpc-1",
			ServiceName: "com.amazonaws.us-east-1.s3",
			Type:        "Gateway",
			RouteTables: []string{"rtb-1"},
		},
		{
			ID:          "vpce-ddb",
			VPCID:       "vpc-1",
			ServiceName: "com.amazonaws.us-east-1.dynamodb",
			Type:        "Gateway",
			RouteTables: []string{"rtb-1"},
		},
		{
			ID:          "vpce-ecr-api",
			VPCID:       "vpc-1",
			ServiceName: "com.amazonaws.us-east-1.ecr.api",
			Type:        "Interface",
			SubnetIDs:   []string{"subnet-a"},
		},
		{
			ID:          "vpce-ecr-dkr",
			VPCID:       "vpc-1",
			ServiceName: "com.amazonaws.us-east-1.ecr.dkr",
			Type:        "Interface",
			SubnetIDs:   []string{"subnet-a"},
		},
	}
	routeTables := []types.RouteTable{
		{
			ID:      "rtb-1",
			VPCID:   "vpc-1",
			Subnets: []string{"subnet-a"},
			Routes: []types.Route{
				{DestinationCIDR: "0.0.0.0/0", TargetType: "nat-gateway"},
			},
		},
	}

	a := AnalyzeEndpoints("us-east-1", "vpc-1", endpoints, routeTables)
	if a.HasMissingECRInterfaceEndpoints() {
		t.Fatalf("expected ECR interface endpoints to be fully configured")
	}

	cmds := a.GetCreateEndpointCommands()
	if len(cmds) != 0 {
		t.Fatalf("expected no create commands when all endpoints exist, got %d", len(cmds))
	}
}

func TestGetCreateEndpointCommandsIncludesMissingECREndpoints(t *testing.T) {
	endpoints := []types.VPCEndpoint{
		{
			ID:          "vpce-s3",
			VPCID:       "vpc-1",
			ServiceName: "com.amazonaws.us-east-1.s3",
			Type:        "Gateway",
			RouteTables: []string{"rtb-1"},
		},
		{
			ID:          "vpce-ddb",
			VPCID:       "vpc-1",
			ServiceName: "com.amazonaws.us-east-1.dynamodb",
			Type:        "Gateway",
			RouteTables: []string{"rtb-1"},
		},
	}
	routeTables := []types.RouteTable{
		{
			ID:      "rtb-1",
			VPCID:   "vpc-1",
			Subnets: []string{"subnet-a", "subnet-b"},
			Routes: []types.Route{
				{DestinationCIDR: "0.0.0.0/0", TargetType: "nat-gateway"},
			},
		},
	}

	a := AnalyzeEndpoints("us-east-1", "vpc-1", endpoints, routeTables)
	cmds := a.GetCreateEndpointCommands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 ECR interface endpoint create commands, got %d", len(cmds))
	}

	joined := strings.Join(cmds, "\n")
	if !strings.Contains(joined, "com.amazonaws.us-east-1.ecr.api") {
		t.Fatalf("missing ecr.api create command")
	}
	if !strings.Contains(joined, "com.amazonaws.us-east-1.ecr.dkr") {
		t.Fatalf("missing ecr.dkr create command")
	}
	if !strings.Contains(joined, "--vpc-endpoint-type Interface") {
		t.Fatalf("missing interface endpoint flag in remediation commands")
	}
	if !strings.Contains(joined, "--security-group-ids '<security-group-id>'") {
		t.Fatalf("missing security group placeholder in remediation commands")
	}
	if !strings.Contains(joined, "--subnet-ids 'subnet-a' 'subnet-b'") {
		t.Fatalf("missing NAT subnet IDs in remediation commands")
	}
}

func TestEstimateECRInterfaceEndpointMonthlyCost(t *testing.T) {
	a := &EndpointAnalysis{
		Region: "us-east-1",
		RouteTables: []types.RouteTable{
			{
				ID:      "rtb-1",
				Subnets: []string{"subnet-a", "subnet-b"},
				Routes: []types.Route{
					{DestinationCIDR: "0.0.0.0/0", TargetType: "nat-gateway"},
				},
			},
		},
	}

	fixed, data, total, azCount, endpointCount := a.EstimateECRInterfaceEndpointMonthlyCost(100)
	if azCount != 2 {
		t.Fatalf("expected azCount=2, got %d", azCount)
	}
	if endpointCount != 2 {
		t.Fatalf("expected endpointCount=2 (api+dkr), got %d", endpointCount)
	}
	assertApprox(t, fixed, 28.8, 0.0001, "fixed monthly cost")
	assertApprox(t, data, 1.0, 0.0001, "data monthly cost")
	assertApprox(t, total, 29.8, 0.0001, "total monthly cost")
}

func TestGetECRInterfaceEndpointPricingFallback(t *testing.T) {
	a := &EndpointAnalysis{Region: "unknown-region-1"}
	hourly, data := a.GetECRInterfaceEndpointPricing()
	assertApprox(t, hourly, 0.01, 0.0001, "fallback hourly price")
	assertApprox(t, data, 0.01, 0.0001, "fallback data price")
}

func assertApprox(t *testing.T, got, want, tol float64, label string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s: expected %.4f, got %.4f", label, want, got)
	}
}
