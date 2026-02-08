package datahub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/pkg/types"
)

var apiURL = "https://api.doit.com/datahub/v1/events"

type Dimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type Metric struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

type Event struct {
	Provider   string      `json:"provider"`
	ID         string      `json:"id"`
	Time       string      `json:"time"`
	Dimensions []Dimension `json:"dimensions"`
	Metrics    []Metric    `json:"metrics"`
}

type eventBatch struct {
	Events []Event `json:"events"`
}

// BuildEvents creates DataHub events from scan results.
// Produces 5 events per NAT: 1 aggregated + 4 per-service (S3, DynamoDB, ECR, Other).
func BuildEvents(accountID, region string, nats []types.NATGateway, stats *analysis.TrafficStats, cost *analysis.CostEstimate, endpoints *analysis.EndpointAnalysis) []Event {
	if stats == nil || cost == nil {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	date := time.Now().UTC().Format("2006-01-02")

	// Determine endpoint status per service
	s3Status, dynamoStatus := "missing", "missing"
	if endpoints != nil {
		if endpoints.S3Endpoint != nil {
			s3Status = "configured"
		}
		if endpoints.DynamoEndpoint != nil {
			dynamoStatus = "configured"
		}
	}

	type svcData struct {
		name     string
		sku      string
		costVal  float64
		savings  float64
		usageGB  float64
		epStatus string
	}

	services := []svcData{
		{"S3", "S3 traffic via NAT", cost.S3SavingsMonthly, cost.S3SavingsMonthly, cost.S3DataGB, s3Status},
		{"DynamoDB", "DynamoDB traffic via NAT", cost.DynamoSavingsMonthly, cost.DynamoSavingsMonthly, cost.DynamoDataGB, dynamoStatus},
		{"ECR", "ECR traffic via NAT", cost.OtherDataGB * cost.NATGatewayPricePerGB * (stats.ECRPercentage() / cost.OtherPercentage()), 0, cost.OtherDataGB * (stats.ECRPercentage() / cost.OtherPercentage()), "n-a"},
		{"Other", "Other traffic via NAT", cost.OtherDataGB * cost.NATGatewayPricePerGB, 0, cost.OtherDataGB, "n-a"},
	}

	// Fix ECR NaN when OtherPercentage is 0
	if cost.OtherPercentage() == 0 {
		services[2].costVal = 0
		services[2].usageGB = 0
	}

	var events []Event
	for _, nat := range nats {
		baseDims := []Dimension{
			{Key: "project_id", Value: accountID, Type: "fixed"},
			{Key: "region", Value: region, Type: "fixed"},
			{Key: "resource_id", Value: nat.ID, Type: "fixed"},
			{Key: "service_description", Value: "NAT Gateway Data Processing", Type: "fixed"},
			{Key: "vpc_id", Value: nat.VPCID, Type: "label"},
			{Key: "scan_type", Value: "deep", Type: "label"},
		}

		// Aggregated event
		aggDims := append(append([]Dimension{}, baseDims...),
			Dimension{Key: "sku_description", Value: "NAT Gateway - Avoidable Cost", Type: "fixed"},
			Dimension{Key: "traffic_service", Value: "Total", Type: "label"},
			Dimension{Key: "endpoint_status", Value: "n-a", Type: "label"},
		)
		events = append(events, Event{
			Provider:   "termiNATor",
			ID:         fmt.Sprintf("%s_total_%s", nat.ID, date),
			Time:       now,
			Dimensions: aggDims,
			Metrics: []Metric{
				{Type: "cost", Value: cost.CurrentMonthlyCost},
				{Type: "savings", Value: cost.TotalSavingsMonthly},
				{Type: "usage", Value: cost.TotalDataGB},
			},
		})

		// Per-service events
		for _, svc := range services {
			dims := append(append([]Dimension{}, baseDims...),
				Dimension{Key: "sku_description", Value: svc.sku, Type: "fixed"},
				Dimension{Key: "traffic_service", Value: svc.name, Type: "label"},
				Dimension{Key: "endpoint_status", Value: svc.epStatus, Type: "label"},
			)
			events = append(events, Event{
				Provider:   "termiNATor",
				ID:         fmt.Sprintf("%s_%s_%s", nat.ID, svc.name, date),
				Time:       now,
				Dimensions: dims,
				Metrics: []Metric{
					{Type: "cost", Value: svc.costVal},
					{Type: "savings", Value: svc.savings},
					{Type: "usage", Value: svc.usageGB},
				},
			})
		}
	}
	return events
}

// Send posts events to the DoiT DataHub API with retry on 429.
func Send(apiKey, customerContext string, events []Event) error {
	// Batch in groups of 255 (API limit)
	for i := 0; i < len(events); i += 255 {
		end := i + 255
		if end > len(events) {
			end = len(events)
		}
		if err := sendBatch(apiKey, customerContext, events[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func sendBatch(apiKey, customerContext string, events []Event) error {
	body, err := json.Marshal(eventBatch{Events: events})
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		if customerContext != "" {
			req.Header.Set("x-customer-context", customerContext)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("DataHub API request failed: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			return nil
		}
		if resp.StatusCode == 429 && attempt < 3 {
			time.Sleep(time.Duration(10*(attempt+1)) * time.Second)
			continue
		}
		return fmt.Errorf("DataHub API returned %d", resp.StatusCode)
	}
	return nil
}
