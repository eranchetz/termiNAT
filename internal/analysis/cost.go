package analysis

import (
	"fmt"
)

// NAT Gateway data processing costs per GB by region (as of 2024)
// Source: https://aws.amazon.com/vpc/pricing/
var natGatewayPricing = map[string]float64{
	"us-east-1":      0.045, // US East (N. Virginia)
	"us-east-2":      0.045, // US East (Ohio)
	"us-west-1":      0.045, // US West (N. California)
	"us-west-2":      0.045, // US West (Oregon)
	"eu-west-1":      0.045, // Europe (Ireland)
	"eu-west-2":      0.045, // Europe (London)
	"eu-west-3":      0.045, // Europe (Paris)
	"eu-central-1":   0.045, // Europe (Frankfurt)
	"ap-southeast-1": 0.045, // Asia Pacific (Singapore)
	"ap-southeast-2": 0.045, // Asia Pacific (Sydney)
	"ap-northeast-1": 0.045, // Asia Pacific (Tokyo)
	"default":        0.045, // Default pricing
}

// VPC Endpoint pricing (Gateway endpoints for S3/DynamoDB are FREE)
// Interface endpoints have hourly charges but we focus on Gateway endpoints
const (
	s3EndpointCost      = 0.0 // Gateway endpoint - FREE
	dynamoEndpointCost  = 0.0 // Gateway endpoint - FREE
)

type CostEstimate struct {
	Region                string
	TotalDataGB           float64
	S3DataGB              float64
	DynamoDataGB          float64
	OtherDataGB           float64
	CurrentMonthlyCost    float64
	S3SavingsMonthly      float64
	DynamoSavingsMonthly  float64
	TotalSavingsMonthly   float64
	NATGatewayPricePerGB  float64
}

func CalculateCosts(region string, stats *TrafficStats, collectionMinutes int) *CostEstimate {
	// Get regional pricing
	pricePerGB, ok := natGatewayPricing[region]
	if !ok {
		pricePerGB = natGatewayPricing["default"]
	}

	// Convert bytes to GB
	totalGB := float64(stats.TotalBytes) / (1024 * 1024 * 1024)
	s3GB := float64(stats.S3Bytes) / (1024 * 1024 * 1024)
	dynamoGB := float64(stats.DynamoBytes) / (1024 * 1024 * 1024)

	// Extrapolate to monthly costs (assuming collection period is representative)
	// 1 month = ~43,200 minutes
	monthlyMultiplier := 43200.0 / float64(collectionMinutes)
	
	monthlyTotalGB := totalGB * monthlyMultiplier
	monthlyS3GB := s3GB * monthlyMultiplier
	monthlyDynamoGB := dynamoGB * monthlyMultiplier

	// Calculate costs
	currentMonthlyCost := monthlyTotalGB * pricePerGB
	s3Savings := monthlyS3GB * pricePerGB // S3 Gateway endpoint is free
	dynamoSavings := monthlyDynamoGB * pricePerGB // DynamoDB Gateway endpoint is free
	totalSavings := s3Savings + dynamoSavings

	return &CostEstimate{
		Region:                region,
		TotalDataGB:           monthlyTotalGB,
		S3DataGB:              monthlyS3GB,
		DynamoDataGB:          monthlyDynamoGB,
		OtherDataGB:           monthlyTotalGB - monthlyS3GB - monthlyDynamoGB,
		CurrentMonthlyCost:    currentMonthlyCost,
		S3SavingsMonthly:      s3Savings,
		DynamoSavingsMonthly:  dynamoSavings,
		TotalSavingsMonthly:   totalSavings,
		NATGatewayPricePerGB:  pricePerGB,
	}
}

func (c *CostEstimate) String() string {
	return fmt.Sprintf(
		"COST ESTIMATE (based on collected traffic sample)\n"+
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"+
		"Region: %s\n"+
		"NAT Gateway Data Processing: $%.4f per GB\n\n"+
		"Projected Monthly Traffic:\n"+
		"  Total:    %.2f GB\n"+
		"  S3:       %.2f GB (%.1f%%)\n"+
		"  DynamoDB: %.2f GB (%.1f%%)\n"+
		"  Other:    %.2f GB (%.1f%%)\n\n"+
		"Current Monthly NAT Gateway Cost: $%.2f\n\n"+
		"Potential Monthly Savings with VPC Endpoints:\n"+
		"  S3 Gateway Endpoint:       $%.2f\n"+
		"  DynamoDB Gateway Endpoint: $%.2f\n"+
		"  ─────────────────────────────────\n"+
		"  Total Potential Savings:   $%.2f/month ($%.2f/year)\n\n"+
		"⚠️  IMPORTANT: This is an ESTIMATE based on the traffic sample collected.\n"+
		"   Actual costs may vary based on traffic patterns, time of day, and workload changes.\n"+
		"   Gateway VPC Endpoints for S3 and DynamoDB are FREE (no hourly or data charges).",
		c.Region,
		c.NATGatewayPricePerGB,
		c.TotalDataGB,
		c.S3DataGB, c.S3Percentage(),
		c.DynamoDataGB, c.DynamoPercentage(),
		c.OtherDataGB, c.OtherPercentage(),
		c.CurrentMonthlyCost,
		c.S3SavingsMonthly,
		c.DynamoSavingsMonthly,
		c.TotalSavingsMonthly,
		c.TotalSavingsMonthly*12,
	)
}

func (c *CostEstimate) S3Percentage() float64 {
	if c.TotalDataGB == 0 {
		return 0
	}
	return (c.S3DataGB / c.TotalDataGB) * 100
}

func (c *CostEstimate) DynamoPercentage() float64 {
	if c.TotalDataGB == 0 {
		return 0
	}
	return (c.DynamoDataGB / c.TotalDataGB) * 100
}

func (c *CostEstimate) OtherPercentage() float64 {
	if c.TotalDataGB == 0 {
		return 0
	}
	return (c.OtherDataGB / c.TotalDataGB) * 100
}
