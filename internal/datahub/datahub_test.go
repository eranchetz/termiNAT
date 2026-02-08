package datahub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terminator/internal/analysis"
	"github.com/doitintl/terminator/pkg/types"
)

func testData() ([]types.NATGateway, *analysis.TrafficStats, *analysis.CostEstimate, *analysis.EndpointAnalysis) {
	nats := []types.NATGateway{{ID: "nat-123", VPCID: "vpc-abc"}}
	stats := &analysis.TrafficStats{
		S3Bytes:     1073741824, // 1 GB
		DynamoBytes: 536870912,  // 0.5 GB
		ECRBytes:    107374182,  // ~0.1 GB
		OtherBytes:  214748365,  // ~0.2 GB
		TotalBytes:  1932734783,
	}
	cost := &analysis.CostEstimate{
		TotalDataGB:          1.8,
		S3DataGB:             1.0,
		DynamoDataGB:         0.5,
		OtherDataGB:          0.3,
		CurrentMonthlyCost:   0.081,
		S3SavingsMonthly:     0.045,
		DynamoSavingsMonthly: 0.0225,
		TotalSavingsMonthly:  0.0675,
		NATGatewayPricePerGB: 0.045,
	}
	endpoints := &analysis.EndpointAnalysis{
		S3Endpoint:     &types.VPCEndpoint{ID: "vpce-s3"},
		DynamoEndpoint: nil, // missing
	}
	return nats, stats, cost, endpoints
}

func TestBuildEventsNil(t *testing.T) {
	nats := []types.NATGateway{{ID: "nat-1"}}
	if events := BuildEvents("acct", "us-east-1", nats, nil, nil, nil); events != nil {
		t.Fatal("expected nil for nil stats/cost")
	}
}

func TestBuildEventsSingleNAT(t *testing.T) {
	nats, stats, cost, endpoints := testData()
	events := BuildEvents("123456789012", "us-east-1", nats, stats, cost, endpoints)

	if len(events) != 5 {
		t.Fatalf("got %d events, want 5", len(events))
	}

	// First event is aggregated
	agg := events[0]
	if agg.Provider != "termiNATor" {
		t.Fatalf("provider=%q", agg.Provider)
	}
	if agg.ID == "" {
		t.Fatal("empty event ID")
	}
	if len(agg.Metrics) != 3 {
		t.Fatalf("got %d metrics, want 3", len(agg.Metrics))
	}

	// Check aggregated metrics
	for _, m := range agg.Metrics {
		switch m.Type {
		case "cost":
			if m.Value != cost.CurrentMonthlyCost {
				t.Errorf("cost=%f, want %f", m.Value, cost.CurrentMonthlyCost)
			}
		case "savings":
			if m.Value != cost.TotalSavingsMonthly {
				t.Errorf("savings=%f, want %f", m.Value, cost.TotalSavingsMonthly)
			}
		}
	}

	// Check traffic_service labels across all events
	services := map[string]bool{}
	for _, e := range events {
		for _, d := range e.Dimensions {
			if d.Key == "traffic_service" {
				services[d.Value] = true
			}
		}
	}
	for _, want := range []string{"Total", "S3", "DynamoDB", "ECR", "Other"} {
		if !services[want] {
			t.Errorf("missing traffic_service=%q", want)
		}
	}
}

func TestBuildEventsEndpointStatus(t *testing.T) {
	nats, stats, cost, endpoints := testData()
	events := BuildEvents("acct", "us-east-1", nats, stats, cost, endpoints)

	statusFor := func(svc string) string {
		for _, e := range events {
			var trafSvc, epStatus string
			for _, d := range e.Dimensions {
				if d.Key == "traffic_service" {
					trafSvc = d.Value
				}
				if d.Key == "endpoint_status" {
					epStatus = d.Value
				}
			}
			if trafSvc == svc {
				return epStatus
			}
		}
		return ""
	}

	if s := statusFor("S3"); s != "configured" {
		t.Errorf("S3 endpoint_status=%q, want configured", s)
	}
	if s := statusFor("DynamoDB"); s != "missing" {
		t.Errorf("DynamoDB endpoint_status=%q, want missing", s)
	}
	if s := statusFor("ECR"); s != "n-a" {
		t.Errorf("ECR endpoint_status=%q, want n-a", s)
	}
}

