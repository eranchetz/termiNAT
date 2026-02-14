package analysis

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func TestAnalyzeAggregatedResultsUsesResolvedDestination(t *testing.T) {
	ta := &TrafficAnalyzer{classifier: &TrafficClassifier{}}

	results := [][]types.ResultField{
		{
			{Field: strPtr("resolved_dst"), Value: strPtr("52.216.0.1")},
			{Field: strPtr("total_bytes"), Value: strPtr("1024")},
		},
	}

	stats, err := ta.AnalyzeAggregatedResults(results)
	if err != nil {
		t.Fatalf("AnalyzeAggregatedResults returned error: %v", err)
	}

	if stats.TotalBytes != 1024 {
		t.Fatalf("expected TotalBytes 1024, got %d", stats.TotalBytes)
	}
	if stats.TotalRecords != 1 {
		t.Fatalf("expected TotalRecords 1, got %d", stats.TotalRecords)
	}
	if stats.OtherBytes != 1024 || stats.OtherRecords != 1 {
		t.Fatalf("expected row to be counted as other traffic, got bytes=%d records=%d", stats.OtherBytes, stats.OtherRecords)
	}
}

func TestAnalyzeAggregatedResultsFallsBackToPktDstAddr(t *testing.T) {
	ta := &TrafficAnalyzer{classifier: &TrafficClassifier{}}

	results := [][]types.ResultField{
		{
			{Field: strPtr("pkt_dstaddr"), Value: strPtr("54.239.1.1")},
			{Field: strPtr("total_bytes"), Value: strPtr("2048")},
		},
	}

	stats, err := ta.AnalyzeAggregatedResults(results)
	if err != nil {
		t.Fatalf("AnalyzeAggregatedResults returned error: %v", err)
	}

	if stats.TotalBytes != 2048 || stats.TotalRecords != 1 {
		t.Fatalf("unexpected totals: bytes=%d records=%d", stats.TotalBytes, stats.TotalRecords)
	}
}

func TestAnalyzeAggregatedResultsDoesNotDropBytesWhenDestinationMissing(t *testing.T) {
	ta := &TrafficAnalyzer{classifier: &TrafficClassifier{}}

	results := [][]types.ResultField{
		{
			{Field: strPtr("total_bytes"), Value: strPtr("512")},
		},
	}

	stats, err := ta.AnalyzeAggregatedResults(results)
	if err != nil {
		t.Fatalf("AnalyzeAggregatedResults returned error: %v", err)
	}

	if stats.TotalBytes != 512 || stats.TotalRecords != 1 {
		t.Fatalf("expected bytes row to be preserved, got bytes=%d records=%d", stats.TotalBytes, stats.TotalRecords)
	}
	if stats.OtherBytes != 512 || stats.OtherRecords != 1 {
		t.Fatalf("expected unknown destination to count as other, got bytes=%d records=%d", stats.OtherBytes, stats.OtherRecords)
	}
}

func TestParseAggregatedBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "integer", input: "123", want: 123},
		{name: "float", input: "123.9", want: 123},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAggregatedBytes(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
