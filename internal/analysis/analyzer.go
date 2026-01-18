package analysis

import (
	"fmt"
	"sort"
	"strings"
)

type SourceIPStats struct {
	Bytes   int64
	Records int
	S3      int64
	Dynamo  int64
	ECR     int64
	Other   int64
}

type TrafficStats struct {
	S3Bytes       int64
	DynamoBytes   int64
	ECRBytes      int64
	OtherBytes    int64
	TotalBytes    int64
	S3Records     int
	DynamoRecords int
	ECRRecords    int
	OtherRecords  int
	TotalRecords  int
	SourceIPs     map[string]*SourceIPStats
}

type TrafficAnalyzer struct {
	classifier *TrafficClassifier
	stats      TrafficStats
}

func NewTrafficAnalyzer() (*TrafficAnalyzer, error) {
	classifier, err := NewTrafficClassifier()
	if err != nil {
		return nil, err
	}
	return &TrafficAnalyzer{classifier: classifier}, nil
}

func (ta *TrafficAnalyzer) AnalyzeFlowLogs(logLines []string) (*TrafficStats, error) {
	ta.stats = TrafficStats{SourceIPs: make(map[string]*SourceIPStats)}

	for _, line := range logLines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "ACCEPT") {
			continue
		}

		record, err := ParseFlowLogLine(line)
		if err != nil {
			continue
		}

		service := ta.classifier.ClassifyIP(record.DstAddr)

		ta.stats.TotalBytes += record.Bytes
		ta.stats.TotalRecords++

		// Track source IP
		if _, ok := ta.stats.SourceIPs[record.SrcAddr]; !ok {
			ta.stats.SourceIPs[record.SrcAddr] = &SourceIPStats{}
		}
		ta.stats.SourceIPs[record.SrcAddr].Bytes += record.Bytes
		ta.stats.SourceIPs[record.SrcAddr].Records++

		switch service {
		case "s3":
			ta.stats.S3Bytes += record.Bytes
			ta.stats.S3Records++
			ta.stats.SourceIPs[record.SrcAddr].S3 += record.Bytes
		case "dynamodb":
			ta.stats.DynamoBytes += record.Bytes
			ta.stats.DynamoRecords++
			ta.stats.SourceIPs[record.SrcAddr].Dynamo += record.Bytes
		case "ecr":
			ta.stats.ECRBytes += record.Bytes
			ta.stats.ECRRecords++
			ta.stats.SourceIPs[record.SrcAddr].ECR += record.Bytes
		default:
			ta.stats.OtherBytes += record.Bytes
			ta.stats.OtherRecords++
			ta.stats.SourceIPs[record.SrcAddr].Other += record.Bytes
		}
	}

	return &ta.stats, nil
}

func (ts *TrafficStats) String() string {
	return fmt.Sprintf(
		"Total: %d records, %.2f MB\n"+
			"  S3: %d records, %.2f MB (%.1f%%)\n"+
			"  DynamoDB: %d records, %.2f MB (%.1f%%)\n"+
			"  Other: %d records, %.2f MB (%.1f%%)",
		ts.TotalRecords, float64(ts.TotalBytes)/(1024*1024),
		ts.S3Records, float64(ts.S3Bytes)/(1024*1024), ts.S3Percentage(),
		ts.DynamoRecords, float64(ts.DynamoBytes)/(1024*1024), ts.DynamoPercentage(),
		ts.OtherRecords, float64(ts.OtherBytes)/(1024*1024), ts.OtherPercentage(),
	)
}

func (ts *TrafficStats) S3Percentage() float64 {
	if ts.TotalBytes == 0 {
		return 0
	}
	return float64(ts.S3Bytes) / float64(ts.TotalBytes) * 100
}

func (ts *TrafficStats) DynamoPercentage() float64 {
	if ts.TotalBytes == 0 {
		return 0
	}
	return float64(ts.DynamoBytes) / float64(ts.TotalBytes) * 100
}

func (ts *TrafficStats) ECRPercentage() float64 {
	if ts.TotalBytes == 0 {
		return 0
	}
	return float64(ts.ECRBytes) / float64(ts.TotalBytes) * 100
}

func (ts *TrafficStats) OtherPercentage() float64 {
	if ts.TotalBytes == 0 {
		return 0
	}
	return float64(ts.OtherBytes) / float64(ts.TotalBytes) * 100
}

// TopSourceIPs returns source IPs sorted by bytes descending
func (ts *TrafficStats) TopSourceIPs(limit int) []struct {
	IP    string
	Stats *SourceIPStats
} {
	type ipEntry struct {
		IP    string
		Stats *SourceIPStats
	}
	entries := make([]ipEntry, 0, len(ts.SourceIPs))
	for ip, stats := range ts.SourceIPs {
		entries = append(entries, ipEntry{IP: ip, Stats: stats})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Stats.Bytes > entries[j].Stats.Bytes
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	result := make([]struct {
		IP    string
		Stats *SourceIPStats
	}, len(entries))
	for i, e := range entries {
		result[i] = struct {
			IP    string
			Stats *SourceIPStats
		}{IP: e.IP, Stats: e.Stats}
	}
	return result
}
