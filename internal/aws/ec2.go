package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	pkgtypes "github.com/doitintl/terminator/pkg/types"
)

// EC2Client wraps AWS EC2 API calls
type EC2Client struct {
	client *ec2.Client
}

// NewEC2Client creates a new EC2 client wrapper
func NewEC2Client(client *ec2.Client) *EC2Client {
	return &EC2Client{client: client}
}

// DiscoverNATGateways finds all NAT Gateways in the region
func (c *EC2Client) DiscoverNATGateways(ctx context.Context) ([]pkgtypes.NATGateway, error) {
	input := &ec2.DescribeNatGatewaysInput{}
	result, err := c.client.DescribeNatGateways(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe NAT gateways: %w", err)
	}

	var nats []pkgtypes.NATGateway
	for _, nat := range result.NatGateways {
		// Skip deleted/failed NAT gateways
		if nat.State == types.NatGatewayStateDeleted || nat.State == types.NatGatewayStateFailed {
			continue
		}

		tags := make(map[string]string)
		for _, tag := range nat.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}

		natGW := pkgtypes.NATGateway{
			ID:               *nat.NatGatewayId,
			VPCID:            *nat.VpcId,
			SubnetID:         *nat.SubnetId,
			State:            string(nat.State),
			ConnectivityType: string(nat.ConnectivityType),
			Tags:             tags,
		}

		// Determine availability mode and network interface
		if len(nat.NatGatewayAddresses) > 0 {
			natGW.NetworkInterfaceID = *nat.NatGatewayAddresses[0].NetworkInterfaceId
		}

		// Check if regional NAT (has multiple addresses across AZs)
		if len(nat.NatGatewayAddresses) > 1 {
			natGW.AvailabilityMode = "regional"
		} else {
			natGW.AvailabilityMode = "zonal"
		}

		nats = append(nats, natGW)
	}

	return nats, nil
}

// DiscoverVPCEndpoints finds all VPC endpoints for a given VPC
func (c *EC2Client) DiscoverVPCEndpoints(ctx context.Context, vpcID string) ([]pkgtypes.VPCEndpoint, error) {
	input := &ec2.DescribeVpcEndpointsInput{
		Filters: []types.Filter{
			{
				Name:   stringPtr("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.client.DescribeVpcEndpoints(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPC endpoints: %w", err)
	}

	var endpoints []pkgtypes.VPCEndpoint
	for _, ep := range result.VpcEndpoints {
		tags := make(map[string]string)
		for _, tag := range ep.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}

		endpoint := pkgtypes.VPCEndpoint{
			ID:          *ep.VpcEndpointId,
			VPCID:       *ep.VpcId,
			ServiceName: *ep.ServiceName,
			Type:        string(ep.VpcEndpointType),
			State:       string(ep.State),
			RouteTables: ep.RouteTableIds,
			PrivateDNS:  ep.PrivateDnsEnabled != nil && *ep.PrivateDnsEnabled,
			Tags:        tags,
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

// DiscoverRouteTables finds all route tables for a VPC
func (c *EC2Client) DiscoverRouteTables(ctx context.Context, vpcID string) ([]pkgtypes.RouteTable, error) {
	input := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   stringPtr("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	result, err := c.client.DescribeRouteTables(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe route tables: %w", err)
	}

	var routeTables []pkgtypes.RouteTable
	for _, rt := range result.RouteTables {
		tags := make(map[string]string)
		for _, tag := range rt.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}

		var routes []pkgtypes.Route
		for _, route := range rt.Routes {
			r := pkgtypes.Route{}
			if route.DestinationCidrBlock != nil {
				r.DestinationCIDR = *route.DestinationCidrBlock
			}

			// Determine target type
			if route.NatGatewayId != nil {
				r.Target = *route.NatGatewayId
				r.TargetType = "nat-gateway"
			} else if route.GatewayId != nil {
				r.Target = *route.GatewayId
				if *route.GatewayId == "local" {
					r.TargetType = "local"
				} else {
					r.TargetType = "igw"
				}
			} else if route.VpcPeeringConnectionId != nil {
				r.Target = *route.VpcPeeringConnectionId
				r.TargetType = "vpc-peering"
			}

			routes = append(routes, r)
		}

		var subnets []string
		for _, assoc := range rt.Associations {
			if assoc.SubnetId != nil {
				subnets = append(subnets, *assoc.SubnetId)
			}
		}

		isMain := false
		for _, assoc := range rt.Associations {
			if assoc.Main != nil && *assoc.Main {
				isMain = true
				break
			}
		}

		routeTable := pkgtypes.RouteTable{
			ID:      *rt.RouteTableId,
			VPCID:   *rt.VpcId,
			Routes:  routes,
			Subnets: subnets,
			Main:    isMain,
			Tags:    tags,
		}

		routeTables = append(routeTables, routeTable)
	}

	return routeTables, nil
}

func stringPtr(s string) *string {
	return &s
}


// CreateFlowLogs creates VPC Flow Logs for NAT Gateway analysis
func (c *EC2Client) CreateFlowLogs(ctx context.Context, nat pkgtypes.NATGateway, logGroupName string, deliveryRoleArn string, runID string) (string, error) {
	// Determine resource type and ID based on NAT mode
	var resourceType types.FlowLogsResourceType
	var resourceID string

	if nat.AvailabilityMode == "regional" {
		// Regional NAT: target the NAT Gateway itself
		resourceType = "RegionalNatGateway"
		resourceID = nat.ID
	} else {
		// Zonal NAT: target the ENI
		resourceType = types.FlowLogsResourceTypeNetworkInterface
		resourceID = nat.NetworkInterfaceID
	}

	// Custom log format with pkt-dstaddr for accurate destination tracking
	logFormat := "${interface-id} ${srcaddr} ${dstaddr} ${pkt-srcaddr} ${pkt-dstaddr} ${srcport} ${dstport} ${protocol} ${packets} ${bytes} ${start} ${end} ${action} ${log-status}"

	input := &ec2.CreateFlowLogsInput{
		ResourceType:         resourceType,
		ResourceIds:          []string{resourceID},
		TrafficType:          types.TrafficTypeAll,
		LogDestinationType:   types.LogDestinationTypeCloudWatchLogs,
		LogGroupName:         &logGroupName,
		DeliverLogsPermissionArn: &deliveryRoleArn,
		LogFormat:            &logFormat,
		MaxAggregationInterval: intPtr(60), // 60 seconds for faster data
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpcFlowLog,
				Tags: []types.Tag{
					{Key: stringPtr("CreatedBy"), Value: stringPtr("termiNATor")},
					{Key: stringPtr("RunId"), Value: stringPtr(runID)},
					{Key: stringPtr("Timestamp"), Value: stringPtr(time.Now().Format(time.RFC3339))},
				},
			},
		},
	}

	result, err := c.client.CreateFlowLogs(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create flow logs: %w", err)
	}

	if len(result.FlowLogIds) == 0 {
		return "", fmt.Errorf("no flow log ID returned")
	}

	if len(result.Unsuccessful) > 0 {
		return "", fmt.Errorf("flow log creation failed: %s", *result.Unsuccessful[0].Error.Message)
	}

	return result.FlowLogIds[0], nil
}

