package types

import "time"

// NATGateway represents a NAT Gateway with its metadata
type NATGateway struct {
	ID                 string
	VPCID              string
	SubnetID           string
	State              string
	ConnectivityType   string
	AvailabilityMode   string // "zonal" or "regional"
	NetworkInterfaceID string // For zonal NAT
	Tags               map[string]string
}

// VPCEndpoint represents a VPC endpoint
type VPCEndpoint struct {
	ID          string
	VPCID       string
	ServiceName string
	Type        string // "Gateway" or "Interface"
	State       string
	RouteTables []string
	SubnetIDs   []string // Subnets = AZs for Interface endpoints
	PrivateDNS  bool
	Tags        map[string]string
}

// RouteTable represents a VPC route table
type RouteTable struct {
	ID      string
	VPCID   string
	Routes  []Route
	Subnets []string
	Main    bool
	Tags    map[string]string
}

// Route represents a single route in a route table
type Route struct {
	DestinationCIDR string
	Target          string
	TargetType      string // "nat-gateway", "igw", "vpc-endpoint", etc.
}

// Finding represents a configuration issue or recommendation
type Finding struct {
	Type        string // "missing-endpoint", "misconfigured-endpoint", etc.
	Severity    string // "high", "medium", "low"
	Title       string
	Description string
	VPCID       string
	Service     string // "S3", "DynamoDB", etc.
	Action      string
	Impact      string
}

// TrafficAnalysis represents analyzed traffic data
type TrafficAnalysis struct {
	NATGatewayID string
	Service      string
	BytesTotal   int64
	FlowCount    int64
	SampleWindow int // minutes
}

// CostEstimate represents cost calculations
type CostEstimate struct {
	Service               string
	CurrentNATCost        float64
	ProjectedEndpointCost float64
	MonthlySavings        float64
	ConfidenceLevel       string
}

// FlowLog represents a VPC Flow Log
type FlowLog struct {
	ID           string
	ResourceID   string
	Status       string
	LogGroupName string
	CreationTime time.Time
}