func TestBuildEventsMultipleNATs(t *testing.T) {
	nats := []types.NATGateway{{ID: "nat-1", VPCID: "vpc-1"}, {ID: "nat-2", VPCID: "vpc-2"}}
	stats := &analysis.TrafficStats{TotalBytes: 1000, S3Bytes: 500, OtherBytes: 500}
	cost := &analysis.CostEstimate{TotalDataGB: 1, S3DataGB: 0.5, OtherDataGB: 0.5, NATGatewayPricePerGB: 0.045}
	events := BuildEvents("acct", "us-east-1", nats, stats, cost, nil)
	if len(events) != 10 {
		t.Fatalf("got %d events, want 10 (5 per NAT)", len(events))
	}
}

func TestBuildEventsECRNaNGuard(t *testing.T) {
	nats := []types.NATGateway{{ID: "nat-1"}}
	stats := &analysis.TrafficStats{TotalBytes: 1000, S3Bytes: 1000} // no Other/ECR bytes
	cost := &analysis.CostEstimate{TotalDataGB: 1, S3DataGB: 1, OtherDataGB: 0, NATGatewayPricePerGB: 0.045}
	events := BuildEvents("acct", "us-east-1", nats, stats, cost, nil)

	for _, e := range events {
		for _, m := range e.Metrics {
			if m.Value != m.Value { // NaN check
				t.Fatalf("NaN in metric %s for event %s", m.Type, e.ID)
			}
		}
	}
}

func TestSendSuccess(t *testing.T) {
	var received eventBatch
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("bad auth: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("x-customer-context") != "ctx" {
			t.Errorf("bad context: %q", r.Header.Get("x-customer-context"))
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	orig := apiURL
	apiURL = srv.URL
	defer func() { apiURL = orig }()

	events := []Event{{Provider: "test", ID: "e1"}}
	if err := Send("test-key", "ctx", events); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(received.Events) != 1 {
		t.Fatalf("server got %d events", len(received.Events))
	}
}

func TestSendNoCustomerContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-customer-context") != "" {
			t.Error("x-customer-context should be absent")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	orig := apiURL
	apiURL = srv.URL
	defer func() { apiURL = orig }()

	Send("key", "", []Event{{ID: "e1"}})
}

func TestSendErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	orig := apiURL
	apiURL = srv.URL
	defer func() { apiURL = orig }()

	err := Send("key", "", []Event{{ID: "e1"}})
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestSendRetry429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	orig := apiURL
	apiURL = srv.URL
	defer func() { apiURL = orig }()

	if err := Send("key", "", []Event{{ID: "e1"}}); err != nil {
		t.Fatalf("Send with retry: %v", err)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatal("expected at least 2 calls for 429 retry")
	}
}

func TestSendBatching(t *testing.T) {
	var batchCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch eventBatch
		json.NewDecoder(r.Body).Decode(&batch)
		if len(batch.Events) > 255 {
			t.Errorf("batch too large: %d", len(batch.Events))
		}
		atomic.AddInt32(&batchCount, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	orig := apiURL
	apiURL = srv.URL
	defer func() { apiURL = orig }()

	events := make([]Event, 300)
	for i := range events {
		events[i] = Event{ID: "e"}
	}
	if err := Send("key", "", events); err != nil {
		t.Fatalf("Send batching: %v", err)
	}
	if atomic.LoadInt32(&batchCount) != 2 {
		t.Fatalf("got %d batches, want 2", atomic.LoadInt32(&batchCount))
	}
}