// DeleteFlowLogs deletes VPC Flow Logs
func (c *EC2Client) DeleteFlowLogs(ctx context.Context, flowLogIDs []string) error {
	if len(flowLogIDs) == 0 {
		return nil
	}

	input := &ec2.DeleteFlowLogsInput{
		FlowLogIds: flowLogIDs,
	}

	result, err := c.client.DeleteFlowLogs(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete flow logs: %w", err)
	}

	if len(result.Unsuccessful) > 0 {
		return fmt.Errorf("flow log deletion failed: %s", *result.Unsuccessful[0].Error.Message)
	}

	return nil
}

// CheckActiveFlowLogs checks if any Flow Logs are actively using a log group
func (c *EC2Client) CheckActiveFlowLogs(ctx context.Context, logGroupName string) ([]string, error) {
	resp, err := c.client.DescribeFlowLogs(ctx, &ec2.DescribeFlowLogsInput{
		Filter: []types.Filter{
			{
				Name:   stringPtr("log-group-name"),
				Values: []string{logGroupName},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	var activeIDs []string
	for _, fl := range resp.FlowLogs {
		if fl.FlowLogStatus != nil && *fl.FlowLogStatus == "ACTIVE" {
			activeIDs = append(activeIDs, *fl.FlowLogId)
		}
	}

	return activeIDs, nil
}

// DescribeFlowLogs describes VPC Flow Logs
func (c *EC2Client) DescribeFlowLogs(ctx context.Context, flowLogIDs []string) ([]pkgtypes.FlowLog, error) {
	input := &ec2.DescribeFlowLogsInput{}
	if len(flowLogIDs) > 0 {
		input.FlowLogIds = flowLogIDs
	}

	result, err := c.client.DescribeFlowLogs(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe flow logs: %w", err)
	}

	var flowLogs []pkgtypes.FlowLog
	for _, fl := range result.FlowLogs {
		flowLog := pkgtypes.FlowLog{
			ID:             *fl.FlowLogId,
			ResourceID:     *fl.ResourceId,
			Status:         *fl.FlowLogStatus,
			LogGroupName:   stringValue(fl.LogGroupName),
			CreationTime:   *fl.CreationTime,
		}
		flowLogs = append(flowLogs, flowLog)
	}

	return flowLogs, nil
}

func intPtr(i int32) *int32 {
	return &i
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
